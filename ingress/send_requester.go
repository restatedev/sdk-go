package ingress

import (
	"context"

	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal/ingress"
	"github.com/restatedev/sdk-go/internal/options"
)

// SimpleSendResponse represents the result of a send-only invocation (fire-and-forget).
// It provides the invocation ID and status without requiring the output type parameter.
//
// If you need to attach to the invocation later to retrieve its output, you can:
//  1. Create an InvocationHandle using InvocationById with the Id() from this response, or
//  2. Use Service/Object/Workflow functions instead of ServiceSend/ObjectSend/WorkflowSend,
//     which return a full SendResponse[O] that includes an InvocationHandle.
type SimpleSendResponse interface {
	Id() string
	Status() string
}

// SendRequester is a simplified version of Requester that only supports Send operations (fire-and-forget).
// Unlike Requester, it does not require specifying the output type parameter, making it useful when you
// don't need to retrieve the invocation result.
//
// If you need to later retrieve the output, use InvocationById with the Id() from SimpleSendResponse,
// or use Service/Object/Workflow functions instead which return SendResponse[O] with an InvocationHandle.
type SendRequester[I any] interface {
	Send(ctx context.Context, input I, options ...options.IngressSendOption) (SimpleSendResponse, error)
}

// ServiceSend gets a send-only ingress client for a Restate service handler.
//
// This is a simplified version of Service that doesn't require the output type generic parameter.
// Use this when you only need to fire-and-forget invocations and don't need to retrieve results.
//
// Example:
//
//	requester := ingress.ServiceSend[*MyInput](client, "MyService", "myHandler")
//	response, err := requester.Send(ctx, &MyInput{...})
//	fmt.Println("Invocation ID:", response.Id())
func ServiceSend[I any](c *Client, serviceName, handlerName string) SendRequester[I] {
	return sendRequester[I]{
		client: c,
		params: ingress.IngressParams{
			Service: serviceName,
			Handler: handlerName,
		},
	}
}

// ObjectSend gets a send-only ingress client for a Restate virtual object handler.
//
// This is a simplified version of Object that doesn't require the output type generic parameter.
// Use this when you only need to fire-and-forget invocations and don't need to retrieve results.
//
// Example:
//
//	requester := ingress.ObjectSend[*MyInput](client, "MyObject", "object-123", "myHandler")
//	response, err := requester.Send(ctx, &MyInput{...})
//	fmt.Println("Invocation ID:", response.Id())
func ObjectSend[I any](c *Client, serviceName, objectKey, handlerName string) SendRequester[I] {
	return sendRequester[I]{
		client: c,
		params: ingress.IngressParams{
			Service: serviceName,
			Key:     objectKey,
			Handler: handlerName,
		},
	}
}

// WorkflowSend gets a send-only ingress client for a Restate workflow handler.
//
// This is a simplified version of Workflow that doesn't require the output type generic parameter.
// Use this when you only need to fire-and-forget invocations and don't need to retrieve results.
//
// Example:
//
//	requester := ingress.WorkflowSend[*MyInput](client, "MyWorkflow", "workflow-123", "myHandler")
//	response, err := requester.Send(ctx, &MyInput{...})
//	fmt.Println("Invocation ID:", response.Id())
func WorkflowSend[I any](c *Client, serviceName, workflowID, handlerName string) SendRequester[I] {
	return sendRequester[I]{
		client: c,
		params: ingress.IngressParams{
			Service: serviceName,
			Handler: handlerName,
			Key:     workflowID,
		},
	}
}

type sendRequester[I any] struct {
	client *Client
	params ingress.IngressParams
	codec  encoding.PayloadCodec
}

type simpleSendResponse struct {
	ingress.Invocation
}

func (s simpleSendResponse) Id() string {
	return s.Invocation.Id
}

func (s simpleSendResponse) Status() string {
	return s.Invocation.Status
}

// Send calls the ingress API with the given input and returns an Invocation instance.
func (c sendRequester[I]) Send(ctx context.Context, input I, opts ...options.IngressSendOption) (SimpleSendResponse, error) {
	sendOpts := options.IngressSendOptions{}
	sendOpts.Codec = c.codec
	for _, opt := range opts {
		opt.BeforeIngressSend(&sendOpts)
	}

	inv, err := c.client.Send(ctx, c.params, input, sendOpts)
	if err != nil {
		return nil, err
	}

	return simpleSendResponse{inv}, nil
}
