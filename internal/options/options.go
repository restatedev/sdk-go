package options

import (
	"net/http"
	"time"

	"github.com/restatedev/sdk-go/encoding"
)

type AwakeableOptions struct {
	Codec encoding.Codec
}

type AwakeableOption interface {
	BeforeAwakeable(*AwakeableOptions)
}

type PromiseOptions struct {
	Codec encoding.Codec
}

type PromiseOption interface {
	BeforePromise(*PromiseOptions)
}

type ResolveAwakeableOptions struct {
	Codec encoding.Codec
}

type ResolveAwakeableOption interface {
	BeforeResolveAwakeable(*ResolveAwakeableOptions)
}

type GetOptions struct {
	Codec encoding.Codec
}

type GetOption interface {
	BeforeGet(*GetOptions)
}

type SetOptions struct {
	Codec encoding.Codec
}

type SetOption interface {
	BeforeSet(*SetOptions)
}

type ClientOptions struct {
	Codec encoding.Codec
}

type ClientOption interface {
	BeforeClient(*ClientOptions)
}

type IngressClientOptions struct {
	Codec encoding.PayloadCodec
}

type IngressClientOption interface {
	BeforeIngressClient(*IngressClientOptions)
}

type RequestOptions struct {
	Headers map[string]string
	// IdempotencyKey is currently only supported in ingress clients
	IdempotencyKey string
}

type RequestOption interface {
	BeforeRequest(*RequestOptions)
}

type SendOptions struct {
	Headers map[string]string
	Delay   time.Duration
	// IdempotencyKey is currently only supported in ingress clients
	IdempotencyKey string
}

type SendOption interface {
	BeforeSend(*SendOptions)
}

type WorkflowSubmitOptions struct {
	IngressClientOptions
	SendOptions
	RunHandler string
}

var _ SendOption = WorkflowSubmitOptions{}
var _ IngressClientOption = WorkflowSubmitOptions{}

func (w WorkflowSubmitOptions) BeforeSend(opts *SendOptions) {
	if w.SendOptions.Headers != nil {
		opts.Headers = w.SendOptions.Headers
	}
	if w.SendOptions.Delay != 0 {
		opts.Delay = w.SendOptions.Delay
	}
	if w.SendOptions.IdempotencyKey != "" {
		opts.IdempotencyKey = w.SendOptions.IdempotencyKey
	}
}

func (w WorkflowSubmitOptions) BeforeIngressClient(opts *IngressClientOptions) {
	if w.IngressClientOptions.Codec != nil {
		opts.Codec = w.IngressClientOptions.Codec
	}
}

type WorkflowSubmitOption interface {
	BeforeWorkflowSubmit(*WorkflowSubmitOptions)
}

type RunOptions struct {
	Codec encoding.Codec
}

type RunOption interface {
	BeforeRun(*RunOptions)
}

type HandlerOptions struct {
	Codec encoding.PayloadCodec
}

type HandlerOption interface {
	BeforeHandler(*HandlerOptions)
}

type ServiceDefinitionOptions struct {
	DefaultCodec encoding.PayloadCodec
}

type ServiceDefinitionOption interface {
	BeforeServiceDefinition(*ServiceDefinitionOptions)
}

type ConnectOptions struct {
	Headers map[string]string
	Client  *http.Client
}

type ConnectOption interface {
	BeforeConnect(*ConnectOptions)
}
