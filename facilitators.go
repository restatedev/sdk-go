package restate

import (
	"errors"
	"time"

	"github.com/restatedev/sdk-go/interfaces"
	"github.com/restatedev/sdk-go/internal/futures"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/rand"
)

// Rand returns a random source which will give deterministic results for a given invocation
// The source wraps the stdlib rand.Rand but with some extra helper methods
// This source is not safe for use inside .Run()
func Rand(ctx Context) *rand.Rand {
	return ctx.Rand()
}

// Sleep for the duration d. Can return a terminal error in the case where the invocation was cancelled mid-sleep.
func Sleep(ctx Context, d time.Duration) error {
	return ctx.Sleep(d)
}

// After is an alternative to [Sleep] which allows you to complete other tasks concurrently
// with the sleep. This is particularly useful when combined with [Select] to race between
// the sleep and other Selectable operations.
func After(ctx Context, d time.Duration) interfaces.After {
	return ctx.After(d)
}

// Service gets a Service request client by service and method name
func Service[O any](ctx Context, service string, method string, options ...options.ClientOption) TypedClient[any, O] {
	return typedClient[any, O]{ctx.Service(service, method, options...)}
}

// Service gets a Service send client by service and method name
func ServiceSend(ctx Context, service string, method string, options ...options.ClientOption) interfaces.SendClient {
	return ctx.Service(service, method, options...)
}

// Object gets an Object request client by service name, key and method name
func Object[O any](ctx Context, service string, key string, method string, options ...options.ClientOption) TypedClient[any, O] {
	return typedClient[any, O]{ctx.Object(service, key, method, options...)}
}

// ObjectSend gets an Object send client by service name, key and method name
func ObjectSend(ctx Context, service string, key string, method string, options ...options.ClientOption) interfaces.SendClient {
	return ctx.Object(service, key, method, options...)
}

// TypedClient is an extension of [interfaces.Client] and [interfaces.SendClient] which deals in typed values
type TypedClient[I any, O any] interface {
	// RequestFuture makes a call and returns a handle on a future response
	RequestFuture(input I, options ...options.RequestOption) TypedResponseFuture[O]
	// Request makes a call and blocks on getting the response
	Request(input I, options ...options.RequestOption) (O, error)
	// Send makes a one-way call which is executed in the background
	Send(input I, options ...options.SendOption)
}

type typedClient[I any, O any] struct {
	inner interfaces.Client
}

// NewTypedClient is primarily intended to be called from generated code, to provide
// type safety of input types. In other contexts it's generally less cumbersome to use [CallAs],
// as the output type can be inferred.
func NewTypedClient[I any, O any](client interfaces.Client) TypedClient[I, O] {
	return typedClient[I, O]{client}
}

func (t typedClient[I, O]) Request(input I, options ...options.RequestOption) (output O, err error) {
	err = t.inner.RequestFuture(input, options...).Response(&output)
	return
}

func (t typedClient[I, O]) RequestFuture(input I, options ...options.RequestOption) TypedResponseFuture[O] {
	return typedResponseFuture[O]{t.inner.RequestFuture(input, options...)}
}

func (t typedClient[I, O]) Send(input I, options ...options.SendOption) {
	t.inner.Send(input, options...)
}

// TypedResponseFuture is an extension of [ResponseFuture] which returns typed responses instead of accepting a pointer
type TypedResponseFuture[O any] interface {
	// Response blocks on the response to the call and returns it or the associated error
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Response() (O, error)
	futures.Selectable
}

type typedResponseFuture[O any] struct {
	interfaces.ResponseFuture
}

func (t typedResponseFuture[O]) Response() (output O, err error) {
	err = t.ResponseFuture.Response(&output)
	return
}

// Awakeable returns a Restate awakeable; a 'promise' to a future
// value or error, that can be resolved or rejected by other services.
func Awakeable[T any](ctx Context, options ...options.AwakeableOption) TypedAwakeable[T] {
	return typedAwakeable[T]{ctx.Awakeable(options...)}
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
	futures.Selectable
}

type typedAwakeable[T any] struct {
	interfaces.Awakeable
}

func (t typedAwakeable[T]) Result() (output T, err error) {
	err = t.Awakeable.Result(&output)
	return
}

// ResolveAwakeable allows an awakeable (not necessarily from this service) to be
// resolved with a particular value.
func ResolveAwakeable[T any](ctx Context, id string, value T, options ...options.ResolveAwakeableOption) {
	ctx.ResolveAwakeable(id, value, options...)
}

// ResolveAwakeable allows an awakeable (not necessarily from this service) to be
// rejected with a particular error.
func RejectAwakeable[T any](ctx Context, id string, reason error) {
	ctx.RejectAwakeable(id, reason)
}

func Select(ctx Context, futs ...interfaces.Selectable) interfaces.Selector {
	return ctx.Select(futs...)
}

// Run runs the function (fn), storing final results (including terminal errors)
// durably in the journal, or otherwise for transient errors stopping execution
// so Restate can retry the invocation. Replays will produce the same value, so
// all non-deterministic operations (eg, generating a unique ID) *must* happen
// inside Run blocks.
func Run[T any](ctx Context, fn func(ctx RunContext) (T, error), options ...options.RunOption) (output T, err error) {
	err = ctx.Run(func(ctx RunContext) (any, error) {
		return fn(ctx)
	}, &output, options...)

	return
}

// Get gets the value for a key. If there is no associated value with key, the zero value is returned.
// To check explicitly for this case use ctx.Get directly or pass a pointer eg *string as T.
// If the invocation was cancelled while obtaining the state (only possible if eager state is disabled),
// a cancellation error is returned.
func Get[T any](ctx KeyValueReader, key string, options ...options.GetOption) (output T, err error) {
	if err := ctx.Get(key, &output, options...); !errors.Is(err, ErrKeyNotFound) {
		return output, err
	} else {
		return output, nil
	}
}

// If the invocation was cancelled while obtaining the state (only possible if eager state is disabled),
// a cancellation error is returned.
func Keys(ctx KeyValueReader) ([]string, error) {
	return ctx.Keys()
}

// Key retrieves the key for this virtual object invocation. This is a no-op and is
// always safe to call.
func Key(ctx KeyValueReader) string {
	return ctx.Key()
}

// Set sets a value against a key, using the provided codec (defaults to JSON)
func Set[T any](ctx KeyValueWriter, key string, value T, options ...options.SetOption) {
	ctx.Set(key, value, options...)
}

// Clear deletes a key
func Clear(ctx KeyValueWriter, key string) {
	ctx.Clear(key)
}

// ClearAll drops all stored state associated with this Object key
func ClearAll(ctx KeyValueWriter) {
	ctx.ClearAll()
}
