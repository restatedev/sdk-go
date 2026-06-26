package restate

import (
	"github.com/restatedev/sdk-go/internal/genericfutures"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/restatecontext"
)

// SignalOption is an option for [Signal].
type SignalOption = options.SignalOption

// ResolveSignalOption is an option for [ResolveSignal].
type ResolveSignalOption = options.ResolveSignalOption

// Signal returns a future for a signal by name.
func Signal[T any](ctx Context, name string, options ...options.SignalOption) SignalFuture[T] {
	return genericfutures.SignalFuture[T]{SignalFuture: ctx.inner().Signal(name, options...)}
}

// SignalFuture is a promise to a future signal value or error.
type SignalFuture[T any] interface {
	// Result blocks on receiving the result of the signal, returning the value it was
	// resolved with or the error it was rejected with.
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Result() (T, TerminalError)
	restatecontext.Future
}

// ResolveSignal resolves a signal on an invocation with a particular value.
func ResolveSignal[T any](ctx Context, invocationID string, name string, value T, options ...options.ResolveSignalOption) {
	ctx.inner().ResolveSignal(invocationID, name, value, options...)
}

// RejectSignal rejects a signal on an invocation with a particular error.
func RejectSignal(ctx Context, invocationID string, name string, reason error) {
	ctx.inner().RejectSignal(invocationID, name, reason)
}
