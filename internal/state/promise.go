package state

import (
	"bytes"
	"fmt"

	"github.com/restatedev/sdk-go/encoding"
	protocol "github.com/restatedev/sdk-go/generated/dev/restate/service"
	"github.com/restatedev/sdk-go/internal/errors"
	"github.com/restatedev/sdk-go/internal/futures"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/wire"
)

func (c *ctx) Promise(key string, opts ...options.PromiseOption) DurablePromise {
	o := options.PromiseOptions{}
	for _, opt := range opts {
		opt.BeforePromise(&o)
	}
	if o.Codec == nil {
		o.Codec = encoding.JSONCodec
	}
	return decodingPromise{futures.NewPromise(c.machine.suspensionCtx, c.machine.request.ID, func() (*wire.GetPromiseEntryMessage, uint32) {
		return c.machine.getPromise(key)
	}), key, c.machine, o.Codec}
}

type DurablePromise interface {
	futures.Selectable
	Result(output any) (err error)
	Peek(output any) (ok bool, err error)
	Resolve(value any) error
	Reject(reason error) error
}

type decodingPromise struct {
	*futures.Promise
	key     string
	machine *Machine
	codec   encoding.Codec
}

func (d decodingPromise) Result(output any) (err error) {
	bytes, err := d.Promise.Result()
	if err != nil {
		return err
	}
	if err := encoding.Unmarshal(d.codec, bytes, output); err != nil {
		panic(d.machine.newCodecFailure(wire.GetPromiseEntryMessageType, fmt.Errorf("failed to unmarshal Promise result into output: %w", err)))
	}
	return
}

func (d decodingPromise) Peek(output any) (ok bool, err error) {
	bytes, ok, err := d.machine.peekPromise(d.key)
	if err != nil || !ok {
		return ok, err
	}
	if err := encoding.Unmarshal(d.codec, bytes, output); err != nil {
		panic(d.machine.newCodecFailure(wire.PeekPromiseEntryMessageType, fmt.Errorf("failed to unmarshal Promise result into output: %w", err)))
	}
	return
}

func (d decodingPromise) Resolve(value any) error {
	bytes, err := encoding.Marshal(d.codec, value)
	if err != nil {
		panic(d.machine.newCodecFailure(wire.CompletePromiseEntryMessageType, fmt.Errorf("failed to marshal Promise Resolve value: %w", err)))
	}
	return d.machine.resolvePromise(d.key, bytes)
}

func (d decodingPromise) Reject(reason error) error {
	return d.machine.rejectPromise(d.key, reason)
}

func (m *Machine) getPromise(key string) (*wire.GetPromiseEntryMessage, uint32) {
	return replayOrNew(
		m,
		func(entry *wire.GetPromiseEntryMessage) *wire.GetPromiseEntryMessage {
			if entry.Key != key {
				panic(m.newEntryMismatch(&wire.GetPromiseEntryMessage{
					GetPromiseEntryMessage: protocol.GetPromiseEntryMessage{
						Key: key,
					},
				}, entry))
			}
			return entry
		},
		func() *wire.GetPromiseEntryMessage {
			return m._getPromise(key)
		},
	)
}

func (c *Machine) _getPromise(key string) *wire.GetPromiseEntryMessage {
	msg := &wire.GetPromiseEntryMessage{
		GetPromiseEntryMessage: protocol.GetPromiseEntryMessage{
			Key: key,
		},
	}
	c.Write(msg)
	return msg
}

func (m *Machine) peekPromise(key string) ([]byte, bool, error) {
	entry, entryIndex := replayOrNew(
		m,
		func(entry *wire.PeekPromiseEntryMessage) *wire.PeekPromiseEntryMessage {
			if entry.Key != key {
				panic(m.newEntryMismatch(&wire.PeekPromiseEntryMessage{
					PeekPromiseEntryMessage: protocol.PeekPromiseEntryMessage{
						Key: key,
					},
				}, entry))
			}
			return entry
		},
		func() *wire.PeekPromiseEntryMessage {
			return m._peekPromise(key)
		},
	)

	entry.Await(m.suspensionCtx, entryIndex)

	switch value := entry.Result.(type) {
	case *protocol.PeekPromiseEntryMessage_Empty:
		return nil, false, nil
	case *protocol.PeekPromiseEntryMessage_Failure:
		return nil, false, errors.ErrorFromFailure(value.Failure)
	case *protocol.PeekPromiseEntryMessage_Value:
		return value.Value, true, nil
	default:
		panic(m.newProtocolViolation(entry, fmt.Errorf("peek promise entry had invalid result: %v", entry.Result)))
	}
}

