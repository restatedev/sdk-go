package state

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"sync"
	"time"

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

type Context struct {
	ctx context.Context

	partial bool
	current map[string][]byte

	protocol *wire.Protocol
	m        sync.Mutex
}

func (c *Context) Ctx() context.Context {
	return c.ctx
}

func (c *Context) Set(key string, value []byte) error {
	c.m.Lock()
	defer c.m.Unlock()

	c.current[key] = value

	return c.protocol.Write(
		&protocol.SetStateEntryMessage{
			Key:   []byte(key),
			Value: value,
		})
}

func (c *Context) Clear(key string) error {
	c.m.Lock()
	defer c.m.Unlock()

	return c.protocol.Write(
		&protocol.ClearStateEntryMessage{
			Key: []byte(key),
		},
	)
}

// ClearAll drops all associated keys
func (c *Context) ClearAll() error {
	c.m.Lock()
	defer c.m.Unlock()

	return c.protocol.Write(
		&protocol.ClearAllStateEntryMessage{},
	)
}

func (c *Context) Get(key string) ([]byte, error) {
	msg := &protocol.GetStateEntryMessage{
		Key: []byte(key),
	}

	c.m.Lock()
	defer c.m.Unlock()

	value, ok := c.current[key]

	if ok {
		// value in map, we still send the current
		// value to the runtime
		msg.Result = &protocol.GetStateEntryMessage_Value{
			Value: value,
		}

		if err := c.protocol.Write(msg); err != nil {
			return nil, err
		}

		return value, nil
	}

	// key is not in map! there are 2 cases.
	if !c.partial {
		// current is complete. we need to return nil to the user
		// but also send an empty get state entry message
		msg.Result = &protocol.GetStateEntryMessage_Empty{}

		if err := c.protocol.Write(msg); err != nil {
			return nil, err
		}

		return nil, nil
	}

	if err := c.protocol.Write(msg); err != nil {
		return nil, err
	}

	// wait for completion
	response, err := c.protocol.Read()
	if err != nil {
		return nil, err
	}

	if response.Type() != wire.CompletionMessageType {
		return nil, ErrUnexpectedMessage
	}

	completion := response.(*wire.CompletionMessage)

	switch value := completion.Payload.Result.(type) {
	case *protocol.CompletionMessage_Empty:
		return nil, nil
	case *protocol.CompletionMessage_Failure:
		// the get state entry message is not failable so this should
		// never happen
		return nil, fmt.Errorf("[%d] %s", value.Failure.Code, value.Failure.Message)
	case *protocol.CompletionMessage_Value:
		c.current[key] = value.Value
		return value.Value, nil
	}

	return nil, fmt.Errorf("unreachable")
}

func (c *Context) Sleep(deadline time.Time) error {
	if err := c.protocol.Write(&protocol.SleepEntryMessage{
		WakeUpTime: uint64(deadline.UnixMilli()),
	}); err != nil {
		return err
	}

	response, err := c.protocol.Read()
	if err != nil {
		return err
	}

	if response.Type() != wire.CompletionMessageType {
		return ErrUnexpectedMessage
	}

	return nil
}

func newContext(inner context.Context, protocol *wire.Protocol, start *wire.StartMessage) *Context {
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
		current:  state,
		protocol: protocol,
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

func (m *Machine) invoke(ctx *Context, input *dynrpc.RpcRequest) error {
	// always terminate the invocation with
	// an end message.
	// this will always terminate the connection
	defer m.protocol.Write(&protocol.EndMessage{})

	defer func() {
		if err := recover(); err != nil {
			// handle service panic
			// safely

			wErr := m.protocol.Write(&protocol.OutputStreamEntryMessage{
				Result: &protocol.OutputStreamEntryMessage_Failure{
					Failure: &protocol.Failure{
						Code:    2,
						Message: fmt.Sprint(err),
					},
				},
			})

			log.Error().Err(wErr).Msg("error sending failure message")
		}
	}()

	output := m.output(m.handler.Call(ctx, input))

	return m.protocol.Write(output)
}

func (m *Machine) process(ctx *Context) error {

	// expect input message
	msg, err := m.protocol.Read()
	if err != nil {
		return err
	}

	if msg.Type() != wire.PollInputEntryMessageType {
		return ErrUnexpectedMessage
	}

	inputMsg := msg.(*wire.PollInputEntry)
	value := inputMsg.Payload.GetValue()
	var input dynrpc.RpcRequest
	if err := proto.Unmarshal(value, &input); err != nil {
		return fmt.Errorf("invalid invocation input: %w", err)
	}

	return m.invoke(ctx, &input)

}

func (m *Machine) Start(inner context.Context) error {
	// reader starts a rea
	msg, err := m.protocol.Read()
	if err != nil {
		return err
	}

	if msg.Type() != wire.StartMessageType {
		// invalid negotiation
		return ErrUnexpectedMessage
	}

	start := msg.(*wire.StartMessage)

	if start.Version != Version {
		return ErrInvalidVersion
	}

	m.id = start.Payload.Id

	ctx := newContext(inner, m.protocol, start)
	log.Debug().Str("id", base64.URLEncoding.EncodeToString(m.id)).Msg("start invocation")
	return m.process(ctx)
}
