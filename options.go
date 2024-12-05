package restate

import (
	"time"

	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal/options"
)

// re-export for use in generated code
type ClientOption = options.ClientOption
type ServiceDefinitionOption = options.ServiceDefinitionOption

type withCodec struct {
	codec encoding.Codec
}

var _ options.GetOption = withCodec{}
var _ options.SetOption = withCodec{}
var _ options.RunOption = withCodec{}
var _ options.AwakeableOption = withCodec{}
var _ options.ResolveAwakeableOption = withCodec{}
var _ options.ClientOption = withCodec{}

func (w withCodec) BeforeGet(opts *options.GetOptions)             { opts.Codec = w.codec }
func (w withCodec) BeforeSet(opts *options.SetOptions)             { opts.Codec = w.codec }
func (w withCodec) BeforeRun(opts *options.RunOptions)             { opts.Codec = w.codec }
func (w withCodec) BeforeAwakeable(opts *options.AwakeableOptions) { opts.Codec = w.codec }
func (w withCodec) BeforeResolveAwakeable(opts *options.ResolveAwakeableOptions) {
	opts.Codec = w.codec
}
func (w withCodec) BeforeClient(opts *options.ClientOptions) { opts.Codec = w.codec }

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

func (w withPayloadCodec) BeforeHandler(opts *options.HandlerOptions) {
	opts.Codec = w.codec
}
func (w withPayloadCodec) BeforeServiceDefinition(opts *options.ServiceDefinitionOptions) {
	opts.DefaultCodec = w.codec
}
func (w withPayloadCodec) BeforeIngressClient(opts *options.IngressClientOptions) {
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

type withHeaders struct {
	headers map[string]string
}

var _ options.RequestOption = withHeaders{}
var _ options.SendOption = withHeaders{}

func (w withHeaders) BeforeRequest(opts *options.RequestOptions) {
	opts.Headers = w.headers
}

func (w withHeaders) BeforeSend(opts *options.SendOptions) {
	opts.Headers = w.headers
}

// WithHeaders is an option to specify outgoing headers when making a call
func WithHeaders(headers map[string]string) withHeaders {
	return withHeaders{headers}
}

type withDelay struct {
	delay time.Duration
}

var _ options.SendOption = withDelay{}

func (w withDelay) BeforeSend(opts *options.SendOptions) {
	opts.Delay = w.delay
}

// WithDelay is a [SendOption] to specify the duration to delay the request
func WithDelay(delay time.Duration) withDelay {
	return withDelay{delay}
}

type withIdempotencyKey struct {
	idempotencyKey string
}

var _ options.RequestOption = withIdempotencyKey{}
var _ options.SendOption = withIdempotencyKey{}

func (w withIdempotencyKey) BeforeRequest(opts *options.RequestOptions) {
	opts.IdempotencyKey = w.idempotencyKey
}

func (w withIdempotencyKey) BeforeSend(opts *options.SendOptions) {
	opts.IdempotencyKey = w.idempotencyKey
}

// WithIdempotencyKey is an option to specify an idempotency key for the request
// Currently this key is only used by the ingress client
func WithIdempotencyKey(idempotencyKey string) withIdempotencyKey {
	return withIdempotencyKey{idempotencyKey}
}

type withWorkflowRun struct {
	runHandler string
}

var _ options.WorkflowSubmitOption = withWorkflowRun{}

func (w withWorkflowRun) BeforeWorkflowSubmit(opts *options.WorkflowSubmitOptions) {
	opts.RunHandler = w.runHandler
}

// WithWorkflowRun is a [WorkflowSubmitOption] to specify a different handler name than 'Run' for the
// workflows main handler.
func WithWorkflowRun(runHandler string) withWorkflowRun {
	return withWorkflowRun{runHandler}
}
