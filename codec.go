package restate

import (
	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal/options"
)

// Codec options select how values are (de)serialised. A single [encoding.Codec] now
// applies everywhere - value operations, handlers, calls, services and ingress - so
// [WithCodec] works in every position. For handlers and calls (which have both an input
// and an output) [WithInputCodec] / [WithOutputCodec] override a single direction.

type withCodec struct {
	codec encoding.Codec
}

var (
	_ options.GetOption                     = withCodec{}
	_ options.SetOption                     = withCodec{}
	_ options.RunOption                     = withCodec{}
	_ options.AwakeableOption               = withCodec{}
	_ options.SignalOption                  = withCodec{}
	_ options.PromiseOption                 = withCodec{}
	_ options.ResolveAwakeableOption        = withCodec{}
	_ options.ResolveSignalOption           = withCodec{}
	_ options.AttachOption                  = withCodec{}
	_ options.ClientOption                  = withCodec{}
	_ options.HandlerOption                 = withCodec{}
	_ options.ServiceDefinitionOption       = withCodec{}
	_ options.IngressClientOption           = withCodec{}
	_ options.IngressRequestOption          = withCodec{}
	_ options.IngressSendOption             = withCodec{}
	_ options.IngressInvocationHandleOption = withCodec{}
)

func (w withCodec) BeforeGet(opts *options.GetOptions)             { opts.Codec = w.codec }
func (w withCodec) BeforeSet(opts *options.SetOptions)             { opts.Codec = w.codec }
func (w withCodec) BeforeRun(opts *options.RunOptions)             { opts.Codec = w.codec }
func (w withCodec) BeforeAwakeable(opts *options.AwakeableOptions) { opts.Codec = w.codec }
func (w withCodec) BeforeSignal(opts *options.SignalOptions)       { opts.Codec = w.codec }
func (w withCodec) BeforePromise(opts *options.PromiseOptions)     { opts.Codec = w.codec }
func (w withCodec) BeforeResolveAwakeable(opts *options.ResolveAwakeableOptions) {
	opts.Codec = w.codec
}
func (w withCodec) BeforeResolveSignal(opts *options.ResolveSignalOptions) {
	opts.Codec = w.codec
}
func (w withCodec) BeforeAttach(opts *options.AttachOptions) { opts.Codec = w.codec }

// On clients and handlers, WithCodec sets both directions.
func (w withCodec) BeforeClient(opts *options.ClientOptions) {
	opts.InputCodec = w.codec
	opts.OutputCodec = w.codec
}
func (w withCodec) BeforeHandler(opts *options.HandlerOptions) {
	opts.InputCodec = w.codec
	opts.OutputCodec = w.codec
}
func (w withCodec) BeforeServiceDefinition(opts *options.ServiceDefinitionOptions) {
	opts.DefaultCodec = w.codec
}
func (w withCodec) BeforeIngress(opts *options.IngressClientOptions) { opts.Codec = w.codec }
func (w withCodec) BeforeIngressRequest(opts *options.IngressRequestOptions) {
	opts.InputCodec = w.codec
	opts.OutputCodec = w.codec
}
func (w withCodec) BeforeIngressSend(opts *options.IngressSendOptions) { opts.Codec = w.codec }
func (w withCodec) BeforeIngressInvocationHandle(opts *options.IngressInvocationHandleOptions) {
	opts.Codec = w.codec
}

// WithCodec sets the [encoding.Codec] used to (de)serialise values. It applies to any
// operation that (de)serialises; on handlers and calls it sets both the input and output
// codec (override one with [WithInputCodec] / [WithOutputCodec]).
//
// See also [WithProto], [WithBinary], [WithJSON], [WithProtoJSON].
func WithCodec(codec encoding.Codec) withCodec {
	return withCodec{codec}
}

type withInputCodec struct {
	codec encoding.Codec
}

var (
	_ options.ClientOption         = withInputCodec{}
	_ options.HandlerOption        = withInputCodec{}
	_ options.IngressRequestOption = withInputCodec{}
)

func (w withInputCodec) BeforeClient(opts *options.ClientOptions)   { opts.InputCodec = w.codec }
func (w withInputCodec) BeforeHandler(opts *options.HandlerOptions) { opts.InputCodec = w.codec }
func (w withInputCodec) BeforeIngressRequest(opts *options.IngressRequestOptions) {
	opts.InputCodec = w.codec
}

// WithInputCodec sets the [encoding.Codec] used to (de)serialise the input of a handler
// or call, independently of the output.
func WithInputCodec(codec encoding.Codec) withInputCodec {
	return withInputCodec{codec}
}

type withOutputCodec struct {
	codec encoding.Codec
}

var (
	_ options.ClientOption         = withOutputCodec{}
	_ options.HandlerOption        = withOutputCodec{}
	_ options.IngressRequestOption = withOutputCodec{}
)

func (w withOutputCodec) BeforeClient(opts *options.ClientOptions)   { opts.OutputCodec = w.codec }
func (w withOutputCodec) BeforeHandler(opts *options.HandlerOptions) { opts.OutputCodec = w.codec }
func (w withOutputCodec) BeforeIngressRequest(opts *options.IngressRequestOptions) {
	opts.OutputCodec = w.codec
}

// WithOutputCodec sets the [encoding.Codec] used to (de)serialise the output of a handler
// or call, independently of the input.
func WithOutputCodec(codec encoding.Codec) withOutputCodec {
	return withOutputCodec{codec}
}

// WithProto is an option to specify the use of [encoding.ProtoCodec] for (de)serialisation
var WithProto = WithCodec(encoding.ProtoCodec)

// WithProtoJSON is an option to specify the use of [encoding.ProtoJSONCodec] for (de)serialisation
var WithProtoJSON = WithCodec(encoding.ProtoJSONCodec)

// WithBinary is an option to specify the use of [encoding.BinaryCodec] for (de)serialisation
var WithBinary = WithCodec(encoding.BinaryCodec)

// WithJSON is an option to specify the use of [encoding.JSONCodec] for (de)serialisation
var WithJSON = WithCodec(encoding.JSONCodec)
