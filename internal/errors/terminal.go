package errors

import (
	"errors"

	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/stringmap"
)

// TerminalError finishes an invocation (or a Run function) with a failure result
// instead of being retried. It carries a status code, a message and optional
// metadata, and implements the error interface.
type TerminalError interface {
	error
	// Code returns the status code attached to the error.
	Code() Code
	// Message returns the error message.
	Message() string
	// Metadata returns the metadata attached to the error as a read-only view.
	Metadata() stringmap.Map

	// To seal the interface
	terminalError()
}

type terminalError struct {
	code     Code
	message  string
	metadata map[string]string
}

var _ TerminalError = (*terminalError)(nil)

func (e *terminalError) Error() string           { return e.message }
func (e *terminalError) Code() Code              { return e.code }
func (e *terminalError) Message() string         { return e.message }
func (e *terminalError) Metadata() stringmap.Map { return stringmap.New(e.metadata) }
func (e *terminalError) terminalError()          {}

// TerminalErrorOption customizes a TerminalError at construction time.
type TerminalErrorOption interface{ applyTerminal(*terminalError) }

// MetadataOption sets metadata. It is shared across everything that accepts metadata:
// terminal errors, service definitions and handlers (and merges with any metadata
// already set). It deliberately lives here, alongside the private terminalError it must
// reach, while also satisfying the options package's service/handler interfaces.
type MetadataOption struct{ metadata map[string]string }

var (
	_ options.ServiceDefinitionOption = MetadataOption{}
	_ options.HandlerOption           = MetadataOption{}
)

func (o MetadataOption) applyTerminal(e *terminalError) {
	e.metadata = mergeMetadata(e.metadata, o.metadata)
}

func (o MetadataOption) BeforeServiceDefinition(opts *options.ServiceDefinitionOptions) {
	opts.Metadata = mergeMetadata(opts.Metadata, o.metadata)
}

func (o MetadataOption) BeforeHandler(opts *options.HandlerOptions) {
	opts.Metadata = mergeMetadata(opts.Metadata, o.metadata)
}

// WithMetadata returns an option adding the given metadata.
func WithMetadata(m map[string]string) MetadataOption { return MetadataOption{metadata: m} }

// mergeMetadata adds src into dst, allocating dst if needed, and returns it.
func mergeMetadata(dst, src map[string]string) map[string]string {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]string, len(src))
	}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// NewTerminalError builds a TerminalError with the given message, defaulting the
// code to DefaultCode unless overridden by an option.
func NewTerminalError(message string, opts ...TerminalErrorOption) TerminalError {
	e := &terminalError{code: DefaultCode, message: message}
	for _, opt := range opts {
		opt.applyTerminal(e)
	}
	return e
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
