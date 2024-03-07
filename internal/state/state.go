package state

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"

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

type contextMessage struct {
	payload proto.Message
	flags   wire.Flag
}

type Context struct {
	ctx context.Context

	messages chan *contextMessage
}

func (c *Context) Ctx() context.Context {
	return c.ctx
}

func (c *Context) Send(msg *contextMessage) error {
	select {
	case c.messages <- msg:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}
func newContext(inner context.Context) *Context {
	return &Context{
		ctx:      inner,
		messages: make(chan *contextMessage),
	}
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
	defer ctx.Send(&contextMessage{
		payload: &protocol.EndMessage{},
	})

	defer func() {
		if err := recover(); err != nil {
			// handle service panic
			// safely
			message := contextMessage{
				payload: &protocol.OutputStreamEntryMessage{
					Result: &protocol.OutputStreamEntryMessage_Failure{
						Failure: &protocol.Failure{
							Code:    2,
							Message: fmt.Sprint(err),
						},
					},
				},
			}

			ctx.Send(&message)

		}
	}()

	output := m.output(m.handler.Call(ctx, input))

	message := contextMessage{
		payload: output,
		flags:   wire.FlagCompleted,
	}

	ctx.Send(&message)
}

func (m *Machine) processMessage(ctx *Context, msg wire.Message, _ *wire.Reader) error {
	log.Debug().Bool("ack", msg.Flags().Ack()).Bool("completed", msg.Flags().Completed()).Uint16("type", uint16(msg.Type())).Msg("received message")

	switch msg := msg.(type) {
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
		return ErrUnexpectedMessage
	}

	return nil
}

func (m *Machine) process(inner context.Context, reader *wire.Reader) error {
	ctx := newContext(inner)

	for {
		select {
		case <-inner.Done():
			log.Debug().Msg("machine context is cancelled")
			return inner.Err()
		case read := <-reader.Next():
			if read.Err != nil {
				return read.Err
			}

			if err := m.processMessage(ctx, read.Message, reader); err != nil {
				return err
			}
		case read := <-ctx.messages:
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

func (m *Machine) Start(ctx context.Context) error {
	// reader starts a reader on protocol
	reader := m.protocol.Reader(ctx)

	msg, err := reader.Read(ctx)
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

	log.Debug().Str("id", base64.URLEncoding.EncodeToString(m.id)).Msg("start invocation")
	return m.process(ctx, reader)
}
