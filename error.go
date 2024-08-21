package restate

import (
	"fmt"

	"github.com/restatedev/sdk-go/internal/errors"
)

var (
	// ErrKeyNotFound is returned when there is no state value for a key
	ErrKeyNotFound = errors.ErrKeyNotFound
)

// Code is a numeric status code for an error, typically a HTTP status code.
type Code = errors.Code

// WithErrorCode returns an error with specific [Code] attached.
func WithErrorCode(err error, code Code) error {
	if err == nil {
		return nil
	}

	return &errors.CodeError{
		Inner: err,
		Code:  code,
	}
}

// TerminalError returns a terminal error with optional code. Code is optional but only one code is allowed.
// By default, restate will retry the invocation or Run function forever unless a terminal error is returned
func TerminalError(err error, code ...errors.Code) error {
	return errors.NewTerminalError(err, code...)
}

// TerminalErrorf is a shorthand for combining fmt.Errorf with TerminalError
func TerminalErrorf(format string, a ...any) error {
	return TerminalError(fmt.Errorf(format, a...))
}

// IsTerminalError checks if err is terminal - ie, that returning it in a handler or Run function will finish
// the invocation with the error as a result.
func IsTerminalError(err error) bool {
	return errors.IsTerminalError(err)
}

// ErrorCode returns [Code] associated with error, defaulting to 500
func ErrorCode(err error) errors.Code {
	return errors.ErrorCode(err)
}
