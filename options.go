package restate

import "github.com/restatedev/sdk-go/encoding"

type withCodec struct {
	codec encoding.Codec
}

var _ GetOption = withCodec{}
var _ SetOption = withCodec{}
var _ RunOption = withCodec{}
var _ AwakeableOption = withCodec{}
var _ ResolveAwakeableOption = withCodec{}
var _ CallOption = withCodec{}

func (w withCodec) beforeGet(opts *getOptions)                           { opts.codec = w.codec }
func (w withCodec) beforeSet(opts *setOptions)                           { opts.codec = w.codec }
func (w withCodec) beforeRun(opts *runOptions)                           { opts.codec = w.codec }
func (w withCodec) beforeAwakeable(opts *awakeableOptions)               { opts.codec = w.codec }
func (w withCodec) beforeResolveAwakeable(opts *resolveAwakeableOptions) { opts.codec = w.codec }
func (w withCodec) beforeCall(opts *callOptions)                         { opts.codec = w.codec }

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

var WithProto = WithPayloadCodec(encoding.ProtoCodec{})
