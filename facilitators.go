package restate

import (
	"github.com/restatedev/sdk-go/internal/converters"
	"github.com/restatedev/sdk-go/internal/restatecontext"
	"time"

	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/rand"
)

// Rand returns a random source which will give deterministic results for a given invocation
// The source wraps the stdlib rand.Rand but with some extra helper methods
// This source is not safe for use inside .Run()
func Rand(ctx Context) rand.Rand {
	return ctx.inner().Rand()
}

// Sleep for the duration d. Can return a terminal error in the case where the invocation was cancelled mid-sleep.
func Sleep(ctx Context, d time.Duration, opts ...options.SleepOption) error {
	return ctx.inner().Sleep(d, opts...)
}

// After is an alternative to [Sleep] which allows you to complete other tasks concurrently
// with the sleep. This is particularly useful when combined with [Select] to race between
// the sleep and other Selectable operations.
func After(ctx Context, d time.Duration, opts ...options.SleepOption) AfterFuture {
	return ctx.inner().After(d, opts...)
}

// After is a handle on a Sleep operation which allows you to do other work concurrently
// with the sleep.
type AfterFuture = restatecontext.AfterFuture

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

// Workflow gets a Workflow request client by service name, workflow ID and method name
func Workflow[O any](ctx Context, service string, workflowID string, method string, options ...options.ClientOption) Client[any, O] {
	return outputClient[O]{ctx.inner().Workflow(service, workflowID, method, options...)}
}

// WorkflowSend gets a Workflow send client by service name, workflow ID and method name
func WorkflowSend(ctx Context, service string, workflowID string, method string, options ...options.ClientOption) SendClient[any] {
	return ctx.inner().Workflow(service, workflowID, method, options...)
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
	Send(input I, options ...options.SendOption) Invocation
}

type outputClient[O any] struct {
	inner restatecontext.Client
}

func (t outputClient[O]) Request(input any, options ...options.RequestOption) (output O, err error) {
	err = t.inner.Request(input, &output, options...)
	return
}

func (t outputClient[O]) RequestFuture(input any, options ...options.RequestOption) ResponseFuture[O] {
	return converters.ResponseFuture[O]{ResponseFuture: t.inner.RequestFuture(input, options...)}
}

func (t outputClient[O]) Send(input any, options ...options.SendOption) Invocation {
	return t.inner.Send(input, options...)
}

type client[I any, O any] struct {
	inner Client[any, O]
}

// WithRequestType is primarily intended to be called from generated code, to provide
// type safety of input types. In other contexts it's generally less cumbersome to use [Object] and [Service],
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

func (t client[I, O]) Send(input I, options ...options.SendOption) Invocation {
	return t.inner.Send(input, options...)
}

// ResponseFuture is a handle on a potentially not-yet completed outbound call.
type ResponseFuture[O any] interface {
	// Response blocks on the response to the call and returns it or the associated error
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Response() (O, error)
	Invocation
	restatecontext.Selectable
}

type Invocation = restatecontext.Invocation

// CancelInvocation cancels the invocation with the given invocationId.
// For more info about cancellations, see https://docs.restate.dev/operate/invocation/#cancelling-invocations
func CancelInvocation(ctx Context, invocationId string) {
	ctx.inner().CancelInvocation(invocationId)
}

// AttachFuture is a handle on a potentially not-yet completed call.
type AttachFuture[O any] interface {
	// Response blocks on the response to the call and returns it or the associated error
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Response() (O, error)
	restatecontext.Selectable
}

// AttachInvocation attaches to the invocation with the given invocation id.
func AttachInvocation[T any](ctx Context, invocationId string, options ...options.AttachOption) AttachFuture[T] {
	return converters.AttachFuture[T]{AttachFuture: ctx.inner().AttachInvocation(invocationId, options...)}
}

// Awakeable returns a Restate awakeable; a 'promise' to a future
// value or error, that can be resolved or rejected by other services.
func Awakeable[T any](ctx Context, options ...options.AwakeableOption) AwakeableFuture[T] {
	return converters.AwakeableFuture[T]{AwakeableFuture: ctx.inner().Awakeable(options...)}
}

