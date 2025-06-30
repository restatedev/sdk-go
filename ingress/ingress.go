package ingress

import (
	"context"

	"github.com/restatedev/sdk-go/internal/ingress"
	"github.com/restatedev/sdk-go/internal/options"
)

type Client = ingress.Client
type Invocation = ingress.Invocation
type InvocationNotFoundError = ingress.InvocationNotFoundError
type InvocationNotReadyError = ingress.InvocationNotReadyError

type Requester[I any, O any] interface {
	Request(ctx context.Context, input I, options ...options.IngressRequestOption) (O, error)
	SendRequester[I]
}

type SendRequester[I any] interface {
	Send(ctx context.Context, input I, options ...options.IngressSendOption) Invocation
}

type InvocationHandle[O any] interface {
	Attach(ctx context.Context) (O, error)
	Output(ctx context.Context) (O, error)
}

type client[I any, O any] struct {
	client *Client
	params ingress.IngressParams
}

type attachClient[O any] struct {
	client *ingress.Client
	params ingress.IngressAttachParams
}

func NewClient(baseUri string, opts ...options.IngressClientOption) *Client {
	clientOpts := options.IngressClientOptions{}
	for _, opt := range opts {
		opt.BeforeIngress(&clientOpts)
	}
	return ingress.NewClient(baseUri, clientOpts)
}

// Service gets a Service request ingress client by service and handlerName name
func Service[I any, O any](c *Client, serviceName, handlerName string) Requester[I, O] {
	return client[I, O]{
		client: c,
		params: ingress.IngressParams{
			ServiceName: serviceName,
			HandlerName: handlerName,
		},
	}
}

// ServiceSend gets a Service send ingress client by service and handlerName name
func ServiceSend[I any](c *Client, serviceName, handlerName string) SendRequester[I] {
	return client[I, any]{
		client: c,
		params: ingress.IngressParams{
			ServiceName: serviceName,
			HandlerName: handlerName,
		},
	}
}

// Object gets an Object request ingress client by service name, key and handlerName name
func Object[I any, O any](c *Client, serviceName, objectKey, handlerName string) Requester[I, O] {
	return client[I, O]{
		client: c,
		params: ingress.IngressParams{
			ServiceName: serviceName,
			ObjectKey:   objectKey,
			HandlerName: handlerName,
		},
	}
}

// ObjectSend gets an Object send ingress client by service name, key and handlerName name
func ObjectSend[I any](c *Client, serviceName, objectKey, handlerName string) SendRequester[I] {
	return client[I, any]{
		client: c,
		params: ingress.IngressParams{
			ServiceName: serviceName,
			ObjectKey:   objectKey,
			HandlerName: handlerName,
		},
	}
}

// Workflow gets a Workflow request ingress client by service name, workflow ID and handlerName name
func Workflow[I any, O any](c *Client, serviceName, workflowID, handlerName string) Requester[I, O] {
	return client[I, O]{
		client: c,
		params: ingress.IngressParams{
			ServiceName: serviceName,
			HandlerName: handlerName,
			WorkflowID:  workflowID,
		},
	}
}

// WorkflowSend gets a Workflow send ingress client by service name, workflow ID and handlerName name
func WorkflowSend[I any](c *Client, serviceName, workflowID, handlerName string) SendRequester[I] {
	return client[I, any]{
		client: c,
		params: ingress.IngressParams{
			ServiceName: serviceName,
			HandlerName: handlerName,
			WorkflowID:  workflowID,
		},
	}
}

// AttachInvocation gets an attachment client based on an invocation ID.
func AttachInvocation[O any](c *Client, invocationID string) InvocationHandle[O] {
	return attachClient[O]{
		client: c,
		params: ingress.IngressAttachParams{
			InvocationID: invocationID,
		},
	}
}

// AttachService gets an attachment client based on a service handler and idempotency key.
func AttachService[O any](c *Client, serviceName, handlerName, idempotencyKey string) InvocationHandle[O] {
	return attachClient[O]{
		client: c,
		params: ingress.IngressAttachParams{
			ServiceName:    serviceName,
			MethodName:     handlerName,
			IdempotencyKey: idempotencyKey,
		},
	}
}

// AttachObject gets an attachment client based on a service handler, object key and idempotency key.
func AttachObject[O any](c *Client, serviceName, objectKey, handlerName, idempotencyKey string) InvocationHandle[O] {
	return attachClient[O]{
		client: c,
		params: ingress.IngressAttachParams{
			ServiceName:    serviceName,
			ObjectKey:      objectKey,
			MethodName:     handlerName,
			IdempotencyKey: idempotencyKey,
		},
	}
}

// AttachWorkflow gets and attachment client based on a service and a workflow ID.
func AttachWorkflow[O any](c *Client, serviceName, workflowID string) InvocationHandle[O] {
	return attachClient[O]{
		client: c,
		params: ingress.IngressAttachParams{
			ServiceName: serviceName,
			WorkflowID:  workflowID,
		},
	}
}

// Request calls the ingress API with the given input and returns the result.
func (c client[I, O]) Request(ctx context.Context, input I, opts ...options.IngressRequestOption) (O, error) {
	reqOpts := options.IngressRequestOptions{}
	for _, opt := range opts {
		opt.BeforeIngressRequest(&reqOpts)
	}

	var output O
	err := c.client.Request(ctx, c.params, input, &output, reqOpts)
	if err != nil {
		return output, err
	}
	return output, nil
}

// Send calls the ingress API with the given input and returns an Invocation instance.
func (c client[I, O]) Send(ctx context.Context, input I, opts ...options.IngressSendOption) Invocation {
	sendOpts := options.IngressSendOptions{}
	for _, opt := range opts {
		opt.BeforeIngressSend(&sendOpts)
	}

	return c.client.Send(ctx, c.params, input, sendOpts)
}

// Attach calls the attachment API and blocks until the output is available. Returns an
// InvocationNotFoundError if the invocation does not exist.
func (c attachClient[O]) Attach(ctx context.Context) (O, error) {
	var output O
	err := c.client.Attach(ctx, c.params, output)
	if err != nil {
		return output, err
	}
	return output, nil
}

// Output calls the attachment API and returns the output if available, otherwise returns an
// InvocationNotFoundError if the invocation does not exist or an InvocationNotReadyError if
// the invocation is not complete.
func (c attachClient[O]) Output(ctx context.Context) (O, error) {
	var output O
	err := c.client.Output(ctx, c.params, output)
	if err != nil {
		return output, err
	}
	return output, nil
}
