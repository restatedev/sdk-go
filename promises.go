package restate

import (
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/restatecontext"
)

// PromiseOption is an option for [Promise].
type PromiseOption = options.PromiseOption

// Promise returns a named Restate durable Promise that can be resolved or rejected during the workflow execution.
// The promise is bound to the workflow and will be persisted across suspensions and retries.
func Promise[T any](ctx WorkflowSharedContext, name string, options ...options.PromiseOption) DurablePromise[T] {
	return durablePromise[T]{ctx.inner().Promise(name, options...)}
}

type DurablePromise[T any] interface {
	// Result blocks on receiving the result of the Promise, returning the value it was
	// resolved or otherwise returning the error it was rejected with or a cancellation error.
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Result() (T, TerminalError)
	// Peek returns the value of the promise if it has been resolved. If it has not been resolved,
	// the zero value of T is returned. To check explicitly for this case pass a pointer eg *string as T.
	// If the promise was rejected or the invocation was cancelled, an error is returned.
	Peek() (T, TerminalError)
	// Resolve resolves the promise with a value, returning an error if it was already completed
	// or if the invocation was cancelled.
	Resolve(value T) TerminalError
	// Reject rejects the promise with an error, returning an error if it was already completed
	// or if the invocation was cancelled.
	Reject(reason error) TerminalError
	restatecontext.Future
}

type durablePromise[T any] struct {
	restatecontext.DurablePromise
}

func (t durablePromise[T]) Result() (output T, err TerminalError) {
	err = t.DurablePromise.Result(&output)
	return
}

func (t durablePromise[T]) Peek() (output T, err TerminalError) {
	_, err = t.DurablePromise.Peek(&output)
	return
}

func (t durablePromise[T]) Resolve(value T) TerminalError {
	return t.DurablePromise.Resolve(value)
}
