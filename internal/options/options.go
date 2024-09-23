package options

import (
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

type RequestOptions struct {
	Headers map[string]string
}

type RequestOption interface {
	BeforeRequest(*RequestOptions)
}

type SendOptions struct {
	Headers map[string]string
	Delay   time.Duration
}

type SendOption interface {
	BeforeSend(*SendOptions)
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
