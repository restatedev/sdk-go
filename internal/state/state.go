package state

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"runtime/debug"
	"sync"
	"time"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/generated/proto/protocol"
	"github.com/restatedev/sdk-go/internal/errors"
	"github.com/restatedev/sdk-go/internal/wire"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	Version = 1
)

var (
	ErrInvalidVersion = fmt.Errorf("invalid version number")
)

var (
	_ restate.Context = (*Context)(nil)
)

type Context struct {
	ctx     context.Context
	machine *Machine
}

var _ restate.ObjectContext = &Context{}
var _ restate.Context = &Context{}

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

func (c *Context) Keys() ([]string, error) {
	return c.machine.keys()
}

func (c *Context) Sleep(d time.Duration) error {
	after, err := c.machine.after(d)
	if err != nil {
		return err
	}
	return after.Done()
}

func (c *Context) After(d time.Duration) (restate.After, error) {
	return c.machine.after(d)
}

func (c *Context) Service(service string) restate.ServiceClient {
	return &serviceProxy{
		Context: c,
		service: service,
	}
}

func (c *Context) ServiceSend(service string, delay time.Duration) restate.ServiceSendClient {
	return &serviceSendProxy{
		Context: c,
		service: service,
		delay:   delay,
	}
}

func (c *Context) Object(service, key string) restate.ServiceClient {
	return &serviceProxy{
		Context: c,
		service: service,
		key:     key,
	}
}

func (c *Context) ObjectSend(service, key string, delay time.Duration) restate.ServiceSendClient {
	return &serviceSendProxy{
		Context: c,
		service: service,
		key:     key,
		delay:   delay,
	}
}

func (c *Context) SideEffect(fn func() ([]byte, error)) ([]byte, error) {
	return c.machine.sideEffect(fn)
}

func (c *Context) Awakeable() (restate.Awakeable[[]byte], error) {
	return c.machine.awakeable()
}

func (c *Context) ResolveAwakeable(id string, value []byte) error {
	return c.machine.resolveAwakeable(id, value)
}

func (c *Context) RejectAwakeable(id string, reason error) error {
	return c.machine.rejectAwakeable(id, reason)
}

func (c *Context) Key() string {
	return c.machine.key
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
	ctx context.Context

	handler  restate.Handler
	protocol *wire.Protocol

	// state
	id  []byte
	key string

	partial bool
	current map[string][]byte

	entries    []wire.Message
	entryIndex uint32
	entryMutex sync.Mutex

	log zerolog.Logger

	pendingCompletions map[uint32]wire.CompleteableMessage
	pendingAcks        map[uint32]wire.AckableMessage
	pendingMutex       sync.RWMutex

	failure any
}

func NewMachine(handler restate.Handler, conn io.ReadWriter) *Machine {
	m := &Machine{
		handler:            handler,
		current:            make(map[string][]byte),
		log:                log.Logger,
		pendingAcks:        map[uint32]wire.AckableMessage{},
		pendingCompletions: map[uint32]wire.CompleteableMessage{},
	}
	m.protocol = wire.NewProtocol(&m.log, conn)
	return m
}

// Start starts the state machine
func (m *Machine) Start(inner context.Context, trace string) error {
	// reader starts a rea
	msg, err := m.protocol.Read()
	if err != nil {
		return err
	}

	start, ok := msg.(*wire.StartMessage)
	if !ok {
		// invalid negotiation
		return wire.ErrUnexpectedMessage
	}

	m.ctx = inner
	m.id = start.Id
	m.key = start.Key

	m.log = m.log.With().Str("id", start.DebugId).Str("method", trace).Logger()

	ctx := newContext(inner, m)

	m.log.Debug().Msg("start invocation")
	defer m.log.Debug().Msg("invocation ended")

	return m.process(ctx, start)
}

