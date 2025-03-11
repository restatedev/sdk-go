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

func (w withPayloadCodec) BeforeHandler(opts *options.HandlerOptions) {
	opts.Codec = w.codec
}
func (w withPayloadCodec) BeforeServiceDefinition(opts *options.ServiceDefinitionOptions) {
	opts.DefaultCodec = w.codec
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

// WithIdempotencyKey is an option to specify the idempotency key to set when making a call
func WithIdempotencyKey(idempotencyKey string) withIdempotencyKey {
	return withIdempotencyKey{idempotencyKey}
}

type withDelay struct {
	delay time.Duration
}

var _ options.SendOption = withDelay{}

func (w withDelay) BeforeSend(opts *options.SendOptions) {
	opts.Delay = w.delay
}

// WithDelay is an [SendOption] to specify the duration to delay the request
func WithDelay(delay time.Duration) withDelay {
	return withDelay{delay}
}

type withMaxRetryAttempts struct {
	maxAttempts uint
}

var _ options.RunOption = withMaxRetryAttempts{}

func (w withMaxRetryAttempts) BeforeRun(opts *options.RunOptions) {
	opts.MaxRetryAttempts = &w.maxAttempts
}

// WithMaxRetryAttempts sets the MaxRetryAttempts before giving up.
//
// When giving up, Run will return a TerminalError wrapping the original error message.
func WithMaxRetryAttempts(maxAttempts uint) withMaxRetryAttempts {
	return withMaxRetryAttempts{maxAttempts}
}

type withMaxRetryDuration struct {
	maxRetryDuration time.Duration
}

var _ options.RunOption = withMaxRetryDuration{}

func (w withMaxRetryDuration) BeforeRun(opts *options.RunOptions) {
	opts.MaxRetryDuration = &w.maxRetryDuration
}

// WithMaxRetryDuration sets the MaxRetryDuration before giving up.
//
// When giving up, Run will return a TerminalError wrapping the original error message.
func WithMaxRetryDuration(maxRetryDuration time.Duration) withMaxRetryDuration {
	return withMaxRetryDuration{maxRetryDuration}
}

type withInitialRetryInterval struct {
	initialRetryInterval time.Duration
}

var _ options.RunOption = withInitialRetryInterval{}

func (w withInitialRetryInterval) BeforeRun(opts *options.RunOptions) {
	opts.InitialRetryInterval = &w.initialRetryInterval
}

// WithInitialRetryInterval sets the InitialRetryInterval for the first retry attempt.
//
// The retry interval will grow by a factor specified in RetryIntervalFactor.
//
// If any of the other retry options are set, this will be set by default to 50 milliseconds.
func WithInitialRetryInterval(initialRetryInterval time.Duration) withInitialRetryInterval {
	return withInitialRetryInterval{initialRetryInterval}
}

type withRetryIntervalFactor struct {
	retryIntervalFactor float32
}

var _ options.RunOption = withRetryIntervalFactor{}

func (w withRetryIntervalFactor) BeforeRun(opts *options.RunOptions) {
	opts.RetryIntervalFactor = &w.retryIntervalFactor
}

// WithRetryIntervalFactor sets the RetryIntervalFactor to use when computing the next retry delay.
//
// If any of the other retry options are set, this will be set by default to 2, meaning retry interval will double at each attempt.
func WithRetryIntervalFactor(retryIntervalFactor float32) withRetryIntervalFactor {
	return withRetryIntervalFactor{retryIntervalFactor}
}

type withMaxRetryInterval struct {
	maxRetryInterval time.Duration
}

var _ options.RunOption = withMaxRetryInterval{}

func (w withMaxRetryInterval) BeforeRun(opts *options.RunOptions) {
	opts.MaxRetryInterval = &w.maxRetryInterval
}

// WithMaxRetryInterval sets the MaxRetryInterval before giving up.
//
// When giving up, Run will return a TerminalError wrapping the original error message.
func WithMaxRetryInterval(maxRetryInterval time.Duration) withMaxRetryInterval {
	return withMaxRetryInterval{maxRetryInterval}
}

type withName struct {
	name string
}

var _ options.RunOption = withName{}

func (w withName) BeforeRun(opts *options.RunOptions) {
	opts.Name = w.name
}

// WithName sets the run name, shown in the UI and other Restate observability tools.
func WithName(name string) withName {
	return withName{name}
}

type withMetadata struct {
	metadata map[string]string
}

var _ options.ServiceDefinitionOption = withMetadata{}
var _ options.HandlerOption = withMetadata{}

func (w withMetadata) BeforeServiceDefinition(opts *options.ServiceDefinitionOptions) {
	if opts.Metadata == nil {
		opts.Metadata = w.metadata
	} else {
		for k, v := range w.metadata {
			opts.Metadata[k] = v
		}
	}
}

func (w withMetadata) BeforeHandler(opts *options.HandlerOptions) {
	for k, v := range w.metadata {
		opts.Metadata[k] = v
	}
}

// WithMetadataMap adds the given map to the metadata of a service/handler shown in the Admin API.
func WithMetadataMap(metadata map[string]string) withMetadata {
	return withMetadata{metadata}
}

// WithMetadata adds the given key/value to the metadata of a service/handler shown in the Admin API.
func WithMetadata(metadataKey string, metadataValue string) withMetadata {
	return withMetadata{map[string]string{metadataKey: metadataValue}}
}
