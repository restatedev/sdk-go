// Package errors holds the SDK's error model. TerminalError and its helpers live in
// terminal.go; RetryableError and its helpers in retryable.go; the shared Code and the
// cross-cutting CodeOption live here.
package errors

// Code is a numeric status code for an error, typically a HTTP status code.
type Code uint16

// DefaultCode is the code assigned to an error when none is provided.
const DefaultCode Code = 500

// CodeOption sets the status code. It is shared: it satisfies both TerminalErrorOption
// and RetryableErrorOption, so the same option works for either error type.
type CodeOption struct{ code Code }

func (o CodeOption) applyTerminal(e *terminalError)   { e.code = o.code }
func (o CodeOption) applyRetryable(e *retryableError) { e.code = o.code }

// WithCode sets the status code on a terminal or retryable error.
func WithCode(code Code) CodeOption { return CodeOption{code: code} }
