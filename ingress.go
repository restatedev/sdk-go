package restate

import (
	"context"
	"errors"

	"github.com/restatedev/sdk-go/internal/ingress"
	"github.com/restatedev/sdk-go/internal/options"
)

type IngressInvocation = ingress.Invocation

type IngressClient[I any, O any] interface {
	Request(ctx context.Context, input I, options ...options.RequestOption) (O, error)
	IngressSendClient[I]
}

type IngressSendClient[I any] interface {
	Send(ctx context.Context, input I, options ...options.SendOption) IngressInvocation
}

type IngressInvocationClient[O any] interface {
	IngressAttachClient[O]
	Cancel(ctx context.Context) error
}

type IngressAttachClient[O any] interface {
	Attach(ctx context.Context) (O, error)
	Output(ctx context.Context) (O, error)
}

type ingressClient[I any, O any] struct {
	opts   []options.IngressOption
	params ingress.IngressParams
}

type ingressInvocationClient[O any] struct {
	opts   []options.IngressOption
	params ingress.IngressAttachParams
}

type ingressAttachClient[O any] struct {
	opts   []options.IngressOption
	params ingress.IngressAttachParams
}

// IngressService gets a Service request ingress client by service and method name
func IngressService[I any, O any](service, method string, opts ...options.IngressOption) IngressClient[I, O] {
	return ingressClient[I, O]{
		opts: opts,
		params: ingress.IngressParams{
			Service: service,
			Method:  method,
		},
	}
}

// IngressServiceSend gets a Service send ingress client by service and method name
func IngressServiceSend[I any](service, method string, opts ...options.IngressOption) IngressSendClient[I] {
	return ingressClient[I, any]{
		opts: opts,
		params: ingress.IngressParams{
			Service: service,
			Method:  method,
		},
	}
}

// IngressObject gets an Object request ingress client by service name, key and method name
func IngressObject[I any, O any](service, key, method string, opts ...options.IngressOption) IngressClient[I, O] {
	return ingressClient[I, O]{
		opts: opts,
		params: ingress.IngressParams{
			Service: service,
			Key:     key,
			Method:  method,
		},
	}
}

// IngressObjectSend gets an Object send ingress client by service name, key and method name
func IngressObjectSend[I any](service, key, method string, opts ...options.IngressOption) IngressSendClient[I] {
	return ingressClient[I, any]{
		opts: opts,
		params: ingress.IngressParams{
			Service: service,
			Key:     key,
			Method:  method,
		},
	}
}

// IngressWorkflow gets a Workflow request ingress client by service name, workflow ID and method name
func IngressWorkflow[I any, O any](service, workflowID, method string, opts ...options.IngressOption) IngressClient[I, O] {
	return ingressClient[I, O]{
		opts: opts,
		params: ingress.IngressParams{
			Service:    service,
			Method:     method,
			WorkflowID: workflowID,
		},
	}
}

// IngressWorkflowSend gets a Workflow send ingress client by service name, workflow ID and method name
func IngressWorkflowSend[I any](service, workflowID, method string, opts ...options.IngressOption) IngressSendClient[I] {
	return ingressClient[I, any]{
		opts: opts,
		params: ingress.IngressParams{
			Service:    service,
			Method:     method,
			WorkflowID: workflowID,
		},
	}
}

func IngressAttachInvocation[O any](invocationID string, opts ...options.IngressOption) IngressInvocationClient[O] {
	return ingressInvocationClient[O]{
		opts: opts,
		params: ingress.IngressAttachParams{
			InvocationID: invocationID,
		},
	}
}

func IngressAttachService[O any](service, method, idempotencyKey string, opts ...options.IngressOption) IngressAttachClient[O] {
	return ingressAttachClient[O]{
		opts: opts,
		params: ingress.IngressAttachParams{
			Service:        service,
			Method:         method,
			IdempotencyKey: idempotencyKey,
		},
	}
}

