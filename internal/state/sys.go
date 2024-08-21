package state

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/restatedev/sdk-go/encoding"
	protocol "github.com/restatedev/sdk-go/generated/dev/restate/service"
	"github.com/restatedev/sdk-go/internal/errors"
	"github.com/restatedev/sdk-go/internal/futures"
	"github.com/restatedev/sdk-go/internal/wire"
	"google.golang.org/protobuf/proto"
)

type entryMismatch struct {
	entryIndex uint32
	// this can be satisfied by a nil pointer in the case that there is an entry type mismatch
	expectedEntry wire.Message
	actualEntry   wire.Message
}

func (m *Machine) newEntryMismatch(expectedEntry wire.Message, actualEntry wire.Message) *entryMismatch {
	e := &entryMismatch{m.entryIndex, expectedEntry, actualEntry}
	m.failure = e
	return e
}

type protocolViolation struct {
	entryIndex uint32
	entry      wire.Message
	err        error
}

func (m *Machine) newProtocolViolation(entry wire.Message, err error) *protocolViolation {
	e := &protocolViolation{m.entryIndex, entry, err}
	m.failure = e
	return e
}

func (m *Machine) set(key string, value []byte) {
	_, _ = replayOrNew(
		m,
		func(entry *wire.SetStateEntryMessage) (void encoding.Void) {
			if string(entry.Key) != key || !bytes.Equal(entry.Value, value) {
				panic(m.newEntryMismatch(&wire.SetStateEntryMessage{
					SetStateEntryMessage: protocol.SetStateEntryMessage{
						Key:   []byte(key),
						Value: value,
					},
				}, entry))
			}
			return
		}, func() (void encoding.Void) {
			m._set(key, value)
			return void
		})

	m.current[key] = value
}

func (m *Machine) _set(key string, value []byte) {
	m.Write(
		&wire.SetStateEntryMessage{
			SetStateEntryMessage: protocol.SetStateEntryMessage{
				Key:   []byte(key),
				Value: value,
			},
		})
}

func (m *Machine) clear(key string) {
	_, _ = replayOrNew(
		m,
		func(entry *wire.ClearStateEntryMessage) (void encoding.Void) {
			if string(entry.Key) != key {
				panic(m.newEntryMismatch(&wire.ClearStateEntryMessage{
					ClearStateEntryMessage: protocol.ClearStateEntryMessage{
						Key: []byte(key),
					},
				}, entry))
			}

			return
		}, func() encoding.Void {
			m._clear(key)
			return encoding.Void{}
		},
	)

	delete(m.current, key)
}

func (m *Machine) _clear(key string) {
	m.Write(
		&wire.ClearStateEntryMessage{
			ClearStateEntryMessage: protocol.ClearStateEntryMessage{
				Key: []byte(key),
			},
		},
	)
}

func (m *Machine) clearAll() {
	_, _ = replayOrNew(
		m,
		func(entry *wire.ClearAllStateEntryMessage) (void encoding.Void) {
			return
		}, func() encoding.Void {
			m._clearAll()
			return encoding.Void{}
		},
	)
	m.current = map[string][]byte{}
	m.partial = false
}

// clearAll drops all associated keys
func (m *Machine) _clearAll() {
	m.Write(
		&wire.ClearAllStateEntryMessage{},
	)
}

func (m *Machine) get(key string) ([]byte, bool, error) {
	entry, entryIndex := replayOrNew(
		m,
		func(entry *wire.GetStateEntryMessage) *wire.GetStateEntryMessage {
			if string(entry.Key) != key {
				panic(m.newEntryMismatch(&wire.GetStateEntryMessage{
					GetStateEntryMessage: protocol.GetStateEntryMessage{
						Key: []byte(key),
					},
				}, entry))
			}
			return entry
		}, func() *wire.GetStateEntryMessage {
			return m._get(key)
		})

	entry.Await(m.suspensionCtx, entryIndex)

	switch value := entry.Result.(type) {
	case *protocol.GetStateEntryMessage_Empty:
		return nil, false, nil
	case *protocol.GetStateEntryMessage_Value:
		m.current[key] = value.Value
		return value.Value, true, nil
	case *protocol.GetStateEntryMessage_Failure:
		return nil, false, errors.ErrorFromFailure(value.Failure)
	default:
		panic(m.newProtocolViolation(entry, fmt.Errorf("get state entry had invalid result: %v", entry.Result)))
	}
}

func (m *Machine) _get(key string) *wire.GetStateEntryMessage {
	msg := &wire.GetStateEntryMessage{
		GetStateEntryMessage: protocol.GetStateEntryMessage{
			Key: []byte(key),
		},
	}

	value, ok := m.current[key]

	if ok {
		// value in map, we still send the current
		// value to the runtime
		msg.Complete(&protocol.CompletionMessage{Result: &protocol.CompletionMessage_Value{Value: value}})

		m.Write(msg)

		return msg
	}

	// key is not in map! there are 2 cases.
	if !m.partial {
		// current is complete. we need to return nil to the user
		// but also send an empty get state entry message
		msg.Complete(&protocol.CompletionMessage{Result: &protocol.CompletionMessage_Empty{Empty: &protocol.Empty{}}})

		m.Write(msg)

		return msg
	}

	// we didn't see the value and we don't know for sure there isn't one; ask the runtime for it

	m.Write(msg)

	return msg
}