func (m *Machine) invoke(ctx *Context, input []byte, outputSeen bool) error {
	// always terminate the invocation with
	// an end message.
	// this will always terminate the connection

	defer func() {
		// recover will return a non-nil object
		// if there was a panic
		//
		recovered := recover()

		switch typ := recovered.(type) {
		case nil:
			// nothing to do, just exit
			return
		case *entryMismatch:
			expected, _ := json.Marshal(typ.expectedEntry)
			actual, _ := json.Marshal(typ.actualEntry)
			msg := fmt.Sprintf(`Journal mismatch: Replayed journal entries did not correspond to the user code. The user code has to be deterministic!
The journal entry at position %d was:
- In the user code: type: %T, message: %s
- In the replayed messages: type: %T, message %s`,
				typ.entryIndex, typ.expectedEntry, string(expected), typ.actualEntry, string(actual))

			m.log.Error().Msg(msg)

			// journal entry mismatch
			if err := m.protocol.Write(&wire.ErrorMessage{
				ErrorMessage: protocol.ErrorMessage{
					Code:        uint32(errors.ErrJournalMismatch),
					Message:     msg,
					Description: string(debug.Stack()),
				},
			}); err != nil {
				m.log.Error().Err(err).Msg("error sending failure message")
			}
		default:
			// unknown panic!
			// send an error message (retryable)
			if err := m.protocol.Write(&wire.ErrorMessage{
				ErrorMessage: protocol.ErrorMessage{
					Code:        500,
					Message:     fmt.Sprint(typ),
					Description: string(debug.Stack()),
				},
			}); err != nil {
				m.log.Error().Err(err).Msg("error sending failure message")
			}

			return
		}
	}()

	if outputSeen {
		return m.protocol.Write(&wire.EndMessage{})
	}

	bytes, err := m.handler.Call(ctx, input)
	if err != nil {
		m.log.Error().Err(err).Msg("failure")
	}

	if err != nil && restate.IsTerminalError(err) {
		// terminal errors.
		if err := m.protocol.Write(&wire.OutputEntryMessage{
			OutputEntryMessage: protocol.OutputEntryMessage{
				Result: &protocol.OutputEntryMessage_Failure{
					Failure: &protocol.Failure{
						Code:    uint32(restate.ErrorCode(err)),
						Message: err.Error(),
					},
				},
			},
		}); err != nil {
			return err
		}
		return m.protocol.Write(&wire.EndMessage{})
	} else if err != nil {
		// non terminal error - no end message
		return m.protocol.Write(&wire.ErrorMessage{
			ErrorMessage: protocol.ErrorMessage{
				Code:    uint32(restate.ErrorCode(err)),
				Message: err.Error(),
			},
		})
	} else {
		if err := m.protocol.Write(&wire.OutputEntryMessage{
			OutputEntryMessage: protocol.OutputEntryMessage{
				Result: &protocol.OutputEntryMessage_Value{
					Value: bytes,
				},
			},
		}); err != nil {
			return err
		}

		return m.protocol.Write(&wire.EndMessage{})
	}
}

func (m *Machine) process(ctx *Context, start *wire.StartMessage) error {
	for _, entry := range start.StateMap {
		m.current[string(entry.Key)] = entry.Value
	}

	// expect input message
	msg, err := m.protocol.Read()
	if err != nil {
		return err
	}

	if _, ok := msg.(*wire.InputEntryMessage); !ok {
		return wire.ErrUnexpectedMessage
	}

	m.log.Trace().Uint32("known entries", start.KnownEntries).Msg("known entires")
	m.entries = make([]wire.Message, 0, start.KnownEntries-1)

	outputSeen := false

	// we don't track the poll input entry
	for i := uint32(1); i < start.KnownEntries; i++ {
		msg, err := m.protocol.Read()
		if err != nil {
			return fmt.Errorf("failed to read entry: %w", err)
		}

		m.log.Trace().Type("type", msg).Msg("replay log entry")
		m.entries = append(m.entries, msg)

		if _, ok := msg.(*wire.OutputEntryMessage); ok {
			outputSeen = true
		}
	}

	go m.handleCompletionsAcks()

	inputMsg := msg.(*wire.InputEntryMessage)
	value := inputMsg.GetValue()
	return m.invoke(ctx, value, outputSeen)

}

func (c *Machine) currentEntry() (wire.Message, bool) {
	if c.entryIndex <= uint32(len(c.entries)) {
		return c.entries[c.entryIndex-1], true
	}

	return nil, false
}

// replayOrNew is a utility function to easily either
// replay a log entry, or create a new one if one
// does not exist
//
// this should be an instance method on Machine but unfortunately
// go does not support generics on instance methods
//
// the idea is when called, it will check if there is a log
// entry at current index, then compare the entry message type
// if not matching, that's obviously an error with the code version
// (code has changed and now doesn't match the play log)
//
// if type is okay, the function will then call a `replay“ callback.
// the replay callback just need to extract the result from the entry
//
// otherwise this function will call a `new` callback to create a new entry in the log
// by sending the proper runtime messages
func replayOrNew[M wire.Message, O any](
	m *Machine,
	replay func(msg M) O,
	new func() (O, error),
) (output O, err error) {
	// lock around preparing the entry, but we would never await an ack or completion with this held.
	m.entryMutex.Lock()
	defer m.entryMutex.Unlock()

	if m.failure != nil {
		// maybe the user will try to catch our panics, but we will just keep producing them
		panic(m.failure)
	}

	m.entryIndex += 1

	// check if there is an entry as this index
	entry, ok := m.currentEntry()

	// if entry exists, we need to replay it
	// by calling the replay function
	if ok {
		if entry, ok := entry.(M); !ok {
			// will be eg *wire.CallEntryMessage(nil)
			var expectedEntry M
			panic(m.newEntryMismatch(expectedEntry, entry))
		} else {
			return replay(entry), nil
		}
	}

	// other wise call the new function
	return new()
}
