package futures

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/restatedev/sdk-go/generated/proto/protocol"
	"github.com/restatedev/sdk-go/internal/errors"
	"github.com/restatedev/sdk-go/internal/wire"
)

var (
	_ Selectable = (*After)(nil)
	_ Selectable = (*Awakeable)(nil)
	_ Selectable = (*ResponseFuture)(nil)
)

type After struct {
	suspensionCtx context.Context
	entry         *wire.SleepEntryMessage
	entryIndex    uint32
}

func NewAfter(suspensionCtx context.Context, entry *wire.SleepEntryMessage, entryIndex uint32) *After {
	return &After{suspensionCtx, entry, entryIndex}
}

func (a *After) Done() {
	a.entry.Await(a.suspensionCtx, a.entryIndex)
}

func (a *After) getEntry() (wire.CompleteableMessage, uint32, error) {
	return a.entry, a.entryIndex, nil
}

const AWAKEABLE_IDENTIFIER_PREFIX = "prom_1"

type Awakeable struct {
	suspensionCtx context.Context
	invocationID  []byte
	entry         *wire.AwakeableEntryMessage
	entryIndex    uint32
}

func NewAwakeable(suspensionCtx context.Context, invocationID []byte, entry *wire.AwakeableEntryMessage, entryIndex uint32) *Awakeable {
	return &Awakeable{suspensionCtx, invocationID, entry, entryIndex}
}

func (c *Awakeable) Id() string { return awakeableID(c.invocationID, c.entryIndex) }
func (c *Awakeable) Result() ([]byte, error) {
	c.entry.Await(c.suspensionCtx, c.entryIndex)

	switch result := c.entry.Result.(type) {
	case *protocol.AwakeableEntryMessage_Value:
		return result.Value, nil
	case *protocol.AwakeableEntryMessage_Failure:
		return nil, errors.ErrorFromFailure(result.Failure)
	default:
		return nil, fmt.Errorf("unexpected result in completed awakeable entry: %v", c.entry.Result)
	}
}
func (c *Awakeable) getEntry() (wire.CompleteableMessage, uint32, error) {
	return c.entry, c.entryIndex, nil
}

func awakeableID(invocationID []byte, entryIndex uint32) string {
	bytes := make([]byte, 0, len(invocationID)+4)
	bytes = append(bytes, invocationID...)
	bytes = binary.BigEndian.AppendUint32(bytes, entryIndex)
	return "prom_1" + base64.RawURLEncoding.EncodeToString(bytes)
}

type ResponseFuture struct {
	suspensionCtx context.Context
	err           error
	entry         *wire.CallEntryMessage
	entryIndex    uint32
}

func NewResponseFuture(suspensionCtx context.Context, entry *wire.CallEntryMessage, entryIndex uint32) *ResponseFuture {
	return &ResponseFuture{suspensionCtx, nil, entry, entryIndex}
}

func NewFailedResponseFuture(err error) *ResponseFuture {
	return &ResponseFuture{nil, err, nil, 0}
}

func (r *ResponseFuture) Response(output any) error {
	if r.err != nil {
		return r.err
	}

	r.entry.Await(r.suspensionCtx, r.entryIndex)

	var bytes []byte
	switch result := r.entry.Result.(type) {
	case *protocol.CallEntryMessage_Failure:
		return errors.ErrorFromFailure(result.Failure)
	case *protocol.CallEntryMessage_Value:
		bytes = result.Value
	default:
		return errors.NewTerminalError(fmt.Errorf("sync call had invalid result: %v", r.entry.Result), 571)

	}

	if err := json.Unmarshal(bytes, output); err != nil {
		// TODO: is this should be a terminal error or not?
		return errors.NewTerminalError(fmt.Errorf("failed to decode response (%s): %w", string(bytes), err))
	}

	return nil
}

func (r *ResponseFuture) getEntry() (wire.CompleteableMessage, uint32, error) {
	return r.entry, r.entryIndex, r.err
}