// AwakeableFuture is a 'promise' to a future value or error, that can be resolved or rejected by other services.
type AwakeableFuture[T any] interface {
	// Id returns the awakeable ID, which can be stored or sent to a another service
	Id() string
	// Result blocks on receiving the result of the awakeable, returning the value it was
	// resolved or otherwise returning the error it was rejected with.
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Result() (T, error)
	restatecontext.Selectable
}

// ResolveAwakeable allows an awakeable (not necessarily from this service) to be
// resolved with a particular value.
func ResolveAwakeable[T any](ctx Context, id string, value T, options ...options.ResolveAwakeableOption) {
	ctx.inner().ResolveAwakeable(id, value, options...)
}

// ResolveAwakeable allows an awakeable (not necessarily from this service) to be
// rejected with a particular error.
func RejectAwakeable(ctx Context, id string, reason error) {
	ctx.inner().RejectAwakeable(id, reason)
}

func Select(ctx Context, futs ...restatecontext.Selectable) Selector {
	return ctx.inner().Select(futs...)
}

// Selectable is a marker interface for futures that can be selected over with [Select]
type Selectable = restatecontext.Selectable

// Selector is an iterator over a list of blocking Restate operations that are running
// in the background.
type Selector = restatecontext.Selector

// Run runs the function (fn), storing final results (including terminal errors)
// durably in the journal, or otherwise for transient errors stopping execution
// so Restate can retry the invocation. Replays will produce the same value, so
// all non-deterministic operations (eg, generating a unique ID) *must* happen
// inside Run blocks.
func Run[T any](ctx Context, fn func(ctx RunContext) (T, error), options ...options.RunOption) (output T, err error) {
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
// This is similar to Run, but it returns a RunAsyncFuture instead that can be used within a Select.
func RunAsync[T any](ctx Context, fn func(ctx RunContext) (T, error), options ...options.RunOption) RunAsyncFuture[T] {
	return converters.RunAsyncFuture[T]{RunAsyncFuture: ctx.inner().RunAsync(func(ctx RunContext) (any, error) {
		return fn(ctx)
	}, options...)}
}

// RunAsyncFuture is a 'promise' for a RunAsync operation.
type RunAsyncFuture[T any] interface {
	// Result blocks on receiving the RunAsync result, returning the value it was
	// resolved or otherwise returning the error it was rejected with.
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Result() (T, error)
	restatecontext.Selectable
}

// Get gets the value for a key. If there is no associated value with key, the zero value is returned.
// To check explicitly for this case pass a pointer eg *string as T.
// If the invocation was cancelled while obtaining the state (only possible if eager state is disabled),
// a cancellation error is returned.
func Get[T any](ctx ObjectSharedContext, key string, options ...options.GetOption) (output T, err error) {
	_, err = ctx.inner().Get(key, &output, options...)
	return output, err
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

// Promise returns a named Restate durable Promise  that can be resolved or rejected during the workflow execution.
// The promise is bound to the workflow and will be persisted across suspensions and retries.
func Promise[T any](ctx WorkflowSharedContext, name string, options ...options.PromiseOption) DurablePromise[T] {
	return durablePromise[T]{ctx.inner().Promise(name, options...)}
}

type DurablePromise[T any] interface {
	// Result blocks on receiving the result of the Promise, returning the value it was
	// resolved or otherwise returning the error it was rejected with or a cancellation error.
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Result() (T, error)
	// Peek returns the value of the promise if it has been resolved. If it has not been resolved,
	// the zero value of T is returned. To check explicitly for this case pass a pointer eg *string as T.
	// If the promise was rejected or the invocation was cancelled, an error is returned.
	Peek() (T, error)
	// Resolve resolves the promise with a value, returning an error if it was already completed
	// or if the invocation was cancelled.
	Resolve(value T) error
	// Reject rejects the promise with an error, returning an error if it was already completed
	// or if the invocation was cancelled.
	Reject(reason error) error
	restatecontext.Selectable
}

type durablePromise[T any] struct {
	restatecontext.DurablePromise
}

func (t durablePromise[T]) Result() (output T, err error) {
	err = t.DurablePromise.Result(&output)
	return
}

func (t durablePromise[T]) Peek() (output T, err error) {
	_, err = t.DurablePromise.Peek(&output)
	return
}

func (t durablePromise[T]) Resolve(value T) (err error) {
	return t.DurablePromise.Resolve(value)
}
