package restate

import (
	"context"
	"log/slog"
	"time"

	"github.com/restatedev/sdk-go/internal/futures"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/rand"
)

type Context interface {
	RunContext

	// Rand returns a random source which will give deterministic results for a given invocation
	// The source wraps the stdlib rand.Rand but with some extra helper methods
	// This source is not safe for use inside .Run()
	Rand() *rand.Rand

	// Sleep for the duration d
	Sleep(d time.Duration)
	// After is an alternative to Context.Sleep which allows you to complete other tasks concurrently
	// with the sleep. This is particularly useful when combined with Context.Select to race between
	// the sleep and other Selectable operations.
	After(d time.Duration) After

	// Service gets a Service accessor by service and method name
	// Note: use the CallAs helper function to deserialise return values
	Service(service, method string, opts ...options.CallOption) CallClient

	// Object gets a Object accessor by name, key and method name
	// Note: use the CallAs helper function to receive serialised values
	Object(object, key, method string, opts ...options.CallOption) CallClient

	// Run runs the function (fn), storing final results (including terminal errors)
	// durably in the journal, or otherwise for transient errors stopping execution
	// so Restate can retry the invocation. Replays will produce the same value, so
	// all non-deterministic operations (eg, generating a unique ID) *must* happen
	// inside Run blocks.
	// Note: use the RunAs helper function to get typed output values instead of providing an output pointer
	Run(fn func(RunContext) (any, error), output any, opts ...options.RunOption) error

	// Awakeable returns a Restate awakeable; a 'promise' to a future
	// value or error, that can be resolved or rejected by other services.
	// Note: use the AwakeableAs helper function to avoid having to pass a output pointer to Awakeable.Result()
	Awakeable(options ...options.AwakeableOption) Awakeable
	// ResolveAwakeable allows an awakeable (not necessarily from this service) to be
	// resolved with a particular value.
	ResolveAwakeable(id string, value any, options ...options.ResolveAwakeableOption) error
	// ResolveAwakeable allows an awakeable (not necessarily from this service) to be
	// rejected with a particular error.
	RejectAwakeable(id string, reason error)

	// Select returns an iterator over blocking Restate operations (sleep, call, awakeable)
	// which allows you to safely run them in parallel. The Selector will store the order
	// that things complete in durably inside Restate, so that on replay the same order
	// can be used. This avoids non-determinism. It is *not* safe to use goroutines or channels
	// outside of Context.Run functions, as they do not behave deterministically.
	Select(futs ...futures.Selectable) Selector
}

// Awakeable is the Go representation of a Restate awakeable; a 'promise' to a future
// value or error, that can be resolved or rejected by other services.
type Awakeable interface {
	// Id returns the awakeable ID, which can be stored or sent to a another service
	Id() string
	// Result blocks on receiving the result of the awakeable, storing the value it was
	// resolved with in output or otherwise returning the error it was rejected with.
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	// Note: use the AwakeableAs helper function to avoid having to pass a output pointer
	Result(output any) error
	futures.Selectable
}

type CallClient interface {
	// RequestFuture makes a call and returns a handle on a future response
	RequestFuture(input any) (ResponseFuture, error)
	// Request makes a call and blocks on getting the response which is stored in output
	Request(input any, output any) error
	SendClient
}

type SendClient interface {
	// Send makes a one-way call which is executed in the background
	Send(input any, delay time.Duration) error
}

type ResponseFuture interface {
	// Response blocks on the response to the call and stores it in output, or returns the associated error
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Response(output any) error
	futures.Selectable
}

// Selector is an iterator over a list of blocking Restate operations that are running
// in the background.
type Selector interface {
	// Remaining returns whether there are still operations that haven't been returned by Select().
	// There will always be exactly the same number of results as there were operations
	// given to Context.Select
	Remaining() bool
	// Select blocks on the next completed operation
	Select() futures.Selectable
}

// RunContext methods are the only methods safe to call from inside a .Run()
type RunContext interface {
	context.Context

	// Log obtains a handle on a slog.Logger which already has some useful fields (invocationID and method)
	// By default, this logger will not output messages if the invocation is currently replaying
	// The log handler can be set with `.WithLogger()` on the server object
	Log() *slog.Logger
}

// After is a handle on a Sleep operation which allows you to do other work concurrently
// with the sleep.
type After interface {
	// Done blocks waiting on the remaining duration of the sleep.
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Done()
	futures.Selectable
}

type ObjectContext interface {
	Context
	KeyValueReader
	KeyValueWriter
}

type ObjectSharedContext interface {
	Context
	KeyValueReader
}

type KeyValueReader interface {
	// Get gets value associated with key and stores it in value
	// If key does not exist, this function returns ErrKeyNotFound
	// Note: Use GetAs generic helper function to avoid passing in a value pointer
	Get(key string, value any, options ...options.GetOption) error
	// Keys returns a list of all associated key
	Keys() []string
	// Key retrieves the key for this virtual object invocation. This is a no-op and is
	// always safe to call.
	Key() string
}

type KeyValueWriter interface {
	// Set sets a value against a key, using the provided codec (defaults to JSON)
	Set(key string, value any, options ...options.SetOption) error
	// Clear deletes a key
	Clear(key string)
	// ClearAll drops all stored state associated with key
	ClearAll()
}
