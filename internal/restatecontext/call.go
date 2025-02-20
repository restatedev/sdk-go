package restatecontext

import (
	"fmt"
	"github.com/restatedev/sdk-go/encoding"
	pbinternal "github.com/restatedev/sdk-go/internal/generated"
	"github.com/restatedev/sdk-go/internal/statemachine"
	"time"

	"github.com/restatedev/sdk-go/internal/options"
)

type Client interface {
	RequestFuture(input any, opts ...options.RequestOption) ResponseFuture
	Request(input any, output any, opts ...options.RequestOption) error
	Send(input any, opts ...options.SendOption)
}

type client struct {
	options        options.ClientOptions
	restateContext *ctx
	service        string
	key            string
	method         string
}

// RequestFuture makes a call and returns a coreHandle on the response
func (c *client) RequestFuture(input any, opts ...options.RequestOption) ResponseFuture {
	o := options.RequestOptions{}
	for _, opt := range opts {
		opt.BeforeRequest(&o)
	}

	inputBytes, err := encoding.Marshal(c.options.Codec, input)
	if err != nil {
		panic(fmt.Errorf("failed to marshal RequestFuture input: %w", err))
	}

	inputParams := pbinternal.VmSysCallParameters{}
	inputParams.SetService(c.service)
	if c.key != "" {
		inputParams.SetKey(c.key)
	}
	inputParams.SetHandler(c.method)
	if o.Headers != nil {
		var headers []*pbinternal.Header
		for k, v := range o.Headers {
			h := pbinternal.Header{}
			h.SetKey(k)
			h.SetValue(v)
			headers = append(headers, &h)
		}
		inputParams.SetHeaders(headers)
	}
	inputParams.SetInput(inputBytes)

	_, handle, err := c.restateContext.stateMachine.SysCall(c.restateContext, &inputParams)
	if err != nil {
		panic(err)
	}

	return &responseFuture{
		asyncResult: newAsyncResult(c.restateContext, handle),
		options:     c.options,
	}
}

type ResponseFuture interface {
	Selectable
	Response(output any) error
}

type responseFuture struct {
	asyncResult
	options options.ClientOptions
}

func (d *responseFuture) Response(output any) (err error) {
	switch result := d.pollProgressAndLoadValue().(type) {
	case statemachine.ValueSuccess:
		{
			if err := encoding.Unmarshal(d.options.Codec, result.Success, output); err != nil {
				panic(fmt.Errorf("failed to unmarshal call result into output: %w", err))
			}
			return nil
		}
	case statemachine.ValueFailure:
		return errorFromFailure(result)
	default:
		panic(fmt.Errorf("unexpected value %s", result))
	}
}

// Request makes a call and blocks on the response
func (c *client) Request(input any, output any, opts ...options.RequestOption) error {
	return c.RequestFuture(input, opts...).Response(output)
}

// Send runs a call in the background afterFuture delay duration
func (c *client) Send(input any, opts ...options.SendOption) {
	o := options.SendOptions{}
	for _, opt := range opts {
		opt.BeforeSend(&o)
	}

	inputBytes, err := encoding.Marshal(c.options.Codec, input)
	if err != nil {
		panic(fmt.Errorf("failed to marshal RequestFuture input: %w", err))
	}

	inputParams := pbinternal.VmSysSendParameters{}
	inputParams.SetService(c.service)
	if c.key != "" {
		inputParams.SetKey(c.key)
	}
	inputParams.SetHandler(c.method)
	if o.Headers != nil {
		var headers []*pbinternal.Header
		for k, v := range o.Headers {
			h := pbinternal.Header{}
			h.SetKey(k)
			h.SetValue(v)
			headers = append(headers, &h)
		}
		inputParams.SetHeaders(headers)
	}
	inputParams.SetInput(inputBytes)
	if o.Delay != 0 {
		inputParams.SetExecutionTimeSinceUnixEpochMillis(uint64(time.Now().Add(o.Delay).UnixMilli()))
	}

	_, err = c.restateContext.stateMachine.SysSend(c.restateContext, &inputParams)
	if err != nil {
		panic(err)
	}
}

func (restateCtx *ctx) Service(service, method string, opts ...options.ClientOption) Client {
	o := options.ClientOptions{}
	for _, opt := range opts {
		opt.BeforeClient(&o)
	}
	if o.Codec == nil {
		o.Codec = encoding.JSONCodec
	}

	return &client{
		options:        o,
		restateContext: restateCtx,
		service:        service,
		method:         method,
	}
}

func (restateCtx *ctx) Object(service, key, method string, opts ...options.ClientOption) Client {
	o := options.ClientOptions{}
	for _, opt := range opts {
		opt.BeforeClient(&o)
	}
	if o.Codec == nil {
		o.Codec = encoding.JSONCodec
	}

	return &client{
		options:        o,
		restateContext: restateCtx,
		service:        service,
		key:            key,
		method:         method,
	}
}

func (restateCtx *ctx) Workflow(service, workflowID, method string, opts ...options.ClientOption) Client {
	o := options.ClientOptions{}
	for _, opt := range opts {
		opt.BeforeClient(&o)
	}
	if o.Codec == nil {
		o.Codec = encoding.JSONCodec
	}

	return &client{
		options:        o,
		restateContext: restateCtx,
		service:        service,
		key:            workflowID,
		method:         method,
	}
}
