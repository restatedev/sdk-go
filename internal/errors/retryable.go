package errors

import (
	"errors"
	"fmt"
)

// RetryableError finishes an attempt with a non-terminal failure: the invocation (or a
// Run function) is retried rather than completed. It carries a status code and a
// message, wraps the underlying error, and implements the error interface.
type RetryableError interface {
	error
	// Code returns the status code attached to the error.
	Code() Code
	// Message returns the error message.
	Message() string

	// To seal the interface
	retryableError()
}

type retryableError struct {
	code Code
	err  error
}

var _ RetryableError = (*retryableError)(nil)

func (e *retryableError) Error() string   { return fmt.Sprintf("[%d] %v", e.code, e.err) }
func (e *retryableError) Code() Code      { return e.code }
func (e *retryableError) Message() string { return e.err.Error() }
func (e *retryableError) Unwrap() error   { return e.err }
func (e *retryableError) retryableError() {}

// RetryableErrorOption customizes a RetryableError at construction time.
type RetryableErrorOption interface{ applyRetryable(*retryableError) }

// NewRetryableError wraps err as a RetryableError, defaulting the code to DefaultCode
// unless overridden by an option.
func NewRetryableError(err error, opts ...RetryableErrorOption) RetryableError {
	e := &retryableError{code: DefaultCode, err: err}
	for _, opt := range opts {
		opt.applyRetryable(e)
	}
	return e
}

// IsRetryableError reports whether err is, or wraps, a RetryableError.
func IsRetryableError(err error) bool {
	return AsRetryableError(err) != nil
}

// AsRetryableError extracts the RetryableError from err if it is, or wraps, one;
// otherwise it returns nil.
func AsRetryableError(err error) RetryableError {
	var r RetryableError
	if errors.As(err, &r) {
		return r
	}
	return nil
}

// ToRetryableError converts err into a RetryableError. It returns nil if err is nil;
// if err already is, or wraps, a RetryableError and no options are given it is
// returned unchanged; otherwise err is wrapped in a new RetryableError.
func ToRetryableError(err error, opts ...RetryableErrorOption) RetryableError {
	if err == nil {
		return nil
	}
	if len(opts) == 0 {
		if r := AsRetryableError(err); r != nil {
			return r
		}
	}
	return NewRetryableError(err, opts...)
}
