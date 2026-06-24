package errors

import (
	"errors"
	"fmt"
)

type Code uint16

type CodeError struct {
	Code     Code
	Inner    error
	Metadata map[string]string
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

type MetadataError struct {
	Inner    error
	Metadata map[string]string
}

func (e *MetadataError) Error() string {
	return e.Inner.Error()
}

func (e *MetadataError) Unwrap() error {
	return e.Inner
}

func NewMetadataError(err error, metadata map[string]string) error {
	if err == nil {
		return nil
	}
	if len(metadata) == 0 {
		return err
	}
	return &MetadataError{
		Inner:    err,
		Metadata: metadata,
	}
}

func ErrorMetadata(err error) map[string]string {
	var codeErr *CodeError
	if errors.As(err, &codeErr) && len(codeErr.Metadata) > 0 {
		return codeErr.Metadata
	}

	var metadataErr *MetadataError
	if errors.As(err, &metadataErr) && len(metadataErr.Metadata) > 0 {
		return metadataErr.Metadata
	}

	return nil
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
