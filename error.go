package restate

import (
	stderrors "errors"

	"github.com/restatedev/sdk-go/internal/errors"
)

// WithErrorCode returns an error with specific
func WithErrorCode(err error, code errors.Code) error {
	if err == nil {
		return nil
	}

	return &errors.CodeError{
		Inner: err,
		Code:  code,
	}
}

// TerminalError returns a terminal error with optional code.
// code is optional but only one code is allowed.
// By default restate will retry the invocation forever unless a terminal error
// is returned
func TerminalError(err error, code ...errors.Code) error {
	return errors.NewTerminalError(err, code...)
}

// IsTerminalError checks if err is terminal
func IsTerminalError(err error) bool {
	if err == nil {
		return false
	}
	var t *errors.TerminalError
	return stderrors.As(err, &t)
}

// ErrorCode returns code associated with error or UNKNOWN
func ErrorCode(err error) errors.Code {
	var e *errors.CodeError
	if stderrors.As(err, &e) {
		return e.Code
	}

	return 500
}
