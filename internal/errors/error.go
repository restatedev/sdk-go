package errors

import (
	"errors"
	"fmt"
)

// Code is a numeric status code for an error, typically a HTTP status code.
type Code uint16

// DefaultCode is the code assigned to a terminal error when none is provided.
const DefaultCode Code = 500

// TerminalError finishes an invocation (or a Run function) with a failure result
// instead of being retried. It carries a status code, a message and optional
// metadata, and implements the error interface.
type TerminalError interface {
	error
	// Code returns the status code attached to the error.
	Code() Code
	// Message returns the error message.
	Message() string
	// Metadata returns the metadata attached to the error, or nil if none.
	Metadata() map[string]string

	// To seal the interface
	terminalError()
}

type terminalError struct {
	code     Code
	message  string
	metadata map[string]string
}

var _ TerminalError = (*terminalError)(nil)

func (e *terminalError) Error() string               { return e.message }
func (e *terminalError) Code() Code                  { return e.code }
func (e *terminalError) Message() string             { return e.message }
func (e *terminalError) Metadata() map[string]string { return e.metadata }
func (e *terminalError) terminalError()              {}

// TerminalErrorOption customizes a TerminalError at construction time.
type TerminalErrorOption func(*terminalError)

// WithCode sets the status code of a TerminalError.
func WithCode(code Code) TerminalErrorOption {
	return func(e *terminalError) { e.code = code }
}

// WithMetadata sets the metadata of a TerminalError.
func WithMetadata(metadata map[string]string) TerminalErrorOption {
	return func(e *terminalError) { e.metadata = metadata }
}

// NewTerminalError builds a TerminalError with the given message, defaulting the
// code to DefaultCode unless overridden by an option.
func NewTerminalError(message string, opts ...TerminalErrorOption) TerminalError {
	e := &terminalError{code: DefaultCode, message: message}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// RetryableError wraps an error with a status Code. Unlike a TerminalError it does
// not finish the invocation: it represents a retryable runtime/protocol failure
// (e.g. one surfaced by the state machine) and carries the Code the server should
// respond with. It is not a terminal error and does not satisfy TerminalError.
type RetryableError struct {
	code Code
	err  error
}

// NewRetryableError wraps err with the given status code.
func NewRetryableError(err error, code Code) *RetryableError {
	return &RetryableError{code: code, err: err}
}

func (e *RetryableError) Error() string { return fmt.Sprintf("[%d] %v", e.code, e.err) }
func (e *RetryableError) Code() Code    { return e.code }
func (e *RetryableError) Unwrap() error { return e.err }

// AsRetryableError extracts the RetryableError from err if it is, or wraps, one;
// otherwise it returns nil.
func AsRetryableError(err error) *RetryableError {
	var e *RetryableError
	if errors.As(err, &e) {
		return e
	}
	return nil
}

// IsTerminalError reports whether err is, or wraps, a TerminalError.
func IsTerminalError(err error) bool {
	return AsTerminalError(err) != nil
}

// AsTerminalError extracts the TerminalError from err if it is, or wraps, one;
// otherwise it returns nil.
func AsTerminalError(err error) TerminalError {
	var t TerminalError
	if errors.As(err, &t) {
		return t
	}
	return nil
}

// ToTerminalError converts err into a TerminalError. It returns nil if err is nil;
// if err already is, or wraps, a TerminalError and no options are given it is
// returned unchanged; otherwise a TerminalError is built from err.Error(). err is
// not wrapped - only its message is copied, a TerminalError carries no nested error.
func ToTerminalError(err error, opts ...TerminalErrorOption) TerminalError {
	if err == nil {
		return nil
	}
	if len(opts) == 0 {
		if t := AsTerminalError(err); t != nil {
			return t
		}
	}
	return NewTerminalError(err.Error(), opts...)
}
