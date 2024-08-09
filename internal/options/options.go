package options

import "github.com/restatedev/sdk-go/encoding"

type AwakeableOptions struct {
	Codec encoding.Codec
}

type AwakeableOption interface {
	BeforeAwakeable(*AwakeableOptions)
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

type CallOptions struct {
	Codec   encoding.Codec
	Headers map[string]string
}

type CallOption interface {
	BeforeCall(*CallOptions)
}

type RunOptions struct {
	Codec encoding.Codec
}

type RunOption interface {
	BeforeRun(*RunOptions)
}

type ServiceHandlerOptions struct {
	Codec encoding.PayloadCodec
}

type ServiceHandlerOption interface {
	BeforeServiceHandler(*ServiceHandlerOptions)
}

type ObjectHandlerOptions struct {
	Codec encoding.PayloadCodec
}

type ObjectHandlerOption interface {
	BeforeObjectHandler(*ObjectHandlerOptions)
}

type ServiceOptions struct {
	DefaultCodec encoding.PayloadCodec
}

type ServiceOption interface {
	BeforeService(*ServiceOptions)
}

type ObjectOptions struct {
	DefaultCodec encoding.PayloadCodec
}

type ObjectOption interface {
	BeforeObject(*ObjectOptions)
}
