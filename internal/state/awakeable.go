package state

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/generated/proto/protocol"
	"github.com/restatedev/sdk-go/internal/wire"
)

const AWAKEABLE_IDENTIFIER_PREFIX = "prom_1"

func resultFromAwakeable(entry *wire.AwakeableEntryMessage) restate.Result[[]byte] {
	switch result := entry.Result.(type) {
	case *protocol.AwakeableEntryMessage_Value:
		return restate.Result[[]byte]{Value: result.Value}
	case *protocol.AwakeableEntryMessage_Failure:
		return restate.Result[[]byte]{Err: restate.TerminalError(ErrorFromFailure(result.Failure))}
	default:
		panic("unreachable")
	}
}

type completionAwakeable struct {
	ctx          context.Context
	invocationID []byte
	entryIndex   uint32
	entry        *wire.AwakeableEntryMessage
}

func (c *completionAwakeable) Id() string { return awakeableID(c.invocationID, c.entryIndex) }
func (c *completionAwakeable) Chan() <-chan restate.Result[[]byte] {
	ch := make(chan restate.Result[[]byte], 1)
	if c.entry.Completed() {
		// fast path
		ch <- resultFromAwakeable(c.entry)
		return ch
	}
	// slow path
	go func() {
		if err := c.entry.Await(c.ctx); err != nil {
			ch <- restate.Result[[]byte]{Err: err}
		} else {
			ch <- resultFromAwakeable(c.entry)
		}
	}()
	return ch
}

type completedAwakeable[T any] struct {
	invocationID []byte
	entryIndex   uint32
	result       restate.Result[T]
}

func (c *completedAwakeable[T]) Id() string { return awakeableID(c.invocationID, c.entryIndex) }
func (c *completedAwakeable[T]) Chan() <-chan restate.Result[T] {
	ch := make(chan restate.Result[T], 1)
	ch <- c.result
	return ch
}

func awakeableID(invocationID []byte, entryIndex uint32) string {
	bytes := make([]byte, 0, len(invocationID)+4)
	bytes = append(bytes, invocationID...)
	bytes = binary.BigEndian.AppendUint32(bytes, entryIndex)
	return "prom_1" + base64.RawURLEncoding.EncodeToString(bytes)
}

func (c *Machine) awakeable() (restate.Awakeable[[]byte], error) {
	awakeable, err := replayOrNew(
		c,
		func(entry *wire.AwakeableEntryMessage) (restate.Awakeable[[]byte], error) {
			return &completionAwakeable{ctx: c.ctx, entryIndex: c.entryIndex, invocationID: c.id, entry: entry}, nil
		},
		c._awakeable,
	)
	if err != nil {
		return nil, err
	}
	return awakeable, nil
}

func (c *Machine) _awakeable() (restate.Awakeable[[]byte], error) {
	msg := &wire.AwakeableEntryMessage{}
	if err := c.Write(msg); err != nil {
		return nil, err
	}
	return &completionAwakeable{ctx: c.ctx, entryIndex: c.entryIndex, invocationID: c.id, entry: msg}, nil
}

func (c *Machine) resolveAwakeable(id string, value []byte) error {
	_, err := replayOrNew(
		c,
		func(entry *wire.CompleteAwakeableEntryMessage) (restate.Void, error) {
			messageValue, ok := entry.Result.(*protocol.CompleteAwakeableEntryMessage_Value)
			if entry.Id != id || !ok || !bytes.Equal(messageValue.Value, value) {
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

func (c *Machine) rejectAwakeable(id string, reason error) error {
	_, err := replayOrNew(
		c,
		func(entry *wire.CompleteAwakeableEntryMessage) (restate.Void, error) {
			messageFailure, ok := entry.Result.(*protocol.CompleteAwakeableEntryMessage_Failure)
			if entry.Id != id || !ok || messageFailure.Failure.Code != uint32(restate.ErrorCode(reason)) || messageFailure.Failure.Message != reason.Error() {
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
