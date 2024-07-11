package state

import (
	"bytes"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/generated/proto/protocol"
	"github.com/restatedev/sdk-go/internal/futures"
	"github.com/restatedev/sdk-go/internal/wire"
)

type indexedEntry struct {
	entry      *wire.AwakeableEntryMessage
	entryIndex uint32
}

func (c *Machine) awakeable() (restate.Awakeable[[]byte], error) {
	indexedEntry, err := replayOrNew(
		c,
		func(entry *wire.AwakeableEntryMessage) indexedEntry {
			return indexedEntry{entry, c.entryIndex}
		},
		c._awakeable,
	)
	if err != nil {
		return nil, err
	}

	return futures.NewAwakeable(c.ctx, c.id, indexedEntry.entryIndex, indexedEntry.entry), nil
}

func (c *Machine) _awakeable() indexedEntry {
	msg := &wire.AwakeableEntryMessage{}
	c.Write(msg)
	return indexedEntry{msg, c.entryIndex}
}

func (m *Machine) resolveAwakeable(id string, value []byte) error {
	_, err := replayOrNew(
		m,
		func(entry *wire.CompleteAwakeableEntryMessage) restate.Void {
			messageValue, ok := entry.Result.(*protocol.CompleteAwakeableEntryMessage_Value)
			if entry.Id != id || !ok || !bytes.Equal(messageValue.Value, value) {
				panic(m.newEntryMismatch(&wire.CompleteAwakeableEntryMessage{
					CompleteAwakeableEntryMessage: protocol.CompleteAwakeableEntryMessage{
						Id:     id,
						Result: &protocol.CompleteAwakeableEntryMessage_Value{Value: value},
					},
				}, entry))
			}
			return restate.Void{}
		},
		func() restate.Void {
			m._resolveAwakeable(id, value)
			return restate.Void{}
		},
	)
	return err
}

func (c *Machine) _resolveAwakeable(id string, value []byte) {
	c.Write(&wire.CompleteAwakeableEntryMessage{
		CompleteAwakeableEntryMessage: protocol.CompleteAwakeableEntryMessage{
			Id:     id,
			Result: &protocol.CompleteAwakeableEntryMessage_Value{Value: value},
		},
	})
}

func (m *Machine) rejectAwakeable(id string, reason error) error {
	_, err := replayOrNew(
		m,
		func(entry *wire.CompleteAwakeableEntryMessage) restate.Void {
			messageFailure, ok := entry.Result.(*protocol.CompleteAwakeableEntryMessage_Failure)
			if entry.Id != id || !ok || messageFailure.Failure.Code != uint32(restate.ErrorCode(reason)) || messageFailure.Failure.Message != reason.Error() {
				panic(m.newEntryMismatch(&wire.CompleteAwakeableEntryMessage{
					CompleteAwakeableEntryMessage: protocol.CompleteAwakeableEntryMessage{
						Id: id,
						Result: &protocol.CompleteAwakeableEntryMessage_Failure{Failure: &protocol.Failure{
							Code:    uint32(restate.ErrorCode(reason)),
							Message: reason.Error(),
						}},
					},
				}, entry))
			}
			return restate.Void{}
		},
		func() restate.Void {
			m._rejectAwakeable(id, reason)
			return restate.Void{}
		},
	)
	return err
}

func (c *Machine) _rejectAwakeable(id string, reason error) error {
	c.Write(&wire.CompleteAwakeableEntryMessage{
		CompleteAwakeableEntryMessage: protocol.CompleteAwakeableEntryMessage{
			Id: id,
			Result: &protocol.CompleteAwakeableEntryMessage_Failure{Failure: &protocol.Failure{
				Code:    uint32(restate.ErrorCode(reason)),
				Message: reason.Error(),
			}},
		},
	})
	return nil
}
