package restatecontext

import (
	"context"
	"fmt"
	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal/errors"
	pbinternal "github.com/restatedev/sdk-go/internal/generated"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/statemachine"
	"log/slog"
	"runtime/debug"
	"time"
)

func (restateCtx *ctx) Run(fn func(ctx RunContext) (any, error), output any, opts ...options.RunOption) error {
	return restateCtx.RunAsync(fn, opts...).Result(output)
}

func (restateCtx *ctx) RunAsync(fn func(ctx RunContext) (any, error), opts ...options.RunOption) RunAsyncFuture {
	o := options.RunOptions{}
	for _, opt := range opts {
		opt.BeforeRun(&o)
	}
	if o.Codec == nil {
		o.Codec = encoding.JSONCodec
	}

	params := pbinternal.VmSysRunParameters{}
	params.SetName(o.Name)

	handle, err := restateCtx.stateMachine.SysRun(restateCtx, o.Name)
	if err != nil {
		panic(err)
	}
	restateCtx.checkStateTransition()

	restateCtx.runClosures[handle] = func() *pbinternal.VmProposeRunCompletionParameters {
		now := time.Now()

		// Run the user closure
		output, err := runWrapPanic(fn)(runContext{Context: restateCtx, log: restateCtx.userLogger, request: &restateCtx.request})

		// Let's prepare the proposal of the run completion
		proposal := pbinternal.VmProposeRunCompletionParameters{}
		proposal.SetHandle(handle)
		proposal.SetAttemptDurationMillis(uint64(time.Now().Sub(now).Milliseconds()))

		// Set retry policy if any of the retry policy config options are set
		if o.MaxRetryAttempts != nil || o.MaxRetryInterval != nil || o.MaxRetryDuration != nil || o.RetryIntervalFactor != nil || o.InitialRetryInterval != nil {
			retryPolicy := pbinternal.VmProposeRunCompletionParameters_RetryPolicy{}
			retryPolicy.SetInitialInternalMillis(50)
			retryPolicy.SetFactor(2)
			retryPolicy.SetMaxIntervalMillis(2000)

			if o.MaxRetryDuration != nil {
				retryPolicy.SetMaxDurationMillis(uint64((*o.MaxRetryDuration).Milliseconds()))
			}
			if o.MaxRetryInterval != nil {
				retryPolicy.SetMaxIntervalMillis(uint64((*o.MaxRetryInterval).Milliseconds()))
			}
			if o.RetryIntervalFactor != nil {
				retryPolicy.SetFactor(*o.RetryIntervalFactor)
			}
			if o.MaxRetryAttempts != nil {
				retryPolicy.SetMaxAttempts(uint32(*o.MaxRetryAttempts))
			}
			if o.InitialRetryInterval != nil {
				retryPolicy.SetInitialInternalMillis(uint64((*o.InitialRetryInterval).Milliseconds()))
			}
			proposal.SetRetryPolicy(&retryPolicy)
		}

		if errors.IsTerminalError(err) {
			// Terminal error
			failure := pbinternal.Failure{}
			failure.SetCode(uint32(errors.ErrorCode(err)))
			failure.SetMessage(err.Error())
			proposal.SetTerminalFailure(&failure)
		} else if err != nil {
			// Retryable error
			failure := pbinternal.FailureWithStacktrace{}
			failure.SetCode(uint32(errors.ErrorCode(err)))
			failure.SetMessage(err.Error())
			proposal.SetRetryableFailure(&failure)
		} else {
			// Success
			bytes, err := encoding.Marshal(o.Codec, output)
			if err != nil {
				panic(fmt.Errorf("failed to marshal Run output: %w", err))
			}

			proposal.SetSuccess(bytes)
		}

		return &proposal
	}

	return &runAsyncFuture{
		asyncResult: newAsyncResult(restateCtx, handle),
		codec:       o.Codec,
	}
}

func runWrapPanic(fn func(ctx RunContext) (any, error)) func(ctx RunContext) (any, error) {
	return func(ctx RunContext) (res any, err error) {
		defer func() {
			recovered := recover()

			switch typ := recovered.(type) {
			case nil:
				// nothing to do, just exit
				break
			case *statemachine.SuspensionError:
			case statemachine.SuspensionError:
				err = typ
				break
			default:
				err = fmt.Errorf("panic occurred while executing Run: %s\nStack: %s", fmt.Sprint(typ), string(debug.Stack()))
				break
			}
		}()
		res, err = fn(ctx)
		return
	}
}

type RunAsyncFuture interface {
	Selectable
	Result(output any) error
}

type runAsyncFuture struct {
	asyncResult
	codec encoding.Codec
}

func (d *runAsyncFuture) Result(output any) error {
	switch result := d.pollProgressAndLoadValue().(type) {
	case statemachine.ValueSuccess:
		{
			if err := encoding.Unmarshal(d.codec, result.Success, output); err != nil {
				panic(fmt.Errorf("failed to unmarshal runAsync result into output: %w", err))
			}
			return nil
		}
	case statemachine.ValueFailure:
		return errorFromFailure(result)
	default:
		panic(fmt.Errorf("unexpected value %s", result))

	}
}

// RunContext is passed to [Run] closures and provides the limited set of Restate operations that are safe to use there.
type RunContext interface {
	context.Context

	// Log obtains a coreHandle on a slog.Logger which already has some useful fields (invocationID and method)
	// By default, this logger will not output messages if the invocation is currently replaying
	// The log handler can be set with `.WithLogger()` on the server object
	Log() *slog.Logger

	// Request gives extra information about the request that started this invocation
	Request() *Request
}

type runContext struct {
	context.Context
	log     *slog.Logger
	request *Request
}

func (r runContext) Log() *slog.Logger { return r.log }
func (r runContext) Request() *Request { return r.request }
