package state

import (
	"fmt"
	"time"

	"github.com/muhamadazmy/restate-sdk-go/generated/proto/protocol"
	"github.com/muhamadazmy/restate-sdk-go/internal/wire"
)

func (c *Machine) set(key string, value []byte) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.current[key] = value

	return c.protocol.Write(
		&protocol.SetStateEntryMessage{
			Key:   []byte(key),
			Value: value,
		})
}

func (c *Machine) clear(key string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.protocol.Write(
		&protocol.ClearStateEntryMessage{
			Key: []byte(key),
		},
	)
}

// clearAll drops all associated keys
func (c *Machine) clearAll() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.protocol.Write(
		&protocol.ClearAllStateEntryMessage{},
	)
}

func (c *Machine) get(key string) ([]byte, error) {
	msg := &protocol.GetStateEntryMessage{
		Key: []byte(key),
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

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
