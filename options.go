package restate

import (
	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal/options"
)

type withCodec struct {
	codec encoding.Codec
}

var _ options.GetOption = withCodec{}
var _ options.SetOption = withCodec{}
var _ options.RunOption = withCodec{}
var _ options.AwakeableOption = withCodec{}
var _ options.ResolveAwakeableOption = withCodec{}
var _ options.CallOption = withCodec{}

func (w withCodec) BeforeGet(opts *options.GetOptions)             { opts.Codec = w.codec }
func (w withCodec) BeforeSet(opts *options.SetOptions)             { opts.Codec = w.codec }
func (w withCodec) BeforeRun(opts *options.RunOptions)             { opts.Codec = w.codec }
func (w withCodec) BeforeAwakeable(opts *options.AwakeableOptions) { opts.Codec = w.codec }
func (w withCodec) BeforeResolveAwakeable(opts *options.ResolveAwakeableOptions) {
	opts.Codec = w.codec
}
func (w withCodec) BeforeCall(opts *options.CallOptions) { opts.Codec = w.codec }

func WithCodec(codec encoding.Codec) withCodec {
	return withCodec{codec}
}

type withPayloadCodec struct {
	withCodec
	codec encoding.PayloadCodec
}

var _ ServiceHandlerOption = withPayloadCodec{}
var _ ServiceRouterOption = withPayloadCodec{}
var _ ObjectHandlerOption = withPayloadCodec{}
var _ ObjectRouterOption = withPayloadCodec{}

func (w withPayloadCodec) beforeServiceHandler(opts *serviceHandlerOptions) { opts.codec = w.codec }
func (w withPayloadCodec) beforeObjectHandler(opts *objectHandlerOptions)   { opts.codec = w.codec }
func (w withPayloadCodec) beforeServiceRouter(opts *serviceRouterOptions) {
	opts.defaultCodec = w.codec
}
func (w withPayloadCodec) beforeObjectRouter(opts *objectRouterOptions) { opts.defaultCodec = w.codec }

func WithPayloadCodec(codec encoding.PayloadCodec) withPayloadCodec {
	return withPayloadCodec{withCodec{codec}, codec}
}

var WithProto = WithPayloadCodec(encoding.ProtoCodec)
var WithBinary = WithPayloadCodec(encoding.BinaryCodec)
var WithJSON = WithPayloadCodec(encoding.JSONCodec)
