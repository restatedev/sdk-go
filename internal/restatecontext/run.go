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
	"time"
)

func (restateCtx *ctx) Run(fn func(ctx RunContext) (any, error), output any, opts ...options.RunOption) error {
	o := options.RunOptions{}
	for _, opt := range opts {
		opt.BeforeRun(&o)
	}
	if o.Codec == nil {
		o.Codec = encoding.JSONCodec
	}

	handle, err := restateCtx.stateMachine.SysRun(restateCtx, "")
	if err != nil {
		panic(err)
	}

	restateCtx.runClosures[handle] = func() *pbinternal.VmProposeRunCompletionParameters {
		now := time.Now()

		// Run the user closure
		output, err := fn(runContext{Context: restateCtx, log: restateCtx.userLogger, request: &restateCtx.request})

		// Let's prepare the proposal of the run completion
		proposal := pbinternal.VmProposeRunCompletionParameters{}
		proposal.SetHandle(handle)
		proposal.SetAttemptDurationMillis(uint64(time.Now().Sub(now).Milliseconds()))
		if errors.IsTerminalError(err) {
			// Terminal error
			failure := pbinternal.Failure{}
			failure.SetCode(uint32(errors.ErrorCode(err)))
			failure.SetMessage(err.Error())
			proposal.SetTerminalFailure(&failure)
		} else if err != nil {
			// Retryable error
			failure := pbinternal.Failure{}
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

	ar := newAsyncResult(restateCtx, handle)
	switch result := ar.pollProgressAndLoadValue().(type) {
	case statemachine.ValueSuccess:
		{
			if err := encoding.Unmarshal(o.Codec, result.Success, output); err != nil {
				panic(fmt.Errorf("failed to unmarshal run result into output: %w", err))
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
