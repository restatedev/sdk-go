package restatecontext

import (
	"fmt"

	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal/errors"
	pbinternal "github.com/restatedev/sdk-go/internal/generated"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/statemachine"
)

func (restateCtx *ctx) Awakeable(opts ...options.AwakeableOption) AwakeableFuture {
	o := options.AwakeableOptions{}
	for _, opt := range opts {
		opt.BeforeAwakeable(&o)
	}
	if o.Codec == nil {
		o.Codec = encoding.JSONCodec
	}

	id, handle, err := restateCtx.stateMachine.SysAwakeable(restateCtx)
	if err != nil {
		panic(err)
	}
	restateCtx.checkStateTransition()

	return &awakeableFuture{
		asyncResult: newAsyncResult(restateCtx, handle),
		id:          id,
		codec:       o.Codec,
	}
}

type AwakeableFuture interface {
	Selectable
	Id() string
	Result(output any) error
}

type awakeableFuture struct {
	asyncResult
	id    string
	codec encoding.Codec
}

func (d *awakeableFuture) Id() string { return d.id }

func (d *awakeableFuture) Result(output any) error {
	switch result := d.pollProgressAndLoadValue().(type) {
	case statemachine.ValueSuccess:
		{
			if err := encoding.Unmarshal(d.codec, result.Success, output); err != nil {
				panic(fmt.Errorf("failed to unmarshal awakeable result into output: %w", err))
			}
			return nil
		}
	case statemachine.ValueFailure:
		return errorFromFailure(result)
	default:
		panic(fmt.Errorf("unexpected value %s", result))

	}
}

func (restateCtx *ctx) ResolveAwakeable(id string, value any, opts ...options.ResolveAwakeableOption) {
	o := options.ResolveAwakeableOptions{}
	for _, opt := range opts {
		opt.BeforeResolveAwakeable(&o)
	}
	if o.Codec == nil {
		o.Codec = encoding.JSONCodec
	}
	bytes, err := encoding.Marshal(o.Codec, value)
	if err != nil {
		panic(fmt.Errorf("failed to marshal ResolveAwakeable value: %w", err))
	}

	input := pbinternal.VmSysCompleteAwakeableParameters{}
	input.SetId(id)
	input.SetSuccess(bytes)
	input.SetUnstableSerialization(
		encoding.IsNonDeterministicSerialization(o.Codec),
	)
	if err := restateCtx.stateMachine.SysCompleteAwakeable(restateCtx, &input); err != nil {
		panic(err)
	}
	restateCtx.checkStateTransition()
}

func (restateCtx *ctx) RejectAwakeable(id string, reason error) {
	failure := pbinternal.Failure{}
	failure.SetCode(uint32(errors.ErrorCode(reason)))
	failure.SetMessage(reason.Error())

	input := pbinternal.VmSysCompleteAwakeableParameters{}
	input.SetId(id)
	input.SetFailure(&failure)
	if err := restateCtx.stateMachine.SysCompleteAwakeable(restateCtx, &input); err != nil {
		panic(err)
	}
	restateCtx.checkStateTransition()
}
