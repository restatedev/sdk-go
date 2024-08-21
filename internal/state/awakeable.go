package state

import (
	"bytes"

	"github.com/restatedev/sdk-go/encoding"
	protocol "github.com/restatedev/sdk-go/generated/dev/restate/service"
	"github.com/restatedev/sdk-go/internal/errors"
	"github.com/restatedev/sdk-go/internal/futures"
	"github.com/restatedev/sdk-go/internal/wire"
)

func (c *Machine) awakeable() *futures.Awakeable {
	entry, entryIndex := replayOrNew(
		c,
		func(entry *wire.AwakeableEntryMessage) *wire.AwakeableEntryMessage {
			return entry
		},
		c._awakeable,
	)

	return futures.NewAwakeable(c.suspensionCtx, c.request.ID, entry, entryIndex)
}

func (c *Machine) _awakeable() *wire.AwakeableEntryMessage {
	msg := &wire.AwakeableEntryMessage{}
	c.Write(msg)
	return msg
}

func (m *Machine) resolveAwakeable(id string, value []byte) {
	_, _ = replayOrNew(
		m,
		func(entry *wire.CompleteAwakeableEntryMessage) encoding.Void {
			messageValue, ok := entry.Result.(*protocol.CompleteAwakeableEntryMessage_Value)
			if entry.Id != id || !ok || !bytes.Equal(messageValue.Value, value) {
				panic(m.newEntryMismatch(&wire.CompleteAwakeableEntryMessage{
					CompleteAwakeableEntryMessage: protocol.CompleteAwakeableEntryMessage{
						Id:     id,
						Result: &protocol.CompleteAwakeableEntryMessage_Value{Value: value},
					},
				}, entry))
			}
			return encoding.Void{}
		},
		func() encoding.Void {
			m._resolveAwakeable(id, value)
			return encoding.Void{}
		},
	)
}

func (c *Machine) _resolveAwakeable(id string, value []byte) {
	c.Write(&wire.CompleteAwakeableEntryMessage{
		CompleteAwakeableEntryMessage: protocol.CompleteAwakeableEntryMessage{
			Id:     id,
			Result: &protocol.CompleteAwakeableEntryMessage_Value{Value: value},
		},
	})
}

func (m *Machine) rejectAwakeable(id string, reason error) {
	_, _ = replayOrNew(
		m,
		func(entry *wire.CompleteAwakeableEntryMessage) encoding.Void {
			messageFailure, ok := entry.Result.(*protocol.CompleteAwakeableEntryMessage_Failure)
			if entry.Id != id || !ok || messageFailure.Failure.Code != uint32(errors.ErrorCode(reason)) || messageFailure.Failure.Message != reason.Error() {
				panic(m.newEntryMismatch(&wire.CompleteAwakeableEntryMessage{
					CompleteAwakeableEntryMessage: protocol.CompleteAwakeableEntryMessage{
						Id: id,
						Result: &protocol.CompleteAwakeableEntryMessage_Failure{Failure: &protocol.Failure{
							Code:    uint32(errors.ErrorCode(reason)),
							Message: reason.Error(),
						}},
					},
				}, entry))
			}
			return encoding.Void{}
		},
		func() encoding.Void {
			m._rejectAwakeable(id, reason)
			return encoding.Void{}
		},
	)
}

func (c *Machine) _rejectAwakeable(id string, reason error) {
	c.Write(&wire.CompleteAwakeableEntryMessage{
		CompleteAwakeableEntryMessage: protocol.CompleteAwakeableEntryMessage{
			Id: id,
			Result: &protocol.CompleteAwakeableEntryMessage_Failure{Failure: &protocol.Failure{
				Code:    uint32(errors.ErrorCode(reason)),
				Message: reason.Error(),
			}},
		},
	})
}
