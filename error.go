package restate

import (
	"errors"
	"fmt"
)

type Code uint16

type codeError struct {
	code  Code
	inner error
}

func (e *codeError) Error() string {
	return fmt.Sprintf("[CODE %04X] %s", e.code, e.inner)
}

func (e *codeError) Unwrap() error {
	return e.inner
}

type terminalError struct {
	inner error
}

func (e *terminalError) Error() string {
	return e.inner.Error()
}

func (e *terminalError) Unwrap() error {
	return e.inner
}

// WithErrorCode returns an error with specific
func WithErrorCode(err error, code Code) error {
	if err == nil {
		return nil
	}

	return &codeError{
		inner: err,
		code:  code,
	}
}

// TerminalError returns a terminal error with optional code.
// code is optional but only one code is allowed.
// By default restate will retry the invocation forever unless a terminal error
// is returned
func TerminalError(err error, code ...Code) error {
	if err == nil {
		return nil
	}

	if len(code) > 1 {
		panic("only single code is allowed")
	}

	err = &terminalError{
		inner: err,
	}

	if len(code) == 1 {
		err = &codeError{
			inner: err,
			code:  code[0],
		}
	}

	return err
}

// IsTerminalError checks if err is terminal
func IsTerminalError(err error) bool {
	if err == nil {
		return false
	}
	var t *terminalError
	return errors.As(err, &t)
}

// ErrorCode returns code associated with error or UNKNOWN
func ErrorCode(err error) Code {
	var e *codeError
	if errors.As(err, &e) {
		return e.code
	}

	return 500
}
