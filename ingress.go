package restate

import (
	"context"

	"github.com/restatedev/sdk-go/internal/ingress"
	"github.com/restatedev/sdk-go/internal/options"
)

type IngressClient = ingress.Client
type IngressInvocation = ingress.Invocation

type IIngressClient[I any, O any] interface {
	Request(ctx context.Context, input I, options ...options.RequestOption) (O, error)
	IngressSendClient[I]
}

type IngressSendClient[I any] interface {
	Send(ctx context.Context, input I, options ...options.SendOption) IngressInvocation
}

type IngressAttachClient[O any] interface {
	Attach(ctx context.Context) (O, error)
	Output(ctx context.Context) (O, error)
}

type ingressClient[I any, O any] struct {
	client *IngressClient
	params ingress.IngressParams
}

type ingressAttachClient[O any] struct {
	client *ingress.Client
	params ingress.IngressAttachParams
}

func NewIngressClient(baseUri string, opts ...options.IngressClientOption) *IngressClient {
	clientOpts := options.IngressClientOptions{}
	for _, opt := range opts {
		opt.BeforeIngress(&clientOpts)
	}
	return ingress.NewClient(baseUri, clientOpts)
}

// IngressService gets a Service request ingress client by service and method name
func IngressService[I any, O any](client *IngressClient, service, method string) IIngressClient[I, O] {
	return ingressClient[I, O]{
		client: client,
		params: ingress.IngressParams{
			Service: service,
			Method:  method,
		},
	}
}

// IngressServiceSend gets a Service send ingress client by service and method name
func IngressServiceSend[I any](client *IngressClient, service, method string) IngressSendClient[I] {
	return ingressClient[I, any]{
		client: client,
		params: ingress.IngressParams{
			Service: service,
			Method:  method,
		},
	}
}

// IngressObject gets an Object request ingress client by service name, key and method name
func IngressObject[I any, O any](client *IngressClient, service, key, method string) IIngressClient[I, O] {
	return ingressClient[I, O]{
		client: client,
		params: ingress.IngressParams{
			Service: service,
			Key:     key,
			Method:  method,
		},
	}
}

// IngressObjectSend gets an Object send ingress client by service name, key and method name
func IngressObjectSend[I any](client *IngressClient, service, key, method string) IngressSendClient[I] {
	return ingressClient[I, any]{
		client: client,
		params: ingress.IngressParams{
			Service: service,
			Key:     key,
			Method:  method,
		},
	}
}

// IngressWorkflow gets a Workflow request ingress client by service name, workflow ID and method name
func IngressWorkflow[I any, O any](client *IngressClient, service, workflowID, method string) IIngressClient[I, O] {
	return ingressClient[I, O]{
		client: client,
		params: ingress.IngressParams{
			Service:    service,
			Method:     method,
			WorkflowID: workflowID,
		},
	}
}

// IngressWorkflowSend gets a Workflow send ingress client by service name, workflow ID and method name
func IngressWorkflowSend[I any](client *IngressClient, service, workflowID, method string) IngressSendClient[I] {
	return ingressClient[I, any]{
		client: client,
		params: ingress.IngressParams{
			Service:    service,
			Method:     method,
			WorkflowID: workflowID,
		},
	}
}

func IngressAttachInvocation[O any](client *IngressClient, invocationID string) IngressAttachClient[O] {
	return ingressAttachClient[O]{
		client: client,
		params: ingress.IngressAttachParams{
			InvocationID: invocationID,
		},
	}
}

func IngressAttachService[O any](client *IngressClient, service, method, idempotencyKey string) IngressAttachClient[O] {
	return ingressAttachClient[O]{
		client: client,
		params: ingress.IngressAttachParams{
			Service:        service,
			Method:         method,
			IdempotencyKey: idempotencyKey,
		},
	}
}

func IngressAttachObject[O any](client *IngressClient, service, key, method, idempotencyKey string) IngressAttachClient[O] {
	return ingressAttachClient[O]{
		client: client,
		params: ingress.IngressAttachParams{
			Service:        service,
			Key:            key,
			Method:         method,
			IdempotencyKey: idempotencyKey,
		},
	}
}

func IngressAttachWorkflow[O any](client *IngressClient, service, workflowID string) IngressAttachClient[O] {
	return ingressAttachClient[O]{
		client: client,
		params: ingress.IngressAttachParams{
			Service:    service,
			WorkflowID: workflowID,
		},
	}
}

func (c ingressClient[I, O]) Request(ctx context.Context, input I, opts ...options.RequestOption) (O, error) {
	reqOpts := options.RequestOptions{}
	for _, opt := range opts {
		opt.BeforeRequest(&reqOpts)
	}

	var output O
	err := c.client.Request(ctx, c.params, input, output, reqOpts)
	if err != nil {
		return output, err
	}
	return output, nil
}

func (c ingressClient[I, O]) Send(ctx context.Context, input I, opts ...options.SendOption) IngressInvocation {
	sendOpts := options.SendOptions{}
	for _, opt := range opts {
		opt.BeforeSend(&sendOpts)
	}

	return c.client.Send(ctx, c.params, input, sendOpts)
}

func (c ingressAttachClient[O]) Attach(ctx context.Context) (O, error) {
	var output O
	err := c.client.Attach(ctx, c.params, output)
	if err != nil {
		return output, err
	}
	return output, nil
}

func (c ingressAttachClient[O]) Output(ctx context.Context) (O, error) {
	var output O
	err := c.client.Output(ctx, c.params, output)
	if err != nil {
		return output, err
	}
	return output, nil
}
