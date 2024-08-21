package restate

import (
	"errors"
	"time"

	"github.com/restatedev/sdk-go/internal/futures"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/rand"
	"github.com/restatedev/sdk-go/internal/state"
)

// Rand returns a random source which will give deterministic results for a given invocation
// The source wraps the stdlib rand.Rand but with some extra helper methods
// This source is not safe for use inside .Run()
func Rand(ctx Context) *rand.Rand {
	return ctx.inner().Rand()
}

// Sleep for the duration d. Can return a terminal error in the case where the invocation was cancelled mid-sleep.
func Sleep(ctx Context, d time.Duration) error {
	return ctx.inner().Sleep(d)
}

// After is an alternative to [Sleep] which allows you to complete other tasks concurrently
// with the sleep. This is particularly useful when combined with [Select] to race between
// the sleep and other Selectable operations.
func After(ctx Context, d time.Duration) AfterFuture {
	return ctx.inner().After(d)
}

// After is a handle on a Sleep operation which allows you to do other work concurrently
// with the sleep.
type AfterFuture interface {
	// Done blocks waiting on the remaining duration of the sleep.
	// It is *not* safe to call this in a goroutine - use Context.Select if you want to wait on multiple
	// results at once. Can return a terminal error in the case where the invocation was cancelled mid-sleep,
	// hence Done() should always be called, even after using Context.Select.
	Done() error
	futures.Selectable
}

// Service gets a Service request client by service and method name
func Service[O any](ctx Context, service string, method string, options ...options.ClientOption) Client[any, O] {
	return outputClient[O]{ctx.inner().Service(service, method, options...)}
}

// Service gets a Service send client by service and method name
func ServiceSend(ctx Context, service string, method string, options ...options.ClientOption) SendClient[any] {
	return ctx.inner().Service(service, method, options...)
}

// Object gets an Object request client by service name, key and method name
func Object[O any](ctx Context, service string, key string, method string, options ...options.ClientOption) Client[any, O] {
	return outputClient[O]{ctx.inner().Object(service, key, method, options...)}
}

// ObjectSend gets an Object send client by service name, key and method name
func ObjectSend(ctx Context, service string, key string, method string, options ...options.ClientOption) SendClient[any] {
	return ctx.inner().Object(service, key, method, options...)
}

// Client represents all the different ways you can invoke a particular service-method.
type Client[I any, O any] interface {
	// RequestFuture makes a call and returns a handle on a future response
	RequestFuture(input I, options ...options.RequestOption) ResponseFuture[O]
	// Request makes a call and blocks on getting the response
	Request(input I, options ...options.RequestOption) (O, error)
	SendClient[I]
}

// SendClient allows making one-way invocations
type SendClient[I any] interface {
	// Send makes a one-way call which is executed in the background
	Send(input I, options ...options.SendOption)
}

type outputClient[O any] struct {
	inner *state.Client
}

func (t outputClient[O]) Request(input any, options ...options.RequestOption) (output O, err error) {
	err = t.inner.RequestFuture(input, options...).Response(&output)
	return
}

func (t outputClient[O]) RequestFuture(input any, options ...options.RequestOption) ResponseFuture[O] {
	return responseFuture[O]{t.inner.RequestFuture(input, options...)}
}

func (t outputClient[O]) Send(input any, options ...options.SendOption) {
	t.inner.Send(input, options...)
}

type client[I any, O any] struct {
	inner Client[any, O]
}

// WithRequestType is primarily intended to be called from generated code, to provide
// type safety of input types. In other contexts it's generally less cumbersome to use [CallAs],
// as the output type can be inferred.
func WithRequestType[I any, O any](inner Client[any, O]) Client[I, O] {
	return client[I, O]{inner}
}

func (t client[I, O]) Request(input I, options ...options.RequestOption) (output O, err error) {
	output, err = t.inner.RequestFuture(input, options...).Response()
	return
}

func (t client[I, O]) RequestFuture(input I, options ...options.RequestOption) ResponseFuture[O] {
	return t.inner.RequestFuture(input, options...)
}

func (t client[I, O]) Send(input I, options ...options.SendOption) {
	t.inner.Send(input, options...)
}