func (c *Machine) _peekPromise(key string) *wire.PeekPromiseEntryMessage {
	msg := &wire.PeekPromiseEntryMessage{
		PeekPromiseEntryMessage: protocol.PeekPromiseEntryMessage{
			Key: key,
		},
	}
	c.Write(msg)
	return msg
}

func (m *Machine) resolvePromise(key string, value []byte) error {
	entry, entryIndex := replayOrNew(
		m,
		func(entry *wire.CompletePromiseEntryMessage) *wire.CompletePromiseEntryMessage {
			messageValue, ok := entry.Completion.(*protocol.CompletePromiseEntryMessage_CompletionValue)
			if entry.Key != key || !ok || !bytes.Equal(messageValue.CompletionValue, value) {
				panic(m.newEntryMismatch(&wire.CompletePromiseEntryMessage{
					CompletePromiseEntryMessage: protocol.CompletePromiseEntryMessage{
						Key:        key,
						Completion: &protocol.CompletePromiseEntryMessage_CompletionValue{CompletionValue: value},
					},
				}, entry))
			}
			return entry
		},
		func() *wire.CompletePromiseEntryMessage {
			return m._resolvePromise(key, value)
		},
	)

	entry.Await(m.suspensionCtx, entryIndex)

	switch value := entry.Result.(type) {
	case *protocol.CompletePromiseEntryMessage_Empty:
		return nil
	case *protocol.CompletePromiseEntryMessage_Failure:
		return errors.ErrorFromFailure(value.Failure)
	default:
		panic(m.newProtocolViolation(entry, fmt.Errorf("complete promise entry had invalid result: %v", entry.Result)))
	}
}

func (c *Machine) _resolvePromise(key string, value []byte) *wire.CompletePromiseEntryMessage {
	msg := &wire.CompletePromiseEntryMessage{
		CompletePromiseEntryMessage: protocol.CompletePromiseEntryMessage{
			Key:        key,
			Completion: &protocol.CompletePromiseEntryMessage_CompletionValue{CompletionValue: value},
		},
	}
	c.Write(msg)
	return msg
}

func (m *Machine) rejectPromise(key string, reason error) error {
	entry, entryIndex := replayOrNew(
		m,
		func(entry *wire.CompletePromiseEntryMessage) *wire.CompletePromiseEntryMessage {
			messageFailure, ok := entry.Result.(*protocol.CompletePromiseEntryMessage_Failure)
			if entry.Key != key || !ok || messageFailure.Failure.Code != uint32(errors.ErrorCode(reason)) || messageFailure.Failure.Message != reason.Error() {
				panic(m.newEntryMismatch(&wire.CompletePromiseEntryMessage{
					CompletePromiseEntryMessage: protocol.CompletePromiseEntryMessage{
						Key: key,
						Completion: &protocol.CompletePromiseEntryMessage_CompletionFailure{CompletionFailure: &protocol.Failure{
							Code:    uint32(errors.ErrorCode(reason)),
							Message: reason.Error(),
						}},
					},
				}, entry))
			}
			return entry
		},
		func() *wire.CompletePromiseEntryMessage {
			return m._rejectPromise(key, reason)
		},
	)

	entry.Await(m.suspensionCtx, entryIndex)

	switch value := entry.Result.(type) {
	case *protocol.CompletePromiseEntryMessage_Empty:
		return nil
	case *protocol.CompletePromiseEntryMessage_Failure:
		return errors.ErrorFromFailure(value.Failure)
	default:
		panic(m.newProtocolViolation(entry, fmt.Errorf("complete promise entry had invalid result: %v", entry.Result)))
	}
}

func (c *Machine) _rejectPromise(key string, reason error) *wire.CompletePromiseEntryMessage {
	msg := &wire.CompletePromiseEntryMessage{
		CompletePromiseEntryMessage: protocol.CompletePromiseEntryMessage{
			Key: key,
			Completion: &protocol.CompletePromiseEntryMessage_CompletionFailure{CompletionFailure: &protocol.Failure{
				Code:    uint32(errors.ErrorCode(reason)),
				Message: reason.Error(),
			}},
		},
	}
	c.Write(msg)
	return msg
}
