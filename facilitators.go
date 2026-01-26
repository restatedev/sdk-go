package restate

import (
	"iter"
	rand2 "math/rand/v2"
	"time"

	"github.com/google/uuid"
	"github.com/restatedev/sdk-go/internal/converters"
	"github.com/restatedev/sdk-go/internal/restatecontext"

	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/rand"
)

// Rand returns a random source which will give deterministic results for a given invocation
// The source wraps the stdlib rand.Rand but with some extra helper methods
// This source is not safe for use inside .Run()
func Rand(ctx Context) rand.Rand {
	return ctx.inner().Rand()
}

// UUID returns a random UUID seeded deterministically for a given invocation.
func UUID(ctx Context) uuid.UUID {
	return ctx.inner().Rand().UUID()
}

// RandSource returns a random source to be used with math rand implementations.
//
// To create a random implementation, use `rand2.New(RandSource(ctx))`
func RandSource(ctx Context) rand2.Source {
	return ctx.inner().Rand().Source()
}

// Sleep for the duration d. Can return a terminal error in the case where the invocation was cancelled mid-sleep.
func Sleep(ctx Context, d time.Duration, opts ...options.SleepOption) error {
	return ctx.inner().Sleep(d, opts...)
}

// After is an alternative to [Sleep] which allows you to complete other tasks concurrently
// with the sleep. This is particularly useful when combined with [WaitFirst] to race between
// the sleep and other [Future] operations.
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

// Deprecated: Please use WaitFirst or Wait or WaitIter instead
func Select(ctx Context, futs ...restatecontext.Selectable) Selector {
	return ctx.inner().Select(futs...)
}

// Deprecated: use Future instead.
type Selectable = restatecontext.Selectable

// Future is a marker interface for futures.
type Future = restatecontext.Selectable

// Deprecated: Please use WaitFirst or Wait or WaitIter instead
type Selector = restatecontext.Selector

// WaitFirst waits for the first Future to complete among the provided Futures and returns it.
// If the invocation is canceled, a cancellation error is returned.
//
// Example:
//
//	func MyHandler(ctx restate.Context, input string) (string, error) {
//		fut1 := restate.Service[string](ctx, "service1", "method1").RequestFuture(input)
//		fut2 := restate.After(ctx, 5 * time.Second)
//		fut3 := restate.Service[string](ctx, "service2", "method2").RequestFuture(input)
//
//		firstComplete, err := restate.WaitFirst(ctx, fut1, fut2, fut3)
//		if err != nil {
//			return "", err
//		}
//		// Handle the first completed future
//		switch firstComplete {
//		case fut1:
//			return fut1.Response()
//		case fut2:
//			return "", fmt.Errorf("timeout")
//		case fut3:
//			return fut3.Response()
//		default:
//			return "", fmt.Errorf("unknown future")
//		}
//	}
func WaitFirst(ctx Context, futs ...Future) (resultFut Future, cancellationError error) {
	set := WaitIter(ctx, futs...)
	set.Next()
	resultFut, cancellationError = set.Value(), set.Err()
	return
}

// Wait returns an iterator that yields Futures as they complete in order of completion.
// The iterator continues until all Futures have completed or a cancellation error occurs.
// If a cancellation error occurs, it is yielded as the final element with a nil Future.
//
// Example:
//
//	func MyHandler(ctx restate.Context, input string) ([]string, error) {
//		fut1 := restate.Service[string](ctx, "service1", "method1").RequestFuture(input)
//		fut2 := restate.Service[string](ctx, "service2", "method2").RequestFuture(input)
//		fut3 := restate.Service[string](ctx, "service3", "method3").RequestFuture(input)
//
//		results := []string{}
//		for fut, err := range restate.Wait(ctx, fut1, fut2, fut3) {
//			if err != nil {
//				return nil, err
//			}
//			result, err := fut.(restate.ResponseFuture[string]).Response()
//			if err != nil {
//				return nil, err
//			}
//			results = append(results, result)
//		}
//		return results, nil
//	}
func Wait(ctx Context, futs ...Future) iter.Seq2[Future, error] {
	set := WaitIter(ctx, futs...)

	return func(yield func(Future, error) bool) {
		for set.Next() {
			value := set.Value()
			if value == nil {
				break
			}
			if !yield(value, nil) {
				return
			}
		}
		if err := set.Err(); err != nil {
			yield(nil, err)
		}
	}
}

// WaitIter returns an iterator that allows manual control over waiting for multiple Futures to complete.
// This is the low-level primitive that WaitFirst and Wait are built on top of.
// Call Next() to wait for the next Future to complete, then use Value() to retrieve it and Err() to check for errors.
//
// Example:
//
//	func MyHandler(ctx restate.Context, input string) (string, error) {
//		fut1 := restate.Service[string](ctx, "service1", "method1").RequestFuture(input)
//		fut2 := restate.Service[string](ctx, "service2", "method2").RequestFuture(input)
//
//		iter := restate.WaitIter(ctx, fut1, fut2)
//		for iter.Next() {
//			fut := iter.Value()
//			// Process each future as it completes
//			if fut == fut1 {
//				result, _ := fut1.Response()
//				fmt.Printf("fut1 completed with: %s\n", result)
//			}
//		}
//		if err := iter.Err(); err != nil {
//			return "", err
//		}
//		return "all done", nil
//	}
func WaitIter(ctx Context, futs ...Future) WaitIterator {
	return ctx.inner().WaitIter(futs...)
}

// WaitIterator is an iterator over a list of blocking Restate operations that are running
// in the background. See WaitIter for more details.
type WaitIterator = restatecontext.WaitIterator

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
// This is similar to Run, but it returns a RunAsyncFuture instead that can be used within a WaitFirst, Wait.
//
// IMPORTANT: Only use the RunContext parameter provided to the function, NOT the
// handler's Context. See the Run function documentation for detailed examples and guidelines.
func RunAsync[T any](ctx Context, fn func(ctx RunContext) (T, error), options ...options.RunOption) RunAsyncFuture[T] {
	return converters.RunAsyncFuture[T]{RunAsyncFuture: ctx.inner().RunAsync(func(ctx RunContext) (any, error) {
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
func RunVoid(ctx Context, fn func(ctx RunContext) error, options ...options.RunOption) error {
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
