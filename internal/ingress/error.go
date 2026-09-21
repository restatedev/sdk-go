package ingress

import (
	"errors"
	"fmt"
	"net/http"

	restate "github.com/restatedev/sdk-go"
)

// errorSourceHeader is the header restate-server sets to disclose whether an error
// originated in the invocation (the handler ran and terminally failed) or in the ingress
// itself. See https://github.com/restatedev/restate/pull/5173.
const errorSourceHeader = "x-restate-error-source"

// errorSourceInvocation is the errorSourceHeader value for an invocation-sourced error.
const errorSourceInvocation = "invocation"

// statusNotReady is the non-standard HTTP status the Restate ingress returns when an
// invocation's output is requested but the invocation has not completed yet.
const statusNotReady = 470

// Error is implemented by every error the ingress client returns for a non-2xx response
// from the Restate ingress. It exposes the Restate error code, the message, an optional
// stacktrace, and whether the failure came from the invocation (the handler ran and
// terminally failed) or from the ingress itself.
//
// Network failures and (un)marshaling failures are NOT reported as an Error - only a
// non-2xx HTTP response is.
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
// model, so restate.AsTerminalError(err) and restate.IsTerminalError(err) work on it, just
// as inside a handler. The not-found and not-ready conditions are concrete Error types,
// matched with errors.As against [InvocationNotFoundError] and [InvocationNotReadyError].
type Error interface {
	error
	// Code returns the Restate error code (HTTP-status-like).
	Code() restate.Code
	// Message returns the error message.
	Message() string
	// Stacktrace returns the stacktrace attached to the error, if any; it may be empty.
	Stacktrace() string
	// IsInvocationError reports whether the failure came from the invocation - the handler
	// ran and returned a terminal error - as opposed to the ingress itself (routing, auth,
	// overload, ...). It is false when the server did not disclose the source.
	IsInvocationError() bool
}

// restateError is the JSON error body returned by the Restate ingress.
type restateError struct {
	Message    string `json:"message"`
	Code       int    `json:"code,omitempty"`
	Stacktrace string `json:"stacktrace,omitempty"`
}

// ingressError is the default [Error] implementation and the embedded base of the concrete
// not-found / not-ready types.
type ingressError struct {
	code       restate.Code
	message    string
	stacktrace string
	invocation bool
	terminal   restate.TerminalError
}

func (e *ingressError) Error() string {
	return fmt.Sprintf("ingress request failed [%d]: %s", e.code, e.message)
}

func (e *ingressError) Code() restate.Code { return e.code }

func (e *ingressError) Message() string { return e.message }

func (e *ingressError) Stacktrace() string { return e.stacktrace }

func (e *ingressError) IsInvocationError() bool { return e.invocation }

// Unwrap bridges an invocation-sourced error to the SDK terminal-error model: when
// IsInvocationError reports true it returns a restate.TerminalError with the same code and
// message, so restate.AsTerminalError and restate.IsTerminalError reach it. It returns nil
// for ingress/transport errors, which are not terminal.
func (e *ingressError) Unwrap() error {
	if e.terminal == nil {
		return nil
	}
	return e.terminal
}

// InvocationNotFoundError is returned when the invocation, service or handler was not found
// (HTTP 404). It is an [Error].
type InvocationNotFoundError struct {
	ingressError
}

// InvocationNotReadyError is returned when the invocation exists but has not completed yet
// (HTTP 470), for example from InvocationHandle.Output. It is an [Error].
type InvocationNotReadyError struct {
	ingressError
}

var (
	_ Error = (*ingressError)(nil)
	_ Error = (*InvocationNotFoundError)(nil)
	_ Error = (*InvocationNotReadyError)(nil)
)

// newError builds the appropriate [Error] for a non-2xx response. source is the value of
// the errorSourceHeader (empty when the server did not disclose it).
func newError(httpStatus int, source string, rerr *restateError) Error {
	code := restate.Code(rerr.Code)
	if code == 0 {
		code = restate.Code(httpStatus)
	}
	base := ingressError{
		code:       code,
		message:    rerr.Message,
		stacktrace: rerr.Stacktrace,
		invocation: source == errorSourceInvocation,
	}
	if base.invocation {
		// Bridge to the SDK terminal-error model so restate.AsTerminalError /
		// restate.IsTerminalError work on ingress-client errors (see Unwrap).
		base.terminal = restate.ToTerminalError(errors.New(base.message), restate.WithErrorCode(code))
	}
	switch httpStatus {
	case http.StatusNotFound:
		return &InvocationNotFoundError{base}
	case statusNotReady:
		return &InvocationNotReadyError{base}
	}
	return &base
}
