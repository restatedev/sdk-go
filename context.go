package restate

import (
	"context"
	"log/slog"

	"github.com/restatedev/sdk-go/internal/state"
)

// Context is passed to Restate service handlers and enables interaction with Restate
type Context interface {
	RunContext
	inner() *state.Context
}

// RunContext is passed to [Run] closures and provides the limited set of Restate operations that are safe to use there.
type RunContext interface {
	context.Context
	// Log obtains a handle on a slog.Logger which already has some useful fields (invocationID and method)
	// By default, this logger will not output messages if the invocation is currently replaying
	// The log handler can be set with `.WithLogger()` on the server object
	Log() *slog.Logger

	// Request gives extra information about the request that started this invocation
	Request() *state.Request
}

// ObjectContext is an extension of [Context] which is passed to exclusive-mode Virtual Object handlers.
// giving mutable access to state.
type ObjectContext interface {
	ObjectSharedContext
}

// ObjectContext is an extension of [Context] which is passed to shared-mode Virtual Object handlers,
// giving read-only access to a snapshot of state.
type ObjectSharedContext interface {
	Context
}
