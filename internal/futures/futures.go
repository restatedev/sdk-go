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
	ctx   context.Context
	entry *wire.SleepEntryMessage
}

func NewAfter(ctx context.Context, entry *wire.SleepEntryMessage) *After {
	return &After{ctx, entry}
}

func (a *After) Done() error {
	return a.entry.Await(a.ctx)
}

func (a *After) getEntry() (wire.CompleteableMessage, error) {
	return a.entry, nil
}

const AWAKEABLE_IDENTIFIER_PREFIX = "prom_1"

type Awakeable struct {
	ctx          context.Context
	invocationID []byte
	entryIndex   uint32
	entry        *wire.AwakeableEntryMessage
}

func NewAwakeable(ctx context.Context, invocationID []byte, entryIndex uint32, entry *wire.AwakeableEntryMessage) *Awakeable {
	return &Awakeable{ctx, invocationID, entryIndex, entry}
}

func (c *Awakeable) Id() string { return awakeableID(c.invocationID, c.entryIndex) }
func (c *Awakeable) Result() ([]byte, error) {
	if err := c.entry.Await(c.ctx); err != nil {
		return nil, err
	} else {
		switch result := c.entry.Result.(type) {
		case *protocol.AwakeableEntryMessage_Value:
			return result.Value, nil
		case *protocol.AwakeableEntryMessage_Failure:
			return nil, errors.ErrorFromFailure(result.Failure)
		default:
			return nil, fmt.Errorf("unexpected result in completed awakeable entry: %v", c.entry.Result)
		}
	}
}
func (c *Awakeable) getEntry() (wire.CompleteableMessage, error) { return c.entry, nil }

func awakeableID(invocationID []byte, entryIndex uint32) string {
	bytes := make([]byte, 0, len(invocationID)+4)
	bytes = append(bytes, invocationID...)
	bytes = binary.BigEndian.AppendUint32(bytes, entryIndex)
	return "prom_1" + base64.RawURLEncoding.EncodeToString(bytes)
}

type ResponseFuture struct {
	ctx   context.Context
	err   error
	entry *wire.CallEntryMessage
}

func NewResponseFuture(ctx context.Context, entry *wire.CallEntryMessage) *ResponseFuture {
	return &ResponseFuture{ctx, nil, entry}
}

func NewFailedResponseFuture(ctx context.Context, err error) *ResponseFuture {
	return &ResponseFuture{ctx, err, nil}
}

func (r *ResponseFuture) Err() error {
	return r.err
}

func (r *ResponseFuture) Response(output any) error {
	if r.err != nil {
		return r.err
	}

	if err := r.entry.Await(r.ctx); err != nil {
		r.err = err
		return r.err
	}

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

func (r *ResponseFuture) getEntry() (wire.CompleteableMessage, error) {
	return r.entry, r.err
}
