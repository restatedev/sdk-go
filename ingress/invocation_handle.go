package ingress

import (
	"context"

	"github.com/restatedev/sdk-go/internal/ingress"
	"github.com/restatedev/sdk-go/internal/options"
)

type InvocationNotFoundError = ingress.InvocationNotFoundError
type InvocationNotReadyError = ingress.InvocationNotReadyError

type InvocationHandle[O any] interface {
	// Attach calls the attach API and blocks until the output is available. Returns an
	// InvocationNotFoundError if the invocation does not exist.
	Attach(ctx context.Context) (O, error)
	// Output calls the attachment API and returns the output if available, otherwise returns an
	// InvocationNotFoundError if the invocation does not exist or an InvocationNotReadyError if
	// the invocation is not complete.
	Output(ctx context.Context) (O, error)
}

// InvocationById returns a handle that lets you Attach or get Output of an invocation by its unique invocation ID.
// The invocation ID is returned when you submit an invocation (e.g., via Send or Submit methods).
//
// Use this when you have an invocation ID and want to attach to the running invocation or retrieve its output.
// The output type O must match the handler's output type.
//
// Example:
//
//	handle := ingress.InvocationById[*MyOutput](client, "inv_1iHLUz0JfQwr0g3903tBTvJLIPSGwxDRjX")
//	output, err := handle.Attach(ctx)
func InvocationById[O any](c *Client, invocationID string, opts ...options.IngressInvocationHandleOption) InvocationHandle[O] {
	handleOpts := options.IngressInvocationHandleOptions{}
	for _, opt := range opts {
		opt.BeforeIngressInvocationHandle(&handleOpts)
	}

	return invocationHandle[O]{
		client: c,
		params: ingress.IngressAttachParams{
			InvocationID: invocationID,
		},
		handleOpts: handleOpts,
	}
}

// ServiceInvocationByIdempotencyKey returns a handle that lets you Attach or get Output of a service invocation
// by its idempotency key. This is useful when you submitted an invocation with an idempotency key and want to
// retrieve its result later.
//
// The output type O must match the handler's output type.
//
// Example:
//
//	handle := ingress.ServiceInvocationByIdempotencyKey[*MyOutput](client, "MyService", "myHandler", "my-idempotency-key")
//	output, err := handle.Attach(ctx)
func ServiceInvocationByIdempotencyKey[O any](c *Client, serviceName, handlerName, idempotencyKey string, opts ...options.IngressInvocationHandleOption) InvocationHandle[O] {
	handleOpts := options.IngressInvocationHandleOptions{}
	for _, opt := range opts {
		opt.BeforeIngressInvocationHandle(&handleOpts)
	}

	return invocationHandle[O]{
		client: c,
		params: ingress.IngressAttachParams{
			ServiceName:    serviceName,
			MethodName:     handlerName,
			IdempotencyKey: idempotencyKey,
		},
		handleOpts: handleOpts,
	}
}

// ObjectInvocationByIdempotencyKey returns a handle that lets you Attach or get Output of a virtual object invocation
// by its idempotency key. This is useful when you submitted an invocation with an idempotency key and want to
// retrieve its result later.
//
// The output type O must match the handler's output type.
//
// Example:
//
//	handle := ingress.ObjectInvocationByIdempotencyKey[*MyOutput](
//	    client, "MyObject", "object-123", "myHandler", "inv_1iHLUz0JfQwr0g3903tBTvJLIPSGwxDRjX")
//	output, err := handle.Attach(ctx)
func ObjectInvocationByIdempotencyKey[O any](c *Client, serviceName, objectKey, handlerName, idempotencyKey string, opts ...options.IngressInvocationHandleOption) InvocationHandle[O] {
	handleOpts := options.IngressInvocationHandleOptions{}
	for _, opt := range opts {
		opt.BeforeIngressInvocationHandle(&handleOpts)
	}

	return invocationHandle[O]{
		client: c,
		params: ingress.IngressAttachParams{
			ServiceName:    serviceName,
			ObjectKey:      objectKey,
			MethodName:     handlerName,
			IdempotencyKey: idempotencyKey,
		},
		handleOpts: handleOpts,
	}
}

// WorkflowHandle returns a handle that lets you Attach or get Output of a workflow invocation.
// This is the primary way to interact with a workflow that has already been started.
//
// The workflowID uniquely identifies the workflow instance. The output type O must match the
// workflow run handler's output type.
//
// Example:
//
//	handle := ingress.WorkflowHandle[*MyWorkflowOutput](client, "MyWorkflow", "workflow-123")
//	output, err := handle.Attach(ctx)
func WorkflowHandle[O any](c *Client, serviceName, workflowID string, opts ...options.IngressInvocationHandleOption) InvocationHandle[O] {
	handleOpts := options.IngressInvocationHandleOptions{}
	for _, opt := range opts {
		opt.BeforeIngressInvocationHandle(&handleOpts)
	}

	return invocationHandle[O]{
		client: c,
		params: ingress.IngressAttachParams{
			ServiceName: serviceName,
			WorkflowID:  workflowID,
		},
		handleOpts: handleOpts,
	}
}

// Deprecated: use InvocationById
func AttachInvocation[O any](c *Client, invocationID string) InvocationHandle[O] {
	return InvocationById[O](c, invocationID)
}

// Deprecated: use ServiceInvocationByIdempotencyKey
func AttachService[O any](c *Client, serviceName, handlerName, idempotencyKey string) InvocationHandle[O] {
	return ServiceInvocationByIdempotencyKey[O](c, serviceName, handlerName, idempotencyKey)
}

// Deprecated: use ObjectInvocationByIdempotencyKey
func AttachObject[O any](c *Client, serviceName, objectKey, handlerName, idempotencyKey string) InvocationHandle[O] {
	return ObjectInvocationByIdempotencyKey[O](c, serviceName, objectKey, handlerName, idempotencyKey)
}

// Deprecated: use WorkflowHandle
func AttachWorkflow[O any](c *Client, serviceName, workflowID string) InvocationHandle[O] {
	return WorkflowHandle[O](c, serviceName, workflowID)
}

type invocationHandle[O any] struct {
	client     *ingress.Client
	params     ingress.IngressAttachParams
	handleOpts options.IngressInvocationHandleOptions
}

func (c invocationHandle[O]) Attach(ctx context.Context) (O, error) {
	var output O
	err := c.client.Attach(ctx, c.params, &output, c.handleOpts)
	if err != nil {
		return output, err
	}
	return output, nil
}

func (c invocationHandle[O]) Output(ctx context.Context) (O, error) {
	var output O
	err := c.client.Output(ctx, c.params, &output, c.handleOpts)
	if err != nil {
		return output, err
	}
	return output, nil
}