// ResponseFuture is a handle on a potentially not-yet completed outbound call.
type ResponseFuture[O any] interface {
	// Response blocks on the response to the call and returns it or the associated error
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Response() (O, error)
	futures.Selectable
}

type responseFuture[O any] struct {
	state.DecodingResponseFuture
}

func (t responseFuture[O]) Response() (output O, err error) {
	err = t.DecodingResponseFuture.Response(&output)
	return
}

// Awakeable returns a Restate awakeable; a 'promise' to a future
// value or error, that can be resolved or rejected by other services.
func Awakeable[T any](ctx Context, options ...options.AwakeableOption) AwakeableFuture[T] {
	return awakeable[T]{ctx.inner().Awakeable(options...)}
}

// AwakeableFuture is a 'promise' to a future value or error, that can be resolved or rejected by other services.
type AwakeableFuture[T any] interface {
	// Id returns the awakeable ID, which can be stored or sent to a another service
	Id() string
	// Result blocks on receiving the result of the awakeable, storing the value it was
	// resolved with in output or otherwise returning the error it was rejected with.
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Result() (T, error)
	futures.Selectable
}

type awakeable[T any] struct {
	state.DecodingAwakeable
}

func (t awakeable[T]) Result() (output T, err error) {
	err = t.DecodingAwakeable.Result(&output)
	return
}

// ResolveAwakeable allows an awakeable (not necessarily from this service) to be
// resolved with a particular value.
func ResolveAwakeable[T any](ctx Context, id string, value T, options ...options.ResolveAwakeableOption) {
	ctx.inner().ResolveAwakeable(id, value, options...)
}

// ResolveAwakeable allows an awakeable (not necessarily from this service) to be
// rejected with a particular error.
func RejectAwakeable[T any](ctx Context, id string, reason error) {
	ctx.inner().RejectAwakeable(id, reason)
}

func Select(ctx Context, futs ...futures.Selectable) Selector {
	return ctx.inner().Select(futs...)
}

type Selectable = futures.Selectable

// Selector is an iterator over a list of blocking Restate operations that are running
// in the background.
type Selector interface {
	// Remaining returns whether there are still operations that haven't been returned by Select().
	// There will always be exactly the same number of results as there were operations
	// given to Context.Select
	Remaining() bool
	// Select blocks on the next completed operation or returns nil if there are none left
	Select() futures.Selectable
}

// Run runs the function (fn), storing final results (including terminal errors)
// durably in the journal, or otherwise for transient errors stopping execution
// so Restate can retry the invocation. Replays will produce the same value, so
// all non-deterministic operations (eg, generating a unique ID) *must* happen
// inside Run blocks.
func Run[T any](ctx Context, fn func(ctx RunContext) (T, error), options ...options.RunOption) (output T, err error) {
	err = ctx.inner().Run(func(ctx state.RunContext) (any, error) {
		return fn(ctx)
	}, &output, options...)

	return
}

// Get gets the value for a key. If there is no associated value with key, the zero value is returned.
// To check explicitly for this case pass a pointer eg *string as T.
// If the invocation was cancelled while obtaining the state (only possible if eager state is disabled),
// a cancellation error is returned.
func Get[T any](ctx ObjectSharedContext, key string, options ...options.GetOption) (output T, err error) {
	if err := ctx.inner().Get(key, &output, options...); !errors.Is(err, ErrKeyNotFound) {
		return output, err
	} else {
		return output, nil
	}
}

// If the invocation was cancelled while obtaining the state (only possible if eager state is disabled),
// a cancellation error is returned.
func Keys(ctx ObjectSharedContext) ([]string, error) {
	return ctx.inner().Keys()
}

// Key retrieves the key for this virtual object invocation. This is a no-op and is
// always safe to call.
func Key(ctx ObjectSharedContext) string {
	return ctx.inner().Key()
}

// Set sets a value against a key, using the provided codec (defaults to JSON)
func Set[T any](ctx ObjectContext, key string, value T, options ...options.SetOption) {
	ctx.inner().Set(key, value, options...)
}

// Clear deletes a key
func Clear(ctx ObjectContext, key string) {
	ctx.inner().Clear(key)
}

// ClearAll drops all stored state associated with this Object key
func ClearAll(ctx ObjectContext) {
	ctx.inner().ClearAll()
}
