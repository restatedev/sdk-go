package state

import (
	"bytes"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/generated/proto/javascript"
	"github.com/restatedev/sdk-go/generated/proto/protocol"
	"github.com/restatedev/sdk-go/internal/wire"
	"google.golang.org/protobuf/proto"
)

var (
	errEntryMismatch = restate.WithErrorCode(fmt.Errorf("log entry mismatch"), 32)
	errUnreachable   = fmt.Errorf("unreachable")
)

func (m *Machine) set(key string, value []byte) error {
	_, err := replayOrNew(
		m,
		wire.SetStateEntryMessageType,
		func(entry *wire.SetStateEntryMessage) (void restate.Void, err error) {
			if string(entry.Payload.Key) != key || !bytes.Equal(entry.Payload.Value, value) {
				return void, errEntryMismatch
			}

			return
		}, func() (void restate.Void, err error) {
			return void, m._set(key, value)
		})

	return err
}

func (m *Machine) _set(key string, value []byte) error {
	m.current[key] = value

	return m.protocol.Write(
		&protocol.SetStateEntryMessage{
			Key:   []byte(key),
			Value: value,
		})
}

func (m *Machine) clear(key string) error {
	_, err := replayOrNew(
		m,
		wire.ClearStateEntryMessageType,
		func(entry *wire.ClearStateEntryMessage) (void restate.Void, err error) {
			if string(entry.Payload.Key) != key {
				return void, errEntryMismatch
			}

			return void, nil
		}, func() (restate.Void, error) {
			return restate.Void{}, m._clear(key)
		},
	)

	if err != nil {
		return err
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.current, key)

	return err
}

func (m *Machine) _clear(key string) error {
	return m.protocol.Write(
		&protocol.ClearStateEntryMessage{
			Key: []byte(key),
		},
	)
}

