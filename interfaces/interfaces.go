package interfaces

import (
	"github.com/restatedev/sdk-go/internal/futures"
	"github.com/restatedev/sdk-go/internal/options"
)

type Selectable = futures.Selectable

// After is a handle on a Sleep operation which allows you to do other work concurrently
// with the sleep.
type After interface {
	// Done blocks waiting on the remaining duration of the sleep.
	// It is *not* safe to call this in a goroutine - use Context.Select if you want to wait on multiple
	// results at once. Can return a terminal error in the case where the invocation was cancelled mid-sleep,
	// hence Done() should always be called, even after using Context.Select.
	Done() error
	Selectable
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
	Selectable
}

// Client represents all the different ways you can invoke a particular service/key/method tuple.
type Client interface {
	// RequestFuture makes a call and returns a handle on a future response
	RequestFuture(input any, options ...options.RequestOption) ResponseFuture
	// Request makes a call and blocks on getting the response which is stored in output
	Request(input any, output any, options ...options.RequestOption) error
	SendClient
}

type SendClient interface {
	// Send makes a one-way call which is executed in the background
	Send(input any, options ...options.SendOption)
}

// ResponseFuture is a handle on a potentially not-yet completed outbound call.
type ResponseFuture interface {
	// Response blocks on the response to the call and stores it in output, or returns the associated error
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Response(output any) error
	Selectable
}

// Selector is an iterator over a list of blocking Restate operations that are running
// in the background.
type Selector interface {
	// Remaining returns whether there are still operations that haven't been returned by Select().
	// There will always be exactly the same number of results as there were operations
	// given to Context.Select
	Remaining() bool
	// Select blocks on the next completed operation or returns nil if there are none left
	Select() Selectable
}
