package restate

import (
	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal/options"
)

// Codec options apply across any operation that performs (de)serialisation.

type withCodec struct {
	codec encoding.Codec
}

var _ options.GetOption = withCodec{}
var _ options.SetOption = withCodec{}
var _ options.RunOption = withCodec{}
var _ options.AwakeableOption = withCodec{}
var _ options.SignalOption = withCodec{}
var _ options.PromiseOption = withCodec{}
var _ options.ResolveAwakeableOption = withCodec{}
var _ options.ResolveSignalOption = withCodec{}
var _ options.ClientOption = withCodec{}
var _ options.AttachOption = withCodec{}

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
func (w withCodec) BeforeClient(opts *options.ClientOptions) { opts.Codec = w.codec }
func (w withCodec) BeforeAttach(opts *options.AttachOptions) { opts.Codec = w.codec }

// WithCodec is an option that can be provided to many different functions that perform (de)serialisation
// in order to specify a custom codec with which to (de)serialise instead of the default of JSON.
//
// See also [WithProto], [WithBinary], [WithJSON].
func WithCodec(codec encoding.Codec) withCodec {
	return withCodec{codec}
}

type withPayloadCodec struct {
	withCodec
	codec encoding.PayloadCodec
}

var _ options.HandlerOption = withPayloadCodec{}
var _ options.ServiceDefinitionOption = withPayloadCodec{}
var _ options.IngressClientOption = withPayloadCodec{}
var _ options.IngressRequestOption = withPayloadCodec{}
var _ options.IngressSendOption = withPayloadCodec{}
var _ options.IngressInvocationHandleOption = withPayloadCodec{}

func (w withPayloadCodec) BeforeHandler(opts *options.HandlerOptions) {
	opts.Codec = w.codec
}
func (w withPayloadCodec) BeforeServiceDefinition(opts *options.ServiceDefinitionOptions) {
	opts.DefaultCodec = w.codec
}
func (w withPayloadCodec) BeforeIngress(opts *options.IngressClientOptions) {
	opts.Codec = w.codec
}
func (w withPayloadCodec) BeforeIngressRequest(opts *options.IngressRequestOptions) {
	opts.Codec = w.codec
}
func (w withPayloadCodec) BeforeIngressSend(opts *options.IngressSendOptions) {
	opts.Codec = w.codec
}
func (w withPayloadCodec) BeforeIngressInvocationHandle(opts *options.IngressInvocationHandleOptions) {
	opts.Codec = w.codec
}

// WithPayloadCodec is an option that can be provided to handler/service options
// in order to specify a custom [encoding.PayloadCodec] with which to (de)serialise and
// set content-types instead of the default of JSON.
//
// See also [WithProto], [WithBinary], [WithJSON].
func WithPayloadCodec(codec encoding.PayloadCodec) withPayloadCodec {
	return withPayloadCodec{withCodec{codec}, codec}
}

// WithProto is an option to specify the use of [encoding.ProtoCodec] for (de)serialisation
var WithProto = WithPayloadCodec(encoding.ProtoCodec)

// WithProtoJSON is an option to specify the use of [encoding.ProtoJSONCodec] for (de)serialisation
var WithProtoJSON = WithPayloadCodec(encoding.ProtoJSONCodec)

// WithBinary is an option to specify the use of [encoding.BinaryCodec] for (de)serialisation
var WithBinary = WithPayloadCodec(encoding.BinaryCodec)

// WithJSON is an option to specify the use of [encoding.JsonCodec] for (de)serialisation
var WithJSON = WithPayloadCodec(encoding.JSONCodec)
