package state

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"runtime/debug"
	"sync"
	"time"

	"github.com/muhamadazmy/restate-sdk-go"
	"github.com/muhamadazmy/restate-sdk-go/generated/proto/dynrpc"
	"github.com/muhamadazmy/restate-sdk-go/generated/proto/protocol"
	"github.com/muhamadazmy/restate-sdk-go/internal/wire"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

const (
	Version = 1
)

var (
	ErrUnexpectedMessage = fmt.Errorf("unexpected message")
	ErrInvalidVersion    = fmt.Errorf("invalid version number")

	errUnreachable = fmt.Errorf("unreachable")
)

var (
	_ restate.Context = (*Context)(nil)
)

type Context struct {
	ctx     context.Context
	machine *Machine
}

func (c *Context) Ctx() context.Context {
	return c.ctx
}

func (c *Context) Set(key string, value []byte) error {
	return c.machine.set(key, value)
}

func (c *Context) Clear(key string) error {
	return c.machine.clear(key)

}

// ClearAll drops all associated keys
func (c *Context) ClearAll() error {
	return c.machine.clearAll()

}

func (c *Context) Get(key string) ([]byte, error) {
	return c.machine.get(key)
}

func (c *Context) Sleep(until time.Time) error {
	return c.machine.sleep(until)
}

func (c *Context) Service(service string) restate.Service {
	return &serviceProxy{
		Context: c,
		service: service,
	}
}

func newContext(inner context.Context, machine *Machine) *Context {

	// state := make(map[string][]byte)
	// for _, entry := range start.Payload.StateMap {
	// 	state[string(entry.Key)] = entry.Value
	// }

	ctx := &Context{
		ctx:     inner,
		machine: machine,
	}

	return ctx
}

type Machine struct {
	handler  restate.Handler
	protocol *wire.Protocol
	mutex    sync.Mutex

	// state
	id []byte

	partial bool
	current map[string][]byte

	entries []wire.Message
}

func NewMachine(handler restate.Handler, conn io.ReadWriter) *Machine {
	return &Machine{
		handler:  handler,
		protocol: wire.NewProtocol(conn),
		current:  make(map[string][]byte),
	}
}

func (m *Machine) output(r *dynrpc.RpcResponse, err error) proto.Message {
	// TODO: if err is terminal return outputStreamEntryMessage but if error is
	// not terminal, return ErrorMessage instead.
	//var output protocol.OutputStreamEntryMessage

	if err != nil && restate.IsTerminalError(err) {
		// terminal errors.
		return &protocol.OutputStreamEntryMessage{
			Result: &protocol.OutputStreamEntryMessage_Failure{
				Failure: &protocol.Failure{
					Code:    uint32(restate.ErrorCode(err)),
					Message: err.Error(),
				},
			},
		}
	} else if err != nil {
		// non terminal error!
		return &protocol.ErrorMessage{
			Code:    uint32(restate.ErrorCode(err)),
			Message: err.Error(),
		}
	}

	bytes, err := proto.Marshal(r)
	if err != nil {
		// this shouldn't happen but in case we return a retry error
		return &protocol.ErrorMessage{
			Code:        uint32(restate.INTERNAL),
			Message:     err.Error(),
			Description: "failed to serialize call output",
		}
	}

	return &protocol.OutputStreamEntryMessage{
		Result: &protocol.OutputStreamEntryMessage_Value{
			Value: bytes,
		},
	}
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

			// this should become a retry error ErrorMessage
			wErr := m.protocol.Write(&protocol.ErrorMessage{
				Code:        uint32(restate.INTERNAL),
				Message:     fmt.Sprint(err),
				Description: string(debug.Stack()),
			})

			if wErr != nil {
				log.Error().Err(wErr).Msg("error sending failure message")
			}
		}
	}()

	output := m.output(m.handler.Call(ctx, input))

	return m.protocol.Write(output)
}

func (m *Machine) process(ctx *Context, start *wire.StartMessage) error {
	m.id = start.Payload.Id

	for _, entry := range start.Payload.StateMap {
		m.current[string(entry.Key)] = entry.Value
	}

	// expect input message
	msg, err := m.protocol.Read()
	if err != nil {
		return err
	}

	if msg.Type() != wire.PollInputEntryMessageType {
		return ErrUnexpectedMessage
	}

	log.Debug().Uint32("known entries", start.Payload.KnownEntries).Msg("known entires")
	for i := uint32(1); i < start.Payload.KnownEntries; i++ {
		msg, err := m.protocol.Read()
		if err != nil {
			return fmt.Errorf("failed to read entry: %w", err)
		}

		log.Debug().Uint16("type", uint16(msg.Type())).Msg("received entry")
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

	ctx := newContext(inner, m)

	log.Trace().Str("id", base64.URLEncoding.EncodeToString(m.id)).Msg("start invocation")
	return m.process(ctx, start)
}
