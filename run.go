package restate

import (
	"github.com/restatedev/sdk-go/internal/genericfutures"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/restatecontext"
)

// RunOption is an option for [Run], [RunAsync] and [RunVoid].
type RunOption = options.RunOption

// Run runs the function (fn), storing final results (including terminal errors)
// durably in the journal, or otherwise for transient errors stopping execution
// so Restate can retry the invocation. Replays will produce the same value, so
// all non-deterministic operations (eg, generating a unique ID) *must* happen
// inside Run blocks.
//
// Inside Run blocks, you can only:
//   - Perform non-deterministic operations (random number generation, external API calls, etc.)
//   - Use standard Go operations (math, string manipulation, etc.)
//
// You CANNOT use inside Run blocks:
//   - Any Restate SDK operations that require the handler Context
//
// See: https://docs.restate.dev/develop/go/durable-steps
//
// IMPORTANT: Only use the RunContext parameter provided to the function, NOT the
// handler's Context. The RunContext parameter intentionally shadows the handler
// context to prevent accidental misuse. Using the handler context inside Run leads
// to concurrency issues and undefined behavior.
//
// Example:
//
//	func (s *Service) MyHandler(ctx restate.Context, input string) (string, error) {
//		result, err := restate.Run(ctx, func(ctx restate.RunContext) (string, error) {
//			// Use the RunContext parameter 'ctx' here - it shadows the handler context
//			return doNonDeterministicOperation(ctx)
//		})
//		return result, err
//	}
//
// Example (INCORRECT - DO NOT DO THIS):
//
//	func (s *Service) MyHandler(ctx restate.Context, input string) (string, error) {
//		result, err := restate.Run(ctx, func(runCtx restate.RunContext) (string, error) {
//			// WRONG: Using handler context 'ctx' instead of 'runCtx'
//			return doNonDeterministicOperation(ctx)  // This will cause concurrency issues!
//		})
//		return result, err
//	}
func Run[T any](ctx Context, fn func(ctx RunContext) (T, error), options ...options.RunOption) (output T, err TerminalError) {
	err = ctx.inner().Run(func(ctx RunContext) (any, error) {
		return fn(ctx)
	}, &output, options...)

	return
}

// RunAsync runs the function (fn), storing final results (including terminal errors)
// durably in the journal, or otherwise for transient errors stopping execution
// so Restate can retry the invocation. Replays will produce the same value, so
// all non-deterministic operations (eg, generating a unique ID) *must* happen
// inside Run blocks.
//
// This is similar to Run, but it returns a RunAsyncFuture instead that can be used within a WaitFirst, Wait.
//
// IMPORTANT: Only use the RunContext parameter provided to the function, NOT the
// handler's Context. See the Run function documentation for detailed examples and guidelines.
func RunAsync[T any](ctx Context, fn func(ctx RunContext) (T, error), options ...options.RunOption) RunAsyncFuture[T] {
	return genericfutures.RunAsyncFuture[T]{RunAsyncFuture: ctx.inner().RunAsync(func(ctx RunContext) (any, error) {
		return fn(ctx)
	}, options...)}
}

// RunVoid runs the function (fn), storing final results (including terminal errors)
// durably in the journal, or otherwise for transient errors stopping execution
// so Restate can retry the invocation. Replays will produce the same value, so
// all non-deterministic operations (eg, generating a unique ID) *must* happen
// inside RunVoid blocks.
//
// This is similar to Run, but for functions that don't return a value.
//
// IMPORTANT: Only use the RunContext parameter provided to the function, NOT the
// handler's Context. See the Run function documentation for detailed examples and guidelines.
func RunVoid(ctx Context, fn func(ctx RunContext) error, options ...options.RunOption) TerminalError {
	var output Void
	err := ctx.inner().Run(func(ctx RunContext) (any, error) {
		return nil, fn(ctx)
	}, &output, options...)

	return err
}

// RunAsyncFuture is a 'promise' for a RunAsync operation.
type RunAsyncFuture[T any] interface {
	// Result blocks on receiving the RunAsync result, returning the value it was
	// resolved or otherwise returning the error it was rejected with.
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Result() (T, TerminalError)
	restatecontext.Future
}
