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

func (c *Machine) _awakeable() (indexedEntry, error) {
	msg := &wire.AwakeableEntryMessage{}
	if err := c.Write(msg); err != nil {
		return indexedEntry{}, err
	}
	return indexedEntry{msg, c.entryIndex}, nil
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
		func() (restate.Void, error) {
			if err := m._resolveAwakeable(id, value); err != nil {
				return restate.Void{}, err
			}
			return restate.Void{}, nil
		},
	)
	return err
}

func (c *Machine) _resolveAwakeable(id string, value []byte) error {
	if err := c.Write(&wire.CompleteAwakeableEntryMessage{
		CompleteAwakeableEntryMessage: protocol.CompleteAwakeableEntryMessage{
			Id:     id,
			Result: &protocol.CompleteAwakeableEntryMessage_Value{Value: value},
		},
	}); err != nil {
		return err
	}
	return nil
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
		func() (restate.Void, error) {
			if err := m._rejectAwakeable(id, reason); err != nil {
				return restate.Void{}, err
			}
			return restate.Void{}, nil
		},
	)
	return err
}

func (c *Machine) _rejectAwakeable(id string, reason error) error {
	if err := c.Write(&wire.CompleteAwakeableEntryMessage{
		CompleteAwakeableEntryMessage: protocol.CompleteAwakeableEntryMessage{
			Id: id,
			Result: &protocol.CompleteAwakeableEntryMessage_Failure{Failure: &protocol.Failure{
				Code:    uint32(restate.ErrorCode(reason)),
				Message: reason.Error(),
			}},
		},
	}); err != nil {
		return err
	}
	return nil
}
