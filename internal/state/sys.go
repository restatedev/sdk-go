package state

import (
	"bytes"
	"fmt"
	"time"

	"github.com/muhamadazmy/restate-sdk-go"
	"github.com/muhamadazmy/restate-sdk-go/generated/proto/protocol"
	"github.com/muhamadazmy/restate-sdk-go/internal/wire"
)

var (
	errEntryMismatch = restate.WithErrorCode(fmt.Errorf("log entry mismatch"), 32)
	errUnreachable   = fmt.Errorf("unreachable")
)

func (c *Machine) set(key string, value []byte) error {
	_, err := replayOrNew(
		c,
		wire.SetStateEntryMessageType,
		func(entry *wire.SetStateEntryMessage) (void restate.Void, err error) {
			if string(entry.Payload.Key) != key || !bytes.Equal(entry.Payload.Value, value) {
				return void, errEntryMismatch
			}

			return
		}, func() (void restate.Void, err error) {
			return void, c._set(key, value)
		})

	return err
}

func (c *Machine) _set(key string, value []byte) error {
	c.current[key] = value

	return c.protocol.Write(
		&protocol.SetStateEntryMessage{
			Key:   []byte(key),
			Value: value,
		})
}

func (c *Machine) clear(key string) error {
	_, err := replayOrNew(
		c,
		wire.ClearStateEntryMessageType,
		func(entry *wire.ClearStateEntryMessage) (void restate.Void, err error) {
			if string(entry.Payload.Key) != key {
				return void, errEntryMismatch
			}

			return void, nil
		}, func() (restate.Void, error) {
			return restate.Void{}, c._clear(key)
		},
	)

	return err
}

func (c *Machine) _clear(key string) error {
	return c.protocol.Write(
		&protocol.ClearStateEntryMessage{
			Key: []byte(key),
		},
	)
}

func (c *Machine) clearAll() error {
	_, err := replayOrNew(
		c,
		wire.ClearAllStateEntryMessageType,
		func(entry *wire.ClearAllStateEntryMessage) (void restate.Void, err error) {
			return
		}, func() (restate.Void, error) {
			return restate.Void{}, c._clearAll()
		},
	)

	return err
}

// clearAll drops all associated keys
func (c *Machine) _clearAll() error {
	return c.protocol.Write(
		&protocol.ClearAllStateEntryMessage{},
	)
}

func (c *Machine) get(key string) ([]byte, error) {
	return replayOrNew(
		c,
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
			return c._get(key)
		})
}

func (c *Machine) _get(key string) ([]byte, error) {
	msg := &protocol.GetStateEntryMessage{
		Key: []byte(key),
	}

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

func (c *Machine) sleep(until time.Time) error {
	_, err := replayOrNew(
		c,
		wire.SleepEntryMessageType,
		func(entry *wire.SleepEntryMessage) (void restate.Void, err error) {
			// we shouldn't verify the time because this would be different every time
			return
		}, func() (restate.Void, error) {
			return restate.Void{}, c._sleep(until)
		},
	)

	return err
}

func (c *Machine) _sleep(until time.Time) error {
	if err := c.protocol.Write(&protocol.SleepEntryMessage{
		WakeUpTime: uint64(until.UnixMilli()),
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