func (m *Machine) clearAll() error {
	_, err := replayOrNew(
		m,
		wire.ClearAllStateEntryMessageType,
		func(entry *wire.ClearAllStateEntryMessage) (void restate.Void, err error) {
			return
		}, func() (restate.Void, error) {
			return restate.Void{}, m._clearAll()
		},
	)
	if err != nil {
		return err
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.current = map[string][]byte{}
	m.partial = false

	return nil
}

// clearAll drops all associated keys
func (m *Machine) _clearAll() error {
	return m.protocol.Write(
		&protocol.ClearAllStateEntryMessage{},
	)
}

func (m *Machine) get(key string) ([]byte, error) {
	return replayOrNew(
		m,
		wire.GetStateEntryMessageType,
		func(entry *wire.GetStateEntryMessage) ([]byte, error) {
			if string(entry.Payload.Key) != key {
				return nil, errEntryMismatch
			}

			switch result := entry.Payload.Result.(type) {
			case *protocol.GetStateEntryMessage_Empty:
				return nil, nil
			case *protocol.GetStateEntryMessage_Failure:
				return nil, fmt.Errorf("[%d] %s", result.Failure.Code, result.Failure.Message)
			case *protocol.GetStateEntryMessage_Value:
				return result.Value, nil
			}

			return nil, fmt.Errorf("unreachable")
		}, func() ([]byte, error) {
			return m._get(key)
		})
}

func (m *Machine) _get(key string) ([]byte, error) {
	msg := &protocol.GetStateEntryMessage{
		Key: []byte(key),
	}

	value, ok := m.current[key]

	if ok {
		// value in map, we still send the current
		// value to the runtime
		msg.Result = &protocol.GetStateEntryMessage_Value{
			Value: value,
		}

		if err := m.protocol.Write(msg); err != nil {
			return nil, err
		}

		// read and discard response
		_, err := m.protocol.Read()
		if err != nil {
			return value, err
		}
		return value, nil
	}

	// key is not in map! there are 2 cases.
	if !m.partial {
		// current is complete. we need to return nil to the user
		// but also send an empty get state entry message
		msg.Result = &protocol.GetStateEntryMessage_Empty{}

		if err := m.protocol.Write(msg); err != nil {
			return nil, err
		}

		// read and discard response
		_, err := m.protocol.Read()
		if err != nil {
			return value, err
		}

		return nil, nil
	}

	if err := m.protocol.Write(msg); err != nil {
		return nil, err
	}

	// wait for completion
	response, err := m.protocol.Read()
	if err != nil {
		return nil, err
	}

	if response.Type() != wire.CompletionMessageType {
		return nil, wire.ErrUnexpectedMessage
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
		m.current[key] = value.Value
		return value.Value, nil
	}

	return nil, fmt.Errorf("unreachable")
}

func (m *Machine) keys() ([]string, error) {
	return replayOrNew(
		m,
		wire.GetStateKeysEntryMessageType,
		func(entry *wire.GetStateKeysEntryMessage) ([]string, error) {
			switch result := entry.Payload.Result.(type) {
			case *protocol.GetStateKeysEntryMessage_Failure:
				return nil, fmt.Errorf("[%d] %s", result.Failure.Code, result.Failure.Message)
			case *protocol.GetStateKeysEntryMessage_Value:
				keys := make([]string, 0, len(result.Value.Keys))
				for _, key := range result.Value.Keys {
					keys = append(keys, string(key))
				}
				return keys, nil
			}

			return nil, errUnreachable
		},
		m._keys,
	)
}

func (m *Machine) _keys() ([]string, error) {
	if err := m.protocol.Write(&protocol.GetStateKeysEntryMessage{}); err != nil {
		return nil, err
	}

	msg, err := m.protocol.Read()
	if err != nil {
		return nil, err
	}

	if msg.Type() != wire.CompletionMessageType {
		m.log.Error().Stringer("type", msg.Type()).Msg("receiving message of type")
		return nil, wire.ErrUnexpectedMessage
	}

	response := msg.(*wire.CompletionMessage)

	switch value := response.Payload.Result.(type) {
	case *protocol.CompletionMessage_Empty:
		return nil, nil
	case *protocol.CompletionMessage_Failure:
		// the get state entry message is not failable so this should
		// never happen
		return nil, fmt.Errorf("[%d] %s", value.Failure.Code, value.Failure.Message)
	case *protocol.CompletionMessage_Value:
		var keys protocol.GetStateKeysEntryMessage_StateKeys

		if err := proto.Unmarshal(value.Value, &keys); err != nil {
			return nil, err
		}

		values := make([]string, 0, len(keys.Keys))
		for _, key := range keys.Keys {
			values = append(values, string(key))
		}

		return values, nil
	}

	return nil, nil
}

func (m *Machine) sleep(until time.Time) error {
	_, err := replayOrNew(
		m,
		wire.SleepEntryMessageType,
		func(entry *wire.SleepEntryMessage) (void restate.Void, err error) {
			// we shouldn't verify the time because this would be different every time
			return
		}, func() (restate.Void, error) {
			return restate.Void{}, m._sleep(until)
		},
	)

	return err
}

// _sleep creating a new sleep entry. The implementation of this function
// will also suspend execution if sleep duration is greater than 1 second
// as a form of optimization
func (m *Machine) _sleep(until time.Time) error {
	if err := m.protocol.Write(&protocol.SleepEntryMessage{
		WakeUpTime: uint64(until.UnixMilli()),
	}, wire.FlagRequiresAck); err != nil {
		return err
	}

	entryIndex, err := m.protocol.ReadAck()
	if err != nil {
		return err
	}

	// if duration is more than one second, just pause the execution
	if time.Until(until) > time.Second {
		panic(&suspend{entryIndex})
	}

	// we can suspend invocation now
	response, err := m.protocol.Read()
	if err != nil {
		return err
	}

	if response.Type() != wire.CompletionMessageType {
		return wire.ErrUnexpectedMessage
	}

	return nil
}

func (m *Machine) sideEffect(fn func() ([]byte, error), bo backoff.BackOff) ([]byte, error) {
	return replayOrNew(
		m,
		wire.SideEffectEntryMessageType,
		func(entry *wire.SideEffectEntryMessage) ([]byte, error) {
			switch result := entry.Payload.Result.(type) {
			case *javascript.SideEffectEntryMessage_Failure:
				err := fmt.Errorf("[%d] %s", result.Failure.Failure.Code, result.Failure.Failure.Message)
				if result.Failure.Terminal {
					err = restate.TerminalError(err)
				}
				return nil, err
			case *javascript.SideEffectEntryMessage_Value:
				return result.Value, nil
			}

			return nil, errUnreachable
		},
		func() ([]byte, error) {
			return m._sideEffect(fn, bo)
		},
	)
}

func (m *Machine) _sideEffect(fn func() ([]byte, error), bo backoff.BackOff) ([]byte, error) {
	var bytes []byte
	err := backoff.Retry(func() error {
		var err error
		bytes, err = fn()

		if restate.IsTerminalError(err) {
			// if inner function returned a terminal error
			// we need to wrap it in permanent to break
			// the retries
			return backoff.Permanent(err)
		}
		return err
	}, bo)

	var msg javascript.SideEffectEntryMessage
	if err != nil {
		msg.Result = &javascript.SideEffectEntryMessage_Failure{
			Failure: &javascript.FailureWithTerminal{
				Failure: &protocol.Failure{
					Code:    uint32(restate.ErrorCode(err)),
					Message: err.Error(),
				},
				Terminal: restate.IsTerminalError(err),
			},
		}
	} else {
		msg.Result = &javascript.SideEffectEntryMessage_Value{
			Value: bytes,
		}
	}

	if err := m.protocol.Write(&msg); err != nil {
		return nil, err
	}

	return bytes, err
}
