package restate

import (
	"github.com/restatedev/sdk-go/internal/restatecontext"
)

// RunContext is passed to [Run] closures and provides the limited set of Restate operations that are safe to use there.
type RunContext = restatecontext.RunContext

// Request contains a set of information about the request that started an invocation
type Request = restatecontext.Request

// Context is an extension of [RunContext] which is passed to Restate service handlers and enables
// interaction with Restate
type Context interface {
	RunContext
	inner() restatecontext.Context
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
