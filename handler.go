package restate

import (
	"fmt"
	"net/http"

	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/state"
)

// Void is a placeholder to signify 'no value' where a type is otherwise needed. It can be used in several contexts:
//
//  1. Input types for handlers - the request payload codec will reject input at the ingress
//  2. Output types for handlers - the response payload codec will send no bytes and set no content-type
//  3. Input for a outgoing Request or Send - no bytes will be sent
//  4. The output type for an outgoing Request - the response body will be ignored. A pointer is also accepted.
//  5. The output type for an awakeable - the result body will be ignored. A pointer is also accepted.
type Void = encoding.Void

// ServiceHandlerFn is the signature for a Service handler function
type ServiceHandlerFn[I any, O any] func(ctx Context, input I) (O, error)

// ObjectHandlerFn is the signature for a Virtual Object exclusive-mode handler function
type ObjectHandlerFn[I any, O any] func(ctx ObjectContext, input I) (O, error)

// ObjectSharedHandlerFn is the signature for a Virtual Object shared-mode handler function
type ObjectSharedHandlerFn[I any, O any] func(ctx ObjectSharedContext, input I) (O, error)

// ObjectHandlerFn is the signature for a Workflow 'Run' handler function
type WorkflowHandlerFn[I any, O any] func(ctx WorkflowContext, input I) (O, error)

// WorkflowSharedHandlerFn is the signature for a Workflow shared-mode handler function
type WorkflowSharedHandlerFn[I any, O any] func(ctx WorkflowSharedContext, input I) (O, error)

type serviceHandler[I any, O any] struct {
	fn      ServiceHandlerFn[I, O]
	options options.HandlerOptions
}

var _ state.Handler = (*serviceHandler[struct{}, struct{}])(nil)

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

func (h *serviceHandler[I, O]) Call(ctx *state.Context, bytes []byte) ([]byte, error) {
	var input I
	if err := encoding.Unmarshal(h.options.Codec, bytes, &input); err != nil {
		return nil, TerminalError(fmt.Errorf("request could not be decoded into handler input type: %w", err), http.StatusBadRequest)
	}

	output, err := h.fn(
		ctxWrapper{ctx},
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

func (h *serviceHandler[I, O]) GetOptions() *options.HandlerOptions {
	return &h.options
}

type objectHandler[I any, O any] struct {
	// only one of exclusiveFn or sharedFn should be set, as indicated by handlerType
	exclusiveFn ObjectHandlerFn[I, O]
	sharedFn    ObjectSharedHandlerFn[I, O]
	options     options.HandlerOptions
	handlerType internal.ServiceHandlerType
}

var _ state.Handler = (*objectHandler[struct{}, struct{}])(nil)

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

type ctxWrapper struct {
	*state.Context
}

func (o ctxWrapper) inner() *state.Context {
	return o.Context
}
func (o ctxWrapper) object()          {}
func (o ctxWrapper) exclusiveObject() {}
func (o ctxWrapper) workflow()        {}
func (o ctxWrapper) runWorkflow()     {}

func (h *objectHandler[I, O]) Call(ctx *state.Context, bytes []byte) ([]byte, error) {
	var input I
	if err := encoding.Unmarshal(h.options.Codec, bytes, &input); err != nil {
		return nil, TerminalError(fmt.Errorf("request could not be decoded into handler input type: %w", err), http.StatusBadRequest)
	}

	var output O
	var err error
	switch h.handlerType {
	case internal.ServiceHandlerType_EXCLUSIVE:
		output, err = h.exclusiveFn(
			ctxWrapper{ctx},
			input,
		)
	case internal.ServiceHandlerType_SHARED:
		output, err = h.sharedFn(
			ctxWrapper{ctx},
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

func (h *objectHandler[I, O]) GetOptions() *options.HandlerOptions {
	return &h.options
}

func (h *objectHandler[I, O]) HandlerType() *internal.ServiceHandlerType {
	return &h.handlerType
}

type workflowHandler[I any, O any] struct {
	// only one of workflowFn or sharedFn should be set, as indicated by handlerType
	workflowFn  WorkflowHandlerFn[I, O]
	sharedFn    WorkflowSharedHandlerFn[I, O]
	options     options.HandlerOptions
	handlerType internal.ServiceHandlerType
}

var _ state.Handler = (*workflowHandler[struct{}, struct{}])(nil)

// NewWorkflowHandler converts a function of signature [WorkflowHandlerFn] into the 'Run' handler on a Workflow.
// The handler will have access to a full [WorkflowContext] which may mutate state.
func NewWorkflowHandler[I any, O any](fn WorkflowHandlerFn[I, O], opts ...options.HandlerOption) *workflowHandler[I, O] {
	o := options.HandlerOptions{}
	for _, opt := range opts {
		opt.BeforeHandler(&o)
	}
	return &workflowHandler[I, O]{
		workflowFn:  fn,
		options:     o,
		handlerType: internal.ServiceHandlerType_WORKFLOW,
	}
}

// NewWorkflowSharedHandler converts a function of signature [ObjectSharedHandlerFn] into a shared-mode handler on a Workflow.
// The handler will only have access to a [WorkflowSharedContext] which can only read a snapshot of state.
func NewWorkflowSharedHandler[I any, O any](fn WorkflowSharedHandlerFn[I, O], opts ...options.HandlerOption) *workflowHandler[I, O] {
	o := options.HandlerOptions{}
	for _, opt := range opts {
		opt.BeforeHandler(&o)
	}
	return &workflowHandler[I, O]{
		sharedFn:    fn,
		options:     o,
		handlerType: internal.ServiceHandlerType_SHARED,
	}
}

func (h *workflowHandler[I, O]) Call(ctx *state.Context, bytes []byte) ([]byte, error) {
	var input I
	if err := encoding.Unmarshal(h.options.Codec, bytes, &input); err != nil {
		return nil, TerminalError(fmt.Errorf("request could not be decoded into handler input type: %w", err), http.StatusBadRequest)
	}

	var output O
	var err error
	switch h.handlerType {
	case internal.ServiceHandlerType_WORKFLOW:
		output, err = h.workflowFn(
			ctxWrapper{ctx},
			input,
		)
	case internal.ServiceHandlerType_SHARED:
		output, err = h.sharedFn(
			ctxWrapper{ctx},
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

func (h *workflowHandler[I, O]) InputPayload() *encoding.InputPayload {
	var i I
	return encoding.InputPayloadFor(h.options.Codec, i)
}

func (h *workflowHandler[I, O]) OutputPayload() *encoding.OutputPayload {
	var o O
	return encoding.OutputPayloadFor(h.options.Codec, o)
}

func (h *workflowHandler[I, O]) GetOptions() *options.HandlerOptions {
	return &h.options
}

func (h *workflowHandler[I, O]) HandlerType() *internal.ServiceHandlerType {
	return &h.handlerType
}
