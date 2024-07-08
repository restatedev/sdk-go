package state

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/generated/proto/protocol"
	"github.com/restatedev/sdk-go/internal/wire"
)

const AWAKEABLE_IDENTIFIER_PREFIX = "prom_1"

type awakeable[T any] interface {
	restate.Awakeable[T]
	setEntryIndex(entryIndex uint32)
}

type completedAwakeable[T any] struct {
	invocationID []byte
	entryIndex   uint32
	result       restate.Result[T]
}

func (c completedAwakeable[T]) Id() string { return awakeableID(c.invocationID, c.entryIndex) }
func (c completedAwakeable[T]) Chan() <-chan restate.Result[T] {
	ch := make(chan restate.Result[T], 1)
	ch <- c.result
	return ch
}
func (c completedAwakeable[T]) setEntryIndex(entryIndex uint32) { c.entryIndex = entryIndex }

type suspendingAwakeable[T any] struct {
	invocationID []byte
	entryIndex   uint32
}

func (c suspendingAwakeable[T]) Id() string { return awakeableID(c.invocationID, c.entryIndex) }

// this is a temporary hack; always suspend when this channel is read
// currently needed because we don't have a way to process the completion while the invocation is in progress
// and so can only deal with it on replay
func (c suspendingAwakeable[T]) Chan() <-chan restate.Result[T] {
	panic(&suspend{resumeEntry: c.entryIndex})
}
func (c suspendingAwakeable[T]) setEntryIndex(entryIndex uint32) { c.entryIndex = entryIndex }

func awakeableID(invocationID []byte, entryIndex uint32) string {
	bytes := make([]byte, 0, len(invocationID)+4)
	bytes = append(bytes, invocationID...)
	bytes = binary.BigEndian.AppendUint32(bytes, entryIndex)
	return base64.URLEncoding.EncodeToString(bytes)
}

func (c *Machine) awakeable() (restate.Awakeable[[]byte], error) {
	awakeable, err := replayOrNew(
		c,
		wire.AwakeableEntryMessageType,
		func(entry *wire.AwakeableEntryMessage) (awakeable[[]byte], error) {
			if entry.Payload.Result == nil {
				return suspendingAwakeable[[]byte]{invocationID: c.id}, nil
			}
			switch result := entry.Payload.Result.(type) {
			case *protocol.AwakeableEntryMessage_Value:
				return completedAwakeable[[]byte]{invocationID: c.id, result: restate.Result[[]byte]{Value: result.Value}}, nil
			case *protocol.AwakeableEntryMessage_Failure:
				return completedAwakeable[[]byte]{invocationID: c.id, result: restate.Result[[]byte]{Err: restate.TerminalError(fmt.Errorf(result.Failure.Message), restate.Code(result.Failure.Code))}}, nil
			default:
				return nil, restate.TerminalError(fmt.Errorf("awakeable entry had invalid result: %v", entry.Payload.Result), restate.ErrProtocolViolation)
			}
		},
		func() (awakeable[[]byte], error) {
			if err := c._awakeable(); err != nil {
				return nil, err
			}
			return suspendingAwakeable[[]byte]{invocationID: c.id}, nil
		},
	)
	if err != nil {
		return nil, err
	}
	// This needs to be done after handling the message in the state machine
	// otherwise the index is not yet incremented.
	awakeable.setEntryIndex(uint32(c.entryIndex))
	return awakeable, nil
}

func (c *Machine) _awakeable() error {
	if err := c.protocol.Write(&protocol.AwakeableEntryMessage{}); err != nil {
		return err
	}
	return nil
}

func (c *Machine) resolveAwakeable(id string, value []byte) error {
	_, err := replayOrNew(
		c,
		wire.CompleteAwakeableEntryMessageType,
		func(entry *wire.CompleteAwakeableEntryMessage) (restate.Void, error) {
			messageValue, ok := entry.Payload.Result.(*protocol.CompleteAwakeableEntryMessage_Value)
			if entry.Payload.Id != id || !ok || !bytes.Equal(messageValue.Value, value) {
				return restate.Void{}, errEntryMismatch
			}
			return restate.Void{}, nil
		},
		func() (restate.Void, error) {
			if err := c._resolveAwakeable(id, value); err != nil {
				return restate.Void{}, err
			}
			return restate.Void{}, nil
		},
	)
	return err
}

func (c *Machine) _resolveAwakeable(id string, value []byte) error {
	if err := c.protocol.Write(&protocol.CompleteAwakeableEntryMessage{
		Id:     id,
		Result: &protocol.CompleteAwakeableEntryMessage_Value{Value: value},
	}); err != nil {
		return err
	}
	return nil
}

func (c *Machine) rejectAwakeable(id string, reason error) error {
	_, err := replayOrNew(
		c,
		wire.CompleteAwakeableEntryMessageType,
		func(entry *wire.CompleteAwakeableEntryMessage) (restate.Void, error) {
			messageFailure, ok := entry.Payload.Result.(*protocol.CompleteAwakeableEntryMessage_Failure)
			if entry.Payload.Id != id || !ok || messageFailure.Failure.Code != uint32(restate.ErrorCode(reason)) || messageFailure.Failure.Message != reason.Error() {
				return restate.Void{}, errEntryMismatch
			}
			return restate.Void{}, nil
		},
		func() (restate.Void, error) {
			if err := c._rejectAwakeable(id, reason); err != nil {
				return restate.Void{}, err
			}
			return restate.Void{}, nil
		},
	)
	return err
}

func (c *Machine) _rejectAwakeable(id string, reason error) error {
	if err := c.protocol.Write(&protocol.CompleteAwakeableEntryMessage{
		Id: id,
		Result: &protocol.CompleteAwakeableEntryMessage_Failure{Failure: &protocol.Failure{
			Code:    uint32(restate.ErrorCode(reason)),
			Message: reason.Error(),
		}},
	}); err != nil {
		return err
	}
	return nil
}
