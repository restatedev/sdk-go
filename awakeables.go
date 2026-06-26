package restate

import (
	"github.com/restatedev/sdk-go/internal/genericfutures"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/restatecontext"
)

// AwakeableOption is an option for [Awakeable].
type AwakeableOption = options.AwakeableOption

// ResolveAwakeableOption is an option for [ResolveAwakeable].
type ResolveAwakeableOption = options.ResolveAwakeableOption

// Awakeable returns a Restate awakeable; a 'promise' to a future
// value or error, that can be resolved or rejected by other services.
func Awakeable[T any](ctx Context, options ...options.AwakeableOption) AwakeableFuture[T] {
	return genericfutures.AwakeableFuture[T]{AwakeableFuture: ctx.inner().Awakeable(options...)}
}

// AwakeableFuture is a 'promise' to a future value or error, that can be resolved or rejected by other services.
type AwakeableFuture[T any] interface {
	// Id returns the awakeable ID, which can be stored or sent to a another service
	Id() string
	// Result blocks on receiving the result of the awakeable, returning the value it was
	// resolved or otherwise returning the error it was rejected with.
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Result() (T, TerminalError)
	restatecontext.Future
}

// ResolveAwakeable allows an awakeable (not necessarily from this service) to be
// resolved with a particular value.
func ResolveAwakeable[T any](ctx Context, id string, value T, options ...options.ResolveAwakeableOption) {
	ctx.inner().ResolveAwakeable(id, value, options...)
}

// RejectAwakeable allows an awakeable (not necessarily from this service) to be
// rejected with a particular error.
func RejectAwakeable(ctx Context, id string, reason error) {
	ctx.inner().RejectAwakeable(id, reason)
}
