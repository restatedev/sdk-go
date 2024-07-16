package restate

import (
	"context"
	"log/slog"
	"time"

	"github.com/restatedev/sdk-go/internal/futures"
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

	// Service gets a Service accessor by name where service
	// must be another service known by restate runtime
	// Note: use the CallAs and SendAs helper functions to send and receive serialised values
	Service(service, method string) CallClient[[]byte, []byte]

	// Object gets a Object accessor by name where object
	// must be another object known by restate runtime and
	// key is any string representing the key for the object
	// Note: use the CallAs and SendAs helper functions to send and receive serialised values
	Object(object, key, method string) CallClient[[]byte, []byte]

	// Run runs the function (fn), storing final results (including terminal errors)
	// durably in the journal, or otherwise for transient errors stopping execution
	// so Restate can retry the invocation. Replays will produce the same value, so
	// all non-deterministic operations (eg, generating a unique ID) *must* happen
	// inside Run blocks.
	// Note: use the RunAs helper function to serialise non-[]byte return values
	Run(fn func(RunContext) ([]byte, error)) ([]byte, error)

	// Awakeable returns a Restate awakeable; a 'promise' to a future
	// value or error, that can be resolved or rejected by other services.
	// Note: use the AwakeableAs helper function to deserialise the []byte value
	Awakeable() Awakeable[[]byte]
	// ResolveAwakeable allows an awakeable (not necessarily from this service) to be
	// resolved with a particular value.
	// Note: use the ResolveAwakeableAs helper function to provide a value to be serialised
	ResolveAwakeable(id string, value []byte)
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

type CallClient[I any, O any] interface {
	// RequestFuture makes a call and returns a handle on a future response
	RequestFuture(input I) (ResponseFuture[O], error)
	// Request makes a call and blocks on getting the response
	Request(input I) (O, error)
	SendClient[I]
}

type SendClient[I any] interface {
	// Send makes a one-way call which is executed in the background
	Send(input I, delay time.Duration) error
}

type ResponseFuture[O any] interface {
	// Response blocks on the response to the call
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Response() (O, error)
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
	// Key retrieves the key for this virtual object invocation. This is a no-op and is
	// always safe to call.
	Key() string
}

type ObjectSharedContext interface {
	Context
	KeyValueReader
	// Key retrieves the key for this virtual object invocation. This is a no-op and is
	// always safe to call.
	Key() string
}

type KeyValueReader interface {
	// Get gets value (bytes array) associated with key
	// If key does not exist, this function return a nil bytes array
	// Note: Use GetAs helper function to read serialised values
	Get(key string) []byte
	// Keys returns a list of all associated key
	Keys() []string
}

type KeyValueWriter interface {
	// Set sets a byte array against a key
	// Note: Use SetAs helper function to store serialised values
	Set(key string, value []byte)
	// Clear deletes a key
	Clear(key string)
	// ClearAll drops all stored state associated with key
	ClearAll()
}
