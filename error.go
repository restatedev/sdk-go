package restate

import (
	"fmt"

	"github.com/restatedev/sdk-go/internal/errors"
)

// Code is a numeric status code for an error, matching HTTP status code semantics.
type Code = errors.Code

// ErrorCodeOption sets the [Code] on an error. It is shared: pass it to either
// [ToTerminalError] or [ToRetryableError].
type ErrorCodeOption = errors.CodeOption

// WithErrorCode sets the [Code] of a terminal or retryable error. Pass it to
// [ToTerminalError] or [ToRetryableError].
func WithErrorCode(code Code) ErrorCodeOption {
	return errors.WithCode(code)
}

// TerminalError finishes an invocation (or a Run function) with a failure result
// instead of being retried. By default, Restate retries the invocation or Run
// function forever unless a terminal error is returned.
//
// It carries a status code, a message and optional metadata, accessible via the
// Code, Message and Metadata methods, and implements the error interface. Use
// [TerminalErrorf] or [ToTerminalError] to construct one.
type TerminalError = errors.TerminalError

// TerminalErrorOption customizes a [TerminalError]. Pass it to [ToTerminalError].
type TerminalErrorOption = errors.TerminalErrorOption

// ToTerminalError converts err into a [TerminalError], so that returning it from a
// handler or Run finishes the invocation with a failure result instead of being
// retried.
//
// IMPORTANT: this does NOT wrap err. A [TerminalError] carries no nested error and is
// not part of err's chain: errors.Unwrap, errors.Is and errors.As will not reach err
// through the result. Only the message, err.Error(), is copied.
//
// It returns nil if err is nil; if err already is, or wraps, a [TerminalError] and no
// options are given, that [TerminalError] is returned unchanged. The code defaults to
// 500 unless set with [WithErrorCode]; metadata can be attached with [WithMetadata].
func ToTerminalError(err error, opts ...TerminalErrorOption) TerminalError {
	return errors.ToTerminalError(err, opts...)
}

// TerminalErrorf builds a [TerminalError] whose message is fmt.Sprintf(format, a...).
// To attach a code or metadata, build the message with fmt.Errorf and pass it to
// [ToTerminalError] with the relevant options.
func TerminalErrorf(format string, a ...any) TerminalError {
	return errors.NewTerminalError(fmt.Sprintf(format, a...))
}

// IsTerminalError reports whether err is, or wraps, a [TerminalError] - ie, that
// returning it in a handler or Run function will finish the invocation with the
// error as a result.
func IsTerminalError(err error) bool {
	return errors.IsTerminalError(err)
}

// AsTerminalError casts the current error to [TerminalError] if any.
func AsTerminalError(err error) TerminalError {
	return errors.AsTerminalError(err)
}

// RetryableError finishes an attempt with a non-terminal failure: the invocation (or a
// Run closure) is retried rather than completed. It carries a [Code] and a message,
// wraps the underlying error, and implements the error interface. Returning one from a
// handler or Run closure is equivalent to returning any non-terminal error - Restate
// retries - except that its code is carried through. Use [RetryableErrorf] or
// [ToRetryableError] to construct one.
type RetryableError = errors.RetryableError

// RetryableErrorOption customizes a [RetryableError]. Pass it to [ToRetryableError].
type RetryableErrorOption = errors.RetryableErrorOption

// ToRetryableError converts err into a [RetryableError]. It returns nil if err is nil;
// if err already is, or wraps, a [RetryableError] and no options are given, that
// [RetryableError] is returned unchanged; otherwise err is wrapped (errors.Unwrap,
// errors.Is and errors.As reach err through the result). The code defaults to 500 unless
// set with [WithErrorCode].
func ToRetryableError(err error, opts ...RetryableErrorOption) RetryableError {
	return errors.ToRetryableError(err, opts...)
}

// RetryableErrorf builds a [RetryableError] whose message is fmt.Sprintf(format, a...).
// To attach a code, build the message with fmt.Errorf and pass it to [ToRetryableError]
// with [WithErrorCode].
func RetryableErrorf(format string, a ...any) RetryableError {
	return errors.NewRetryableError(fmt.Errorf(format, a...))
}

// IsRetryableError reports whether err is, or wraps, a [RetryableError].
func IsRetryableError(err error) bool {
	return errors.IsRetryableError(err)
}

// AsRetryableError casts the current error to [RetryableError] if any.
func AsRetryableError(err error) RetryableError {
	return errors.AsRetryableError(err)
}
