package errors

import (
	"errors"
	"fmt"
)

type Code uint16

type CodeError struct {
	Code  Code
	Inner error
}

func (e *CodeError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Inner)
}

func (e *CodeError) Unwrap() error {
	return e.Inner
}

func ErrorCode(err error) Code {
	var e *CodeError
	if errors.As(err, &e) {
		return e.Code
	}

	return 500
}

type TerminalError struct {
	Inner error
}

func (e *TerminalError) Error() string {
	return e.Inner.Error()
}

func (e *TerminalError) Unwrap() error {
	return e.Inner
}

func IsTerminalError(err error) bool {
	if err == nil {
		return false
	}
	var t *TerminalError
	return errors.As(err, &t)
}

func NewTerminalError(err error, code ...Code) error {
	if err == nil {
		return nil
	}

	if len(code) > 1 {
		panic("only single code is allowed")
	}

	err = &TerminalError{
		Inner: err,
	}

	if len(code) == 1 {
		err = &CodeError{
			Inner: err,
			Code:  code[0],
		}
	}

	return err
}
