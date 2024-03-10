package state

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"sync"

	"github.com/muhamadazmy/restate-sdk-go/generated/proto/dynrpc"
	"github.com/muhamadazmy/restate-sdk-go/generated/proto/protocol"
	"github.com/muhamadazmy/restate-sdk-go/internal/wire"
	"github.com/muhamadazmy/restate-sdk-go/router"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

const (
	Version = 1
)

var (
	ErrUnexpectedMessage = fmt.Errorf("unexpected message")
	ErrInvalidVersion    = fmt.Errorf("invalid version number")
)

var (
	_ router.Context = (*Context)(nil)
)

type contextRequest struct {
	payload proto.Message
	flags   wire.Flag

	// wait for response
}

type Context struct {
	ctx context.Context

	partial bool
	current map[string][]byte

	messages chan *contextRequest
	cond     *sync.Cond
}

func (c *Context) Ctx() context.Context {
	return c.ctx
}

func (c *Context) send(msg *contextRequest) error {
	select {
	case c.messages <- msg:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

func (c *Context) recv(msg wire.Message) error {
	// received a response for
	log.Debug().Uint16("type", uint16(msg.Type())).Msg("context received a response from runtime")
	if msg.Type() == wire.GetStateEntryMessageType {
		// a get completion message
		c.cond.L.Lock()
		defer c.cond.L.Unlock()
		msg := msg.(*wire.GetStateEntryMessage)
		value := msg.Payload.GetValue()
		key := string(msg.Payload.Key)
		log.Debug().Str("key", key).Msg("received value for key")

		c.current[key] = value

		c.cond.Broadcast()
	}

	return nil
}

func (c *Context) Set(key string, value []byte) error {
	c.current[key] = value

	return c.send(&contextRequest{
		payload: &protocol.SetStateEntryMessage{
			Key:   []byte(key),
			Value: value,
		},
	})
}

func (c *Context) Get(key string) ([]byte, error) {
	msg := &protocol.GetStateEntryMessage{
		Key: []byte(key),
	}

	c.cond.L.Lock()
	defer c.cond.L.Unlock()

	value, ok := c.current[key]

	// debug:
	ok = false
	if ok {
		// value in map, we still send the current
		// value to the runtime
		msg.Result = &protocol.GetStateEntryMessage_Value{
			Value: value,
		}

		if err := c.send(&contextRequest{
			payload: msg,
		}); err != nil {
			return nil, err
		}

		return value, nil
	}

	//debug
	c.partial = true
	// key is not in map! there are 2 cases.
	if !c.partial {
		// current is complete. we need to return nil to the user
		// but also send an empty get state entry message
		msg.Result = &protocol.GetStateEntryMessage_Empty{}

		if err := c.send(&contextRequest{
			payload: msg,
		}); err != nil {
			return nil, err
		}

		return nil, nil
	}

	// so partial is true. so we need to send a empty result to runtime and
	// wait for response

	msg.Result = nil
	if err := c.send(&contextRequest{
		payload: msg,
	}); err != nil {
		return nil, err
	}

	for {
		c.cond.Wait()
		value, ok := c.current[key]
		if ok {
			return value, nil
		}
	}
}

func newContext(inner context.Context, start *wire.StartMessage) *Context {
	log.Debug().
		Bool("partial-state", start.Payload.PartialState).
		Int("state-len", len(start.Payload.StateMap)).
		Msg("start message")

	state := make(map[string][]byte)
	for _, entry := range start.Payload.StateMap {
		state[string(entry.Key)] = entry.Value
	}

	ctx := &Context{
		ctx:      inner,
		partial:  start.Payload.PartialState,
		messages: make(chan *contextRequest),
		current:  state,
		cond:     sync.NewCond(&sync.Mutex{}),
	}

	return ctx
}

type Machine struct {
	handler  router.Handler
	protocol *wire.Protocol

	// state
	id []byte
}

func NewMachine(handler router.Handler, conn io.ReadWriter) *Machine {
	return &Machine{
		handler:  handler,
		protocol: wire.NewProtocol(conn),
	}
}

func (m *Machine) output(r *dynrpc.RpcResponse, err error) *protocol.OutputStreamEntryMessage {
	var output protocol.OutputStreamEntryMessage
	if err != nil {
		output.Result = &protocol.OutputStreamEntryMessage_Failure{
			Failure: &protocol.Failure{
				Code:    2,
				Message: err.Error(),
			},
		}

		return &output
	}

	bytes, err := proto.Marshal(r)
	if err != nil {
		// this shouldn't happen but in case
		output.Result = &protocol.OutputStreamEntryMessage_Failure{
			Failure: &protocol.Failure{
				Code:    13, // internal error
				Message: err.Error(),
			},
		}

		return &output
	}

	output.Result = &protocol.OutputStreamEntryMessage_Value{
		Value: bytes,
	}

	return &output
}

func (m *Machine) invoke(ctx *Context, input *dynrpc.RpcRequest) {
	// always terminate the invocation with
	// an end message.
	// this will always terminate the connection
	defer ctx.send(&contextRequest{
		payload: &protocol.EndMessage{},
	})

	defer func() {
		if err := recover(); err != nil {
			// handle service panic
			// safely
			message := contextRequest{
				payload: &protocol.OutputStreamEntryMessage{
					Result: &protocol.OutputStreamEntryMessage_Failure{
						Failure: &protocol.Failure{
							Code:    2,
							Message: fmt.Sprint(err),
						},
					},
				},
			}

			ctx.send(&message)

		}
	}()

	output := m.output(m.handler.Call(ctx, input))

	message := contextRequest{
		payload: output,
		flags:   wire.FlagCompleted,
	}

	ctx.send(&message)
}

func (m *Machine) incoming(ctx *Context, msg wire.Message, _ *wire.Reader) error {
	log.Debug().Bool("ack", msg.Flags().Ack()).Bool("completed", msg.Flags().Completed()).Uint16("type", uint16(msg.Type())).Msg("received message")

	switch msg := msg.(type) {
	// the PollInputEntry is the message that will
	// trigger the call since we will have the input
	// ready.
	// all other messages are processed by the context.
	case *wire.PollInputEntry:
		value := msg.Payload.GetValue()
		var input dynrpc.RpcRequest
		if err := proto.Unmarshal(value, &input); err != nil {
			return fmt.Errorf("invalid invocation input: %w", err)
		}

		// we run the invocation in the background. Any messages
		// created by the invocation is forwarded to the runtime
		go m.invoke(ctx, &input)
	default:
		return ctx.recv(msg)
	}

	return nil
}

func (m *Machine) process(ctx *Context, reader *wire.Reader) error {

	for {
		select {
		case <-ctx.Ctx().Done():
			log.Debug().Msg("machine context is cancelled")
			return ctx.Ctx().Err()
		case read := <-reader.Next():
			if read.Err != nil {
				return read.Err
			}

			if read.Message == nil {
				// connection interrupted
				return nil
			}

			if err := m.incoming(ctx, read.Message, reader); err != nil {
				return err
			}
		case read := <-ctx.messages:
			// the context messages are sent over a channel
			// only to make sure we can control when to terminate
			// the stream if we need to.
			// returning the channel will cause the context to cancel
			// as well witch in return cause the invoked process to not
			// progress anymore (when the ctx is used)
			if err := m.protocol.Write(read.payload, read.flags); err != nil {
				return err
			}

			// TODO: better approach?
			switch read.payload.(type) {
			case *protocol.EndMessage:
				return nil
			case *protocol.ErrorMessage:
				return nil
			}
		}
	}
}

func (m *Machine) Start(inner context.Context) error {
	// reader starts a reader on protocol
	reader := m.protocol.Reader(inner)

	msg, err := reader.Read(inner)
	if err != nil {
		return err
	}

	if msg.Type() != wire.StartMessageType {
		// invalid negotiation
		return ErrUnexpectedMessage
	}

	start := msg.(*wire.StartMessage)

	fmt.Println("version: ", start.Version)
	if start.Version != Version {
		return ErrInvalidVersion
	}

	m.id = start.Payload.Id

	ctx := newContext(inner, start)
	log.Debug().Str("id", base64.URLEncoding.EncodeToString(m.id)).Msg("start invocation")
	return m.process(ctx, reader)
}
