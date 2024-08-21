package restate

import (
	"errors"

	"github.com/restatedev/sdk-go/internal/options"
)

// GetAs gets the value for a key, returning a typed response instead of accepting a pointer.
// If there is no associated value with key, the zero value is returned - to check explicitly for this case use ctx.Get directly
// or pass a pointer eg *string as T.
// If the invocation was cancelled while obtaining the state (only possible if eager state is disabled),
// a cancellation error is returned.
func GetAs[T any](ctx ObjectSharedContext, key string, options ...options.GetOption) (output T, err error) {
	if err := ctx.Get(key, &output, options...); !errors.Is(err, ErrKeyNotFound) {
		return output, err
	} else {
		return output, nil
	}
}

// RunAs executes a Run function on a [Context], returning a typed response instead of accepting a pointer
func RunAs[T any](ctx Context, fn func(ctx RunContext) (T, error), options ...options.RunOption) (output T, err error) {
	err = ctx.Run(func(ctx RunContext) (any, error) {
		return fn(ctx)
	}, &output, options...)

	return
}

// TypedAwakeable is an extension of [Awakeable] which returns typed responses instead of accepting a pointer
type TypedAwakeable[T any] interface {
	// Id returns the awakeable ID, which can be stored or sent to a another service
	Id() string
	// Result blocks on receiving the result of the awakeable, storing the value it was
	// resolved with in output or otherwise returning the error it was rejected with.
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Result() (T, error)
	Selectable
}

type typedAwakeable[T any] struct {
	Awakeable
}

func (t typedAwakeable[T]) Result() (output T, err error) {
	err = t.Awakeable.Result(&output)
	return
}

// AwakeableAs helper function to treat [Awakeable] results as a particular type.
func AwakeableAs[T any](ctx Context, options ...options.AwakeableOption) TypedAwakeable[T] {
	return typedAwakeable[T]{ctx.Awakeable(options...)}
}

// TypedCallClient is an extension of [CallClient] which deals in typed values
type TypedCallClient[I any, O any] interface {
	// RequestFuture makes a call and returns a handle on a future response
	RequestFuture(input I, opts ...options.RequestOption) TypedResponseFuture[O]
	// Request makes a call and blocks on getting the response
	Request(input I, opts ...options.RequestOption) (O, error)
	// Send makes a one-way call which is executed in the background
	Send(input I, opts ...options.SendOption)
}

type typedCallClient[I any, O any] struct {
	inner CallClient
}

// NewTypedCallClient is primarily intended to be called from generated code, to provide
// type safety of input types. In other contexts it's generally less cumbersome to use [CallAs],
// as the output type can be inferred.
func NewTypedCallClient[I any, O any](client CallClient) TypedCallClient[I, O] {
	return typedCallClient[I, O]{client}
}

func (t typedCallClient[I, O]) Request(input I, opts ...options.RequestOption) (output O, err error) {
	err = t.inner.RequestFuture(input, opts...).Response(&output)
	return
}

func (t typedCallClient[I, O]) RequestFuture(input I, opts ...options.RequestOption) TypedResponseFuture[O] {
	return typedResponseFuture[O]{t.inner.RequestFuture(input, opts...)}
}

func (t typedCallClient[I, O]) Send(input I, opts ...options.SendOption) {
	t.inner.Send(input, opts...)
}

// TypedResponseFuture is an extension of [ResponseFuture] which returns typed responses instead of accepting a pointer
type TypedResponseFuture[O any] interface {
	// Response blocks on the response to the call and returns it or the associated error
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Response() (O, error)
	Selectable
}

type typedResponseFuture[O any] struct {
	ResponseFuture
}

func (t typedResponseFuture[O]) Response() (output O, err error) {
	err = t.ResponseFuture.Response(&output)
	return
}

// CallAs helper function to get typed responses from a [CallClient] instead of passing in a pointer
func CallAs[O any](client CallClient) TypedCallClient[any, O] {
	return typedCallClient[any, O]{client}
}
