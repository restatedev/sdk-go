package ingress

import (
	"context"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal/ingress"
	"github.com/restatedev/sdk-go/internal/options"
)

// Requester provides both synchronous (Request) and asynchronous (Send) invocation methods for Restate handlers.
// It requires both input (I) and output (O) type parameters.
//
// Use Request to make a call and wait for the result.
// Use Send to make a fire-and-forget call that returns immediately with a SendResponse
// containing an InvocationHandle to retrieve the result later.
type Requester[I any, O any] interface {
	// Request makes a synchronous invocation and blocks until the result is available.
	Request(ctx context.Context, input I, options ...options.IngressRequestOption) (O, error)
	// Send makes an asynchronous invocation and returns immediately with a handle to retrieve the result later.
	Send(ctx context.Context, input I, options ...options.IngressSendOption) (SendResponse[O], error)
}

// SendResponse is returned by Requester.Send and combines both SimpleSendResponse (for invocation metadata)
// and InvocationHandle (for retrieving the output).
//
// You can use the embedded InvocationHandle methods (Attach/Output) to retrieve the invocation result,
// or use the SimpleSendResponse methods (Id/Status) to get invocation metadata.
type SendResponse[O any] interface {
	InvocationHandle[O]
	SimpleSendResponse
}

// Service gets an ingress client for a Restate service handler.
// This returns a Requester that supports both Request and Send operations.
//
// Example:
//
//	requester := ingress.Service[*MyInput, *MyOutput](client, "MyService", "myHandler")
//	// Call and wait for response:
//	output, err := requester.Request(ctx, &MyInput{...})
//	// Send request:
//	response, err := requester.Send(ctx, &MyInput{...})
func Service[I any, O any](c *Client, serviceName, handlerName string) Requester[I, O] {
	return requester[I, O]{
		client: c,
		params: ingress.IngressParams{
			Service: serviceName,
			Handler: handlerName,
		},
	}
}

// Object gets an ingress client for a Restate virtual object handler.
// This returns a Requester that supports both Request and Send operations.
//
// Example:
//
//	requester := ingress.Object[*MyInput, *MyOutput](client, "MyObject", "object-123", "myHandler")
//	// Call and wait for response:
//	output, err := requester.Request(ctx, &MyInput{...})
//	// Send request:
//	response, err := requester.Send(ctx, &MyInput{...})
func Object[I any, O any](c *Client, serviceName, objectKey, handlerName string) Requester[I, O] {
	return requester[I, O]{
		client: c,
		params: ingress.IngressParams{
			Service: serviceName,
			Key:     objectKey,
			Handler: handlerName,
		},
	}
}

// Workflow gets an ingress client for a Restate workflow handler.
// This returns a Requester that supports both Request and Send operations.
//
// Example:
//
//	requester := ingress.Workflow[*MyInput, *MyOutput](client, "MyWorkflow", "workflow-123", "myHandler")
//	// Call and wait for response:
//	output, err := requester.Request(ctx, &MyInput{...})
//	// Send request:
//	response, err := requester.Send(ctx, &MyInput{...})
func Workflow[I any, O any](c *Client, serviceName, workflowID, handlerName string) Requester[I, O] {
	return requester[I, O]{
		client: c,
		params: ingress.IngressParams{
			Service: serviceName,
			Handler: handlerName,
			Key:     workflowID,
		},
	}
}

type requester[I any, O any] struct {
	client *Client
	params ingress.IngressParams
	codec  encoding.PayloadCodec
}

func NewRequester[I any, O any](c *Client, serviceName, handlerName string, key *string, codec *encoding.PayloadCodec) Requester[I, O] {
	req := requester[I, O]{
		client: c,
		params: ingress.IngressParams{
			Service: serviceName,
			Handler: handlerName,
		},
	}
	if key != nil {
		req.params.Key = *key
	}
	if codec != nil {
		req.codec = *codec
	}
	return req
}

// Request calls the ingress API with the given input and returns the result.
func (c requester[I, O]) Request(ctx context.Context, input I, opts ...options.IngressRequestOption) (O, error) {
	reqOpts := options.IngressRequestOptions{}
	reqOpts.Codec = c.codec
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

type sendResponse[O any] struct {
	InvocationHandle[O]
	invocation ingress.Invocation
}

func (s *sendResponse[O]) Id() string {
	return s.invocation.Id
}

func (s *sendResponse[O]) Status() string {
	return s.invocation.Status
}

// Send calls the ingress API with the given input and returns an Invocation instance.
func (c requester[I, O]) Send(ctx context.Context, input I, opts ...options.IngressSendOption) (SendResponse[O], error) {
	sendOpts := options.IngressSendOptions{}
	sendOpts.Codec = c.codec
	for _, opt := range opts {
		opt.BeforeIngressSend(&sendOpts)
	}

	inv, err := c.client.Send(ctx, c.params, input, sendOpts)
	if err != nil {
		return nil, err
	}

	return &sendResponse[O]{
		invocation:       inv,
		InvocationHandle: InvocationById[O](c.client, inv.Id, restate.WithPayloadCodec(c.codec)),
	}, nil
}
