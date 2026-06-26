package restatecontext

import (
	"fmt"

	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal/errors"
	pbinternal "github.com/restatedev/sdk-go/internal/generated"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/statemachine"
)

func (restateCtx *ctx) Signal(name string, opts ...options.SignalOption) SignalFuture {
	o := options.SignalOptions{}
	for _, opt := range opts {
		opt.BeforeSignal(&o)
	}
	if o.Codec == nil {
		o.Codec = encoding.JSONCodec
	}

	handle, err := restateCtx.stateMachine.SysSignal(restateCtx, name)
	if err != nil {
		panic(err)
	}
	restateCtx.checkStateTransition()

	return &signalFuture{
		asyncResult: newAsyncResult(restateCtx, handle),
		codec:       o.Codec,
	}
}

type SignalFuture interface {
	Future
	Result(output any) errors.TerminalError
}

type signalFuture struct {
	asyncResult
	codec encoding.Codec
}

func (d *signalFuture) Result(output any) errors.TerminalError {
	switch result := d.pollProgressAndLoadValue().(type) {
	case statemachine.ValueSuccess:
		if err := encoding.Unmarshal(d.codec, result.Success, output); err != nil {
			panic(fmt.Errorf("failed to unmarshal signal result into output: %w", err))
		}
		return nil
	case statemachine.ValueFailure:
		return errorFromFailure(result)
	default:
		panic(fmt.Errorf("unexpected value %s", result))
	}
}

func (restateCtx *ctx) ResolveSignal(invocationID string, name string, value any, opts ...options.ResolveSignalOption) {
	o := options.ResolveSignalOptions{}
	for _, opt := range opts {
		opt.BeforeResolveSignal(&o)
	}
	if o.Codec == nil {
		o.Codec = encoding.JSONCodec
	}
	bytes, err := encoding.Marshal(o.Codec, value)
	if err != nil {
		panic(fmt.Errorf("failed to marshal ResolveSignal value: %w", err))
	}

	input := pbinternal.VmSysCompleteSignalParameters{}
	input.SetInvocationId(invocationID)
	input.SetName(name)
	input.SetSuccess(bytes)
	input.SetUnstableSerialization(
		encoding.IsNonDeterministicSerialization(o.Codec),
	)
	if err := restateCtx.stateMachine.SysCompleteSignal(restateCtx, &input); err != nil {
		panic(err)
	}
	restateCtx.checkStateTransition()
}

func (restateCtx *ctx) RejectSignal(invocationID string, name string, reason error) {
	input := pbinternal.VmSysCompleteSignalParameters{}
	input.SetInvocationId(invocationID)
	input.SetName(name)
	input.SetFailure(newFailureFromError(reason))
	if err := restateCtx.stateMachine.SysCompleteSignal(restateCtx, &input); err != nil {
		panic(err)
	}
	restateCtx.checkStateTransition()
}
