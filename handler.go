package restate

import (
	"fmt"
	"net/http"

	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal"
	"github.com/restatedev/sdk-go/internal/options"
)

// Void is a placeholder to signify 'no value' where a type is otherwise needed. It can be used in several contexts:
//
//  1. Input types for handlers - the request payload codec will reject input at the ingress
//  2. Output types for handlers - the response payload codec will send no bytes and set no content-type
//  3. Input for a outgoing Request or Send - no bytes will be sent
//  4. The output type for an outgoing Request - the response body will be ignored. A pointer is also accepted.
//  5. The output type for an awakeable - the result body will be ignored. A pointer is also accepted.
type Void = encoding.Void

// ObjectHandler is the required set of methods for a Virtual Object handler.
type ObjectHandler interface {
	Call(ctx ObjectContext, request []byte) (output []byte, err error)
	Handler
}

// ServiceHandler is the required set of methods for a Service handler.
type ServiceHandler interface {
	Call(ctx Context, request []byte) (output []byte, err error)
	Handler
}

// Handler is implemented by all Restate handlers
type Handler interface {
	getOptions() *options.HandlerOptions
	InputPayload() *encoding.InputPayload
	OutputPayload() *encoding.OutputPayload
	HandlerType() *internal.ServiceHandlerType
}

// ServiceHandlerFn is the signature for a Service handler function
type ServiceHandlerFn[I any, O any] func(ctx Context, input I) (O, error)

// ObjectHandlerFn is the signature for a Virtual Object exclusive-mode handler function
type ObjectHandlerFn[I any, O any] func(ctx ObjectContext, input I) (O, error)

// ObjectHandlerFn is the signature for a Virtual Object shared-mode handler function
type ObjectSharedHandlerFn[I any, O any] func(ctx ObjectSharedContext, input I) (O, error)

type serviceHandler[I any, O any] struct {
	fn      ServiceHandlerFn[I, O]
	options options.HandlerOptions
}

var _ ServiceHandler = (*serviceHandler[struct{}, struct{}])(nil)

// NewServiceHandler converts a function of signature [ServiceHandlerFn] into a handler on a Restate service.
func NewServiceHandler[I any, O any](fn ServiceHandlerFn[I, O], opts ...options.HandlerOption) *serviceHandler[I, O] {
	o := options.HandlerOptions{}
	for _, opt := range opts {
		opt.BeforeHandler(&o)
	}
	return &serviceHandler[I, O]{
		fn:      fn,
		options: o,
	}
}

func (h *serviceHandler[I, O]) Call(ctx Context, bytes []byte) ([]byte, error) {
	var input I
	if err := encoding.Unmarshal(h.options.Codec, bytes, &input); err != nil {
		return nil, TerminalError(fmt.Errorf("request could not be decoded into handler input type: %w", err), http.StatusBadRequest)
	}

	output, err := h.fn(
		ctx,
		input,
	)
	if err != nil {
		return nil, err
	}

	bytes, err = encoding.Marshal(h.options.Codec, output)
	if err != nil {
		// we don't use a terminal error here as this is hot-fixable by changing the return type
		return nil, fmt.Errorf("failed to serialize output: %w", err)
	}

	return bytes, nil
}

func (h *serviceHandler[I, O]) InputPayload() *encoding.InputPayload {
	var i I
	return encoding.InputPayloadFor(h.options.Codec, i)
}

func (h *serviceHandler[I, O]) OutputPayload() *encoding.OutputPayload {
	var o O
	return encoding.OutputPayloadFor(h.options.Codec, o)
}

func (h *serviceHandler[I, O]) HandlerType() *internal.ServiceHandlerType {
	return nil
}

func (h *serviceHandler[I, O]) getOptions() *options.HandlerOptions {
	return &h.options
}

type objectHandler[I any, O any] struct {
	// only one of exclusiveFn or sharedFn should be set, as indicated by handlerType
	exclusiveFn ObjectHandlerFn[I, O]
	sharedFn    ObjectSharedHandlerFn[I, O]
	options     options.HandlerOptions
	handlerType internal.ServiceHandlerType
}

var _ ObjectHandler = (*objectHandler[struct{}, struct{}])(nil)

// NewObjectHandler converts a function of signature [ObjectHandlerFn] into an exclusive-mode handler on a Virtual Object.
// The handler will have access to a full [ObjectContext] which may mutate state.
func NewObjectHandler[I any, O any](fn ObjectHandlerFn[I, O], opts ...options.HandlerOption) *objectHandler[I, O] {
	o := options.HandlerOptions{}
	for _, opt := range opts {
		opt.BeforeHandler(&o)
	}
	return &objectHandler[I, O]{
		exclusiveFn: fn,
		options:     o,
		handlerType: internal.ServiceHandlerType_EXCLUSIVE,
	}
}

// NewObjectSharedHandler converts a function of signature [ObjectSharedHandlerFn] into a shared-mode handler on a Virtual Object.
// The handler will only have access to a [ObjectSharedContext] which can only read a snapshot of state.
func NewObjectSharedHandler[I any, O any](fn ObjectSharedHandlerFn[I, O], opts ...options.HandlerOption) *objectHandler[I, O] {
	o := options.HandlerOptions{}
	for _, opt := range opts {
		opt.BeforeHandler(&o)
	}
	return &objectHandler[I, O]{
		sharedFn:    fn,
		options:     o,
		handlerType: internal.ServiceHandlerType_SHARED,
	}
}

func (h *objectHandler[I, O]) Call(ctx ObjectContext, bytes []byte) ([]byte, error) {
	var input I
	if err := encoding.Unmarshal(h.options.Codec, bytes, &input); err != nil {
		return nil, TerminalError(fmt.Errorf("request could not be decoded into handler input type: %w", err), http.StatusBadRequest)
	}

	var output O
	var err error
	switch h.handlerType {
	case internal.ServiceHandlerType_EXCLUSIVE:
		output, err = h.exclusiveFn(
			ctx,
			input,
		)
	case internal.ServiceHandlerType_SHARED:
		output, err = h.sharedFn(
			ctx,
			input,
		)
	}
	if err != nil {
		return nil, err
	}

	bytes, err = encoding.Marshal(h.options.Codec, output)
	if err != nil {
		// we don't use a terminal error here as this is hot-fixable by changing the return type
		return nil, fmt.Errorf("failed to serialize output: %w", err)
	}

	return bytes, nil
}

func (h *objectHandler[I, O]) InputPayload() *encoding.InputPayload {
	var i I
	return encoding.InputPayloadFor(h.options.Codec, i)
}

func (h *objectHandler[I, O]) OutputPayload() *encoding.OutputPayload {
	var o O
	return encoding.OutputPayloadFor(h.options.Codec, o)
}

func (h *objectHandler[I, O]) getOptions() *options.HandlerOptions {
	return &h.options
}

func (h *objectHandler[I, O]) HandlerType() *internal.ServiceHandlerType {
	return &h.handlerType
}
