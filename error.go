package restate

import (
	"fmt"

	"github.com/restatedev/sdk-go/internal/errors"
)

// Code is a numeric status code for an error, matching HTTP status code semantics.
type Code = errors.Code

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

// WithErrorCode sets the [Code] of a [TerminalError]. Pass it to [ToTerminalError].
func WithErrorCode(code Code) TerminalErrorOption {
	return errors.WithCode(code)
}

// WithErrorMetadata sets the metadata of a [TerminalError]. Pass it to [ToTerminalError].
func WithErrorMetadata(metadata map[string]string) TerminalErrorOption {
	return errors.WithMetadata(metadata)
}

// ToTerminalError converts err into a [TerminalError], so that returning it from a
// handler or Run finishes the invocation with a failure result instead of being
// retried.
//
// IMPORTANT: this does NOT wrap err. A [TerminalError] carries no nested error and is
// not part of err's chain: errors.Unwrap, errors.Is and errors.As will not reach err
// through the result. Only the message err.Error() is copied.
//
// It returns nil if err is nil; if err already is, or wraps, a [TerminalError] and no
// options are given, that [TerminalError] is returned unchanged. The code defaults to
// 500 unless set with [WithErrorCode]; metadata can be attached with [WithErrorMetadata].
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
