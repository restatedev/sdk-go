package restate

import (
	"context"
	"log/slog"

	"github.com/restatedev/sdk-go/internal/state"
)

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

// Context is an extension of [RunContext] which is passed to Restate service handlers and enables
// interaction with Restate
type Context interface {
	RunContext
	inner() *state.Context
}

// ObjectSharedContext is an extension of [Context] which is passed to shared-mode Virtual Object handlers,
// giving read-only access to a snapshot of state.
type ObjectSharedContext interface {
	Context
	object()
}

// ObjectContext is an extension of [ObjectSharedContext] which is passed to exclusive-mode Virtual Object handlers.
// giving mutable access to state.
type ObjectContext interface {
	ObjectSharedContext
	exclusiveObject()
}

// WorkflowSharedContext is an extension of [ObjectSharedContext] which is passed to shared-mode Workflow handlers,
// giving read-only access to a snapshot of state.
type WorkflowSharedContext interface {
	ObjectSharedContext
	workflow()
}

// WorkflowContext is an extension of [WorkflowSharedContext] and [ObjectContext] which is passed to Workflow 'run' handlers,
// giving mutable access to state.
type WorkflowContext interface {
	WorkflowSharedContext
	ObjectContext
	runWorkflow()
}