func (m *Machine) keys() ([]string, error) {
	entry, entryIndex := replayOrNew(
		m,
		func(entry *wire.GetStateKeysEntryMessage) *wire.GetStateKeysEntryMessage {
			return entry
		},
		m._keys,
	)

	entry.Await(m.suspensionCtx, entryIndex)

	switch value := entry.Result.(type) {
	case *protocol.GetStateKeysEntryMessage_Value:
		values := make([]string, 0, len(value.Value.Keys))
		for _, key := range value.Value.Keys {
			values = append(values, string(key))
		}

		return values, nil
	case *protocol.GetStateKeysEntryMessage_Failure:
		return nil, errors.ErrorFromFailure(value.Failure)
	default:
		panic(m.newProtocolViolation(entry, fmt.Errorf("get state keys entry had invalid result: %v", entry.Result)))
	}
}

func (m *Machine) _keys() *wire.GetStateKeysEntryMessage {
	msg := &wire.GetStateKeysEntryMessage{}
	if !m.partial {
		keys := make([]string, 0, len(m.current))
		for k := range m.current {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		byteKeys := make([][]byte, len(keys))
		for i := range keys {
			byteKeys[i] = []byte(keys[i])
		}

		stateKeys := &protocol.GetStateKeysEntryMessage_StateKeys{Keys: byteKeys}
		value, err := proto.Marshal(stateKeys)
		if err != nil {
			panic(err) // this is pretty much impossible
		}

		// we can return keys entirely from cache
		msg.Complete(&protocol.CompletionMessage{Result: &protocol.CompletionMessage_Value{
			Value: value,
		}})
	}

	m.Write(msg)

	return msg
}

func (m *Machine) after(d time.Duration) *futures.After {
	entry, entryIndex := replayOrNew(
		m,
		func(entry *wire.SleepEntryMessage) *wire.SleepEntryMessage {
			// we shouldn't verify the time because this would be different every time
			return entry
		}, func() *wire.SleepEntryMessage {
			return m._sleep(d)
		},
	)

	return futures.NewAfter(m.suspensionCtx, entry, entryIndex)
}

func (m *Machine) sleep(d time.Duration) error {
	return m.after(d).Done()
}

// _sleep creating a new sleep entry.
func (m *Machine) _sleep(d time.Duration) *wire.SleepEntryMessage {
	msg := &wire.SleepEntryMessage{
		SleepEntryMessage: protocol.SleepEntryMessage{
			WakeUpTime: uint64(time.Now().Add(d).UnixMilli()),
		},
	}
	m.Write(msg)

	return msg
}

func (m *Machine) run(fn func(RunContext) ([]byte, error)) ([]byte, error) {
	entry, entryIndex := replayOrNew(
		m,
		func(entry *wire.RunEntryMessage) *wire.RunEntryMessage {
			return entry
		},
		func() *wire.RunEntryMessage {
			return m._run(fn)
		},
	)

	// run entry must be acknowledged before proceeding
	entry.Await(m.suspensionCtx, entryIndex)

	switch result := entry.Result.(type) {
	case *protocol.RunEntryMessage_Failure:
		return nil, errors.ErrorFromFailure(result.Failure)
	case *protocol.RunEntryMessage_Value:
		return result.Value, nil
	case nil:
		// Empty result is valid
		return nil, nil
	default:
		panic(m.newProtocolViolation(entry, fmt.Errorf("run entry had invalid result: %v", entry.Result)))
	}
}

type RunContext struct {
	context.Context
	log     *slog.Logger
	request *Request
}

func (r RunContext) Log() *slog.Logger { return r.log }
func (r RunContext) Request() *Request { return r.request }

type Request struct {
	// The unique id that identifies the current function invocation. This id is guaranteed to be
	// unique across invocations, but constant across reties and suspensions.
	ID []byte
	// Request headers - the following headers capture the original invocation headers, as provided to
	// the ingress.
	Headers map[string]string
	// Attempt headers - the following headers are sent by the restate runtime.
	// These headers are attempt specific, generated by the restate runtime uniquely for each attempt.
	// These headers might contain information such as the W3C trace context, and attempt specific information.
	AttemptHeaders map[string][]string
	// Raw unparsed request body
	Body []byte
}

func (m *Machine) _run(fn func(RunContext) ([]byte, error)) *wire.RunEntryMessage {
	bytes, err := fn(RunContext{m.ctx, m.userLog, &m.request})

	if err != nil {
		if errors.IsTerminalError(err) {
			msg := &wire.RunEntryMessage{
				RunEntryMessage: protocol.RunEntryMessage{
					Result: &protocol.RunEntryMessage_Failure{
						Failure: &protocol.Failure{
							Code:    uint32(errors.ErrorCode(err)),
							Message: err.Error(),
						},
					},
				},
			}
			m.Write(msg)

			return msg
		} else {
			panic(m.newRunFailure(err))
		}
	} else {
		msg := &wire.RunEntryMessage{
			RunEntryMessage: protocol.RunEntryMessage{
				Result: &protocol.RunEntryMessage_Value{
					Value: bytes,
				},
			},
		}
		m.Write(msg)

		return msg
	}
}

type runFailure struct {
	entryIndex uint32
	err        error
}

func (m *Machine) newRunFailure(err error) *runFailure {
	s := &runFailure{m.entryIndex, err}
	m.failure = s
	return s
}

type codecFailure struct {
	entryIndex uint32
	err        error
}

func (m *Machine) newCodecFailure(err error) *codecFailure {
	c := &codecFailure{m.entryIndex, err}
	m.failure = c
	return c
}
