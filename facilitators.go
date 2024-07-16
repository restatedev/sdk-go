package restate

import (
	"github.com/restatedev/sdk-go/internal/futures"
	"github.com/restatedev/sdk-go/internal/options"
)

// GetAs helper function to get a key, returning a typed response instead of accepting a pointer.
// If there is no associated value with key, an error ErrKeyNotFound is returned
func GetAs[T any](ctx ObjectSharedContext, key string, options ...options.GetOption) (output T, err error) {
	err = ctx.Get(key, &output, options...)
	return
}

// RunAs helper function runs a Run function, returning a typed response instead of accepting a pointer
func RunAs[T any](ctx Context, fn func(RunContext) (T, error), options ...options.RunOption) (output T, err error) {
	err = ctx.Run(func(ctx RunContext) (any, error) {
		return fn(ctx)
	}, &output, options...)

	return
}

// TypedAwakeable is an extension of Awakeable which returns typed responses instead of accepting a pointer
type TypedAwakeable[T any] interface {
	// Id returns the awakeable ID, which can be stored or sent to a another service
	Id() string
	// Result blocks on receiving the result of the awakeable, storing the value it was
	// resolved with in output or otherwise returning the error it was rejected with.
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Result() (T, error)
	futures.Selectable
}

type typedAwakeable[T any] struct {
	Awakeable
}

func (t typedAwakeable[T]) Result() (output T, err error) {
	err = t.Awakeable.Result(&output)
	return
}

// AwakeableAs helper function to treat awakeable results as a particular type.
func AwakeableAs[T any](ctx Context, options ...options.AwakeableOption) TypedAwakeable[T] {
	return typedAwakeable[T]{ctx.Awakeable(options...)}
}

// TypedCallClient is an extension of CallClient which returns typed responses instead of accepting a pointer
type TypedCallClient[O any] interface {
	// RequestFuture makes a call and returns a handle on a future response
	RequestFuture(input any) (TypedResponseFuture[O], error)
	// Request makes a call and blocks on getting the response
	Request(input any) (O, error)
	SendClient
}

type typedCallClient[O any] struct {
	CallClient
}

func (t typedCallClient[O]) Request(input any) (output O, err error) {
	fut, err := t.CallClient.RequestFuture(input)
	if err != nil {
		return output, err
	}
	err = fut.Response(&output)
	return
}

func (t typedCallClient[O]) RequestFuture(input any) (TypedResponseFuture[O], error) {
	fut, err := t.CallClient.RequestFuture(input)
	if err != nil {
		return nil, err
	}
	return typedResponseFuture[O]{fut}, nil
}

// TypedResponseFuture is an extension of ResponseFuture which returns typed responses instead of accepting a pointer
type TypedResponseFuture[O any] interface {
	// Response blocks on the response to the call and returns it or the associated error
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Response() (O, error)
	futures.Selectable
}

type typedResponseFuture[O any] struct {
	ResponseFuture
}

func (t typedResponseFuture[O]) Response() (output O, err error) {
	err = t.ResponseFuture.Response(&output)
	return
}

// CallAs helper function to get typed responses instead of passing in a pointer
func CallAs[O any](client CallClient) TypedCallClient[O] {
	return typedCallClient[O]{client}
}