func IngressAttachObject[O any](service, key, method, idempotencyKey string, opts ...options.IngressOption) IngressAttachClient[O] {
	return ingressAttachClient[O]{
		opts: opts,
		params: ingress.IngressAttachParams{
			Service:        service,
			Key:            key,
			Method:         method,
			IdempotencyKey: idempotencyKey,
		},
	}
}

func IngressAttachWorkflow[O any](service, workflowID string, opts ...options.IngressOption) IngressAttachClient[O] {
	return ingressAttachClient[O]{
		opts: opts,
		params: ingress.IngressAttachParams{
			Service:    service,
			WorkflowID: workflowID,
		},
	}
}

func (c ingressClient[I, O]) Request(ctx context.Context, input I, opts ...options.RequestOption) (O, error) {
	ingOpts := options.IngressOptions{}
	for _, opt := range c.opts {
		opt.BeforeIngress(&ingOpts)
	}

	reqOpts := options.RequestOptions{}
	for _, opt := range opts {
		opt.BeforeRequest(&reqOpts)
	}

	ic := ingress.NewClient(ingOpts.BaseUrl)
	var output O
	err := ic.Request(ctx, c.params, input, output, reqOpts)
	if err != nil {
		return output, err
	}
	return output, nil
}

func (c ingressClient[I, O]) Send(ctx context.Context, input I, opts ...options.SendOption) IngressInvocation {
	ingOpts := options.IngressOptions{}
	for _, opt := range c.opts {
		opt.BeforeIngress(&ingOpts)
	}

	sendOpts := options.SendOptions{}
	for _, opt := range opts {
		opt.BeforeSend(&sendOpts)
	}

	ic := ingress.NewClient(ingOpts.BaseUrl)
	return ic.Send(ctx, c.params, input, sendOpts)
}

func (c ingressInvocationClient[O]) Attach(ctx context.Context) (O, error) {
	ingOpts := options.IngressOptions{}
	for _, opt := range c.opts {
		opt.BeforeIngress(&ingOpts)
	}

	ic := ingress.NewClient(ingOpts.BaseUrl)
	var output O
	err := ic.Attach(ctx, c.params, output)
	if err != nil {
		return output, err
	}
	return output, nil
}

func (c ingressInvocationClient[O]) Output(ctx context.Context) (O, error) {
	ingOpts := options.IngressOptions{}
	for _, opt := range c.opts {
		opt.BeforeIngress(&ingOpts)
	}

	ic := ingress.NewClient(ingOpts.BaseUrl)
	var output O
	err := ic.Output(ctx, c.params, output)
	if err != nil {
		return output, err
	}
	return output, nil
}

// Cancel attempts to cancel the invocation. This call is made against the Admin API.
func (c ingressInvocationClient[O]) Cancel(ctx context.Context) error {
	if c.params.InvocationID == "" {
		return errors.New("cancel can only be called with an invocation ID")
	}
	ingOpts := options.IngressOptions{}
	for _, opt := range c.opts {
		opt.BeforeIngress(&ingOpts)
	}

	ic := ingress.NewClient(ingOpts.BaseUrl)
	return ic.Cancel(ctx, c.params.InvocationID)
}

func (c ingressAttachClient[O]) Attach(ctx context.Context) (O, error) {
	ingOpts := options.IngressOptions{}
	for _, opt := range c.opts {
		opt.BeforeIngress(&ingOpts)
	}

	ic := ingress.NewClient(ingOpts.BaseUrl)
	var output O
	err := ic.Attach(ctx, c.params, output)
	if err != nil {
		return output, err
	}
	return output, nil
}

func (c ingressAttachClient[O]) Output(ctx context.Context) (O, error) {
	ingOpts := options.IngressOptions{}
	for _, opt := range c.opts {
		opt.BeforeIngress(&ingOpts)
	}

	ic := ingress.NewClient(ingOpts.BaseUrl)
	var output O
	err := ic.Output(ctx, c.params, output)
	if err != nil {
		return output, err
	}
	return output, nil
}
