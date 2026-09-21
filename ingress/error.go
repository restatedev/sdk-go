package ingress

import (
	"github.com/restatedev/sdk-go/internal/ingress"
)

// Error is implemented by every error the ingress [Client] returns for a non-2xx response
// from the Restate ingress. It exposes the Restate error code, the message, an optional
// stacktrace, and whether the failure came from the invocation (the handler ran and
// terminally failed) or from the ingress itself.
//
// Match it with errors.As:
//
//	var ie ingress.Error
//	if errors.As(err, &ie) {
//	    ie.Code()              // e.g. restate.Code(403)
//	    ie.IsInvocationError() // true if the handler terminally failed
//	}
//
// When IsInvocationError reports true, the error also satisfies the SDK terminal-error
// model, so restate.AsTerminalError(err) and restate.IsTerminalError(err) work on it.
type Error = ingress.Error

// InvocationNotFoundError is returned when the invocation, service or handler was not found
// (HTTP 404). It is an [Error]; match it with errors.As.
type InvocationNotFoundError = ingress.InvocationNotFoundError

// InvocationNotReadyError is returned when the invocation exists but has not completed yet
// (HTTP 470), for example from an InvocationHandle's Output. It is an [Error]; match it
// with errors.As.
type InvocationNotReadyError = ingress.InvocationNotReadyError
