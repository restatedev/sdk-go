package restatecontext

import (
	"fmt"

	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal/errors"
	pbinternal "github.com/restatedev/sdk-go/internal/generated"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/statemachine"
)

func (restateCtx *ctx) Promise(key string, opts ...options.PromiseOption) DurablePromise {
	o := options.PromiseOptions{}
	for _, opt := range opts {
		opt.BeforePromise(&o)
	}
	if o.Codec == nil {
		o.Codec = encoding.JSONCodec
	}

	handle, err := restateCtx.stateMachine.SysPromiseGet(restateCtx, key)
	if err != nil {
		panic(err)
	}
	restateCtx.checkStateTransition()

	return &durablePromise{
		asyncResult: newAsyncResult(restateCtx, handle),
		key:         key,
		codec:       o.Codec,
	}
}

type DurablePromise interface {
	Selectable
	Result(output any) (err error)
	Peek(output any) (ok bool, err error)
	Resolve(value any) error
	Reject(reason error) error
}

type durablePromise struct {
	asyncResult
	key   string
	codec encoding.Codec
}

func (d *durablePromise) Result(output any) (err error) {
	switch result := d.pollProgressAndLoadValue().(type) {
	case statemachine.ValueSuccess:
		{
			if err := encoding.Unmarshal(d.codec, result.Success, output); err != nil {
				panic(fmt.Errorf("failed to unmarshal promise result into output: %w", err))
			}
			return nil
		}
	case statemachine.ValueFailure:
		return errorFromFailure(result)
	default:
		panic(fmt.Errorf("unexpected value %s", result))

	}
}

func (d *durablePromise) Peek(output any) (ok bool, err error) {
	handle, err := d.ctx.stateMachine.SysPromisePeek(d.ctx, d.key)
	if err != nil {
		panic(err)
	}
	d.ctx.checkStateTransition()

	ar := newAsyncResult(d.ctx, handle)
	switch result := ar.pollProgressAndLoadValue().(type) {
	case statemachine.ValueVoid:
		return false, nil
	case statemachine.ValueSuccess:
		{
			if err := encoding.Unmarshal(d.codec, result.Success, output); err != nil {
				panic(fmt.Errorf("failed to unmarshal promise result into output: %w", err))
			}
			return true, nil
		}
	case statemachine.ValueFailure:
		return false, errorFromFailure(result)
	default:
		panic(fmt.Errorf("unexpected value %s", result))
	}
}

func (d *durablePromise) Resolve(value any) error {
	bytes, err := encoding.Marshal(d.codec, value)
	if err != nil {
		panic(fmt.Errorf("failed to marshal Promise Resolve value: %w", err))
	}

	input := pbinternal.VmSysPromiseCompleteParameters{}
	input.SetId(d.key)
	input.SetSuccess(bytes)
	input.SetNonDeterministicSerialization(isNonDeterministicCodec(d.codec))
	handle, err := d.ctx.stateMachine.SysPromiseComplete(d.ctx, &input)
	if err != nil {
		panic(err)
	}
	d.ctx.checkStateTransition()

	ar := newAsyncResult(d.ctx, handle)
	switch result := ar.pollProgressAndLoadValue().(type) {
	case statemachine.ValueVoid:
		return nil
	case statemachine.ValueFailure:
		return errorFromFailure(result)
	default:
		panic(fmt.Errorf("unexpected value %s", result))
	}
}

func (d *durablePromise) Reject(reason error) error {
	failure := pbinternal.Failure{}
	failure.SetCode(uint32(errors.ErrorCode(reason)))
	failure.SetMessage(reason.Error())

	input := pbinternal.VmSysPromiseCompleteParameters{}
	input.SetId(d.key)
	input.SetFailure(&failure)
	handle, err := d.ctx.stateMachine.SysPromiseComplete(d.ctx, &input)
	if err != nil {
		panic(err)
	}
	d.ctx.checkStateTransition()

	ar := newAsyncResult(d.ctx, handle)
	switch result := ar.pollProgressAndLoadValue().(type) {
	case statemachine.ValueVoid:
		return nil
	case statemachine.ValueFailure:
		return errorFromFailure(result)
	default:
		panic(fmt.Errorf("unexpected value %s", result))
	}
}
