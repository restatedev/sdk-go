package restate

import (
	"fmt"
	"net/http"

	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal"
)

// Void is a placeholder to signify 'no value' where a type is otherwise needed. It can be used in several contexts:
// 1. Input types for handlers - the request payload codec will default to a encoding.VoidCodec which will reject input at the ingress
// 2. Output types for handlers - the response payload codec will default to a encoding.VoidCodec which will send no bytes and set no content-type
type Void = encoding.Void

type ObjectHandler interface {
	Call(ctx ObjectContext, request []byte) (output []byte, err error)
	getOptions() *objectHandlerOptions
	Handler
}

type ServiceHandler interface {
	Call(ctx Context, request []byte) (output []byte, err error)
	getOptions() *serviceHandlerOptions
	Handler
}

type Handler interface {
	sealed()
	InputPayload() *encoding.InputPayload
	OutputPayload() *encoding.OutputPayload
	HandlerType() *internal.ServiceHandlerType
}

// ServiceHandlerFn signature of service (unkeyed) handler function
type ServiceHandlerFn[I any, O any] func(ctx Context, input I) (O, error)

// ObjectHandlerFn signature for object (keyed) handler function
type ObjectHandlerFn[I any, O any] func(ctx ObjectContext, input I) (O, error)

// ObjectHandlerFn signature for object (keyed) handler function that can run concurrently with other handlers against a snapshot of state
type ObjectSharedHandlerFn[I any, O any] func(ctx ObjectSharedContext, input I) (O, error)

type serviceHandlerOptions struct {
	codec encoding.PayloadCodec
}

type serviceHandler[I any, O any] struct {
	fn      ServiceHandlerFn[I, O]
	options serviceHandlerOptions
}

var _ ServiceHandler = (*serviceHandler[struct{}, struct{}])(nil)

type ServiceHandlerOption interface {
	beforeServiceHandler(*serviceHandlerOptions)
}

// NewServiceHandler create a new handler for a service, defaulting to JSON encoding
func NewServiceHandler[I any, O any](fn ServiceHandlerFn[I, O], options ...ServiceHandlerOption) *serviceHandler[I, O] {
	opts := serviceHandlerOptions{}
	for _, opt := range options {
		opt.beforeServiceHandler(&opts)
	}
	if opts.codec == nil {
		opts.codec = encoding.PartialVoidCodec[I, O]()
	}
	return &serviceHandler[I, O]{
		fn:      fn,
		options: opts,
	}
}

func (h *serviceHandler[I, O]) Call(ctx Context, bytes []byte) ([]byte, error) {
	var input I
	if err := h.options.codec.Unmarshal(bytes, &input); err != nil {
		return nil, TerminalError(fmt.Errorf("request could not be decoded into handler input type: %w", err), http.StatusBadRequest)
	}

	output, err := h.fn(
		ctx,
		input,
	)
	if err != nil {
		return nil, err
	}

	bytes, err = h.options.codec.Marshal(output)
	if err != nil {
		return nil, TerminalError(fmt.Errorf("failed to serialize output: %w", err))
	}

	return bytes, nil
}

func (h *serviceHandler[I, O]) InputPayload() *encoding.InputPayload {
	return h.options.codec.InputPayload()
}

func (h *serviceHandler[I, O]) OutputPayload() *encoding.OutputPayload {
	return h.options.codec.OutputPayload()
}

func (h *serviceHandler[I, O]) HandlerType() *internal.ServiceHandlerType {
	return nil
}

func (h *serviceHandler[I, O]) getOptions() *serviceHandlerOptions {
	return &h.options
}

func (h *serviceHandler[I, O]) sealed() {}

type objectHandlerOptions struct {
	codec encoding.PayloadCodec
}

type objectHandler[I any, O any] struct {
	// only one of exclusiveFn or sharedFn should be set, as indicated by handlerType
	exclusiveFn ObjectHandlerFn[I, O]
	sharedFn    ObjectSharedHandlerFn[I, O]
	options     objectHandlerOptions
	handlerType internal.ServiceHandlerType
}

var _ ObjectHandler = (*objectHandler[struct{}, struct{}])(nil)

type ObjectHandlerOption interface {
	beforeObjectHandler(*objectHandlerOptions)
}

func NewObjectHandler[I any, O any](fn ObjectHandlerFn[I, O], options ...ObjectHandlerOption) *objectHandler[I, O] {
	opts := objectHandlerOptions{}
	for _, opt := range options {
		opt.beforeObjectHandler(&opts)
	}
	if opts.codec == nil {
		opts.codec = encoding.PartialVoidCodec[I, O]()
	}
	return &objectHandler[I, O]{
		exclusiveFn: fn,
		options:     opts,
		handlerType: internal.ServiceHandlerType_EXCLUSIVE,
	}
}

func NewObjectSharedHandler[I any, O any](fn ObjectSharedHandlerFn[I, O], options ...ObjectHandlerOption) *objectHandler[I, O] {
	opts := objectHandlerOptions{}
	for _, opt := range options {
		opt.beforeObjectHandler(&opts)
	}
	if opts.codec == nil {
		opts.codec = encoding.PartialVoidCodec[I, O]()
	}
	return &objectHandler[I, O]{
		sharedFn:    fn,
		options:     opts,
		handlerType: internal.ServiceHandlerType_SHARED,
	}
}

func (h *objectHandler[I, O]) Call(ctx ObjectContext, bytes []byte) ([]byte, error) {
	var input I
	if err := h.options.codec.Unmarshal(bytes, &input); err != nil {
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

	bytes, err = h.options.codec.Marshal(output)
	if err != nil {
		return nil, TerminalError(fmt.Errorf("failed to serialize output: %w", err))
	}

	return bytes, nil
}

func (h *objectHandler[I, O]) InputPayload() *encoding.InputPayload {
	return h.options.codec.InputPayload()
}

func (h *objectHandler[I, O]) OutputPayload() *encoding.OutputPayload {
	return h.options.codec.OutputPayload()
}

func (h *objectHandler[I, O]) getOptions() *objectHandlerOptions {
	return &h.options
}

func (h *objectHandler[I, O]) HandlerType() *internal.ServiceHandlerType {
	return &h.handlerType
}

func (h *objectHandler[I, O]) sealed() {}
