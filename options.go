package restate

import (
	"net/http"
	"time"

	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal/options"
)

// re-export options types so users can create arrays of them and functions that accept/return them
type SleepOption = options.SleepOption
type AwakeableOption = options.AwakeableOption
type PromiseOption = options.PromiseOption
type ResolveAwakeableOption = options.ResolveAwakeableOption
type GetOption = options.GetOption
type SetOption = options.SetOption
type ClientOption = options.ClientOption
type RequestOption = options.RequestOption
type IngressRequestOption = options.IngressRequestOption
type SendOption = options.SendOption
type IngressSendOption = options.IngressSendOption
type RunOption = options.RunOption
type AttachOption = options.AttachOption
type HandlerOption = options.HandlerOption
type ServiceDefinitionOption = options.ServiceDefinitionOption

// Retry policy types
type InvocationRetryPolicy = options.InvocationRetryPolicy
type OnMaxAttempts = options.OnMaxAttempts

// Retry policy option builders
type InvocationRetryPolicyOption = options.InvocationRetryPolicyOption
type IngressClientOption = options.IngressClientOption

type withCodec struct {
	codec encoding.Codec
}

var _ options.GetOption = withCodec{}
var _ options.SetOption = withCodec{}
var _ options.RunOption = withCodec{}
var _ options.AwakeableOption = withCodec{}
var _ options.PromiseOption = withCodec{}
var _ options.ResolveAwakeableOption = withCodec{}
var _ options.ClientOption = withCodec{}
var _ options.AttachOption = withCodec{}

func (w withCodec) BeforeGet(opts *options.GetOptions)             { opts.Codec = w.codec }
func (w withCodec) BeforeSet(opts *options.SetOptions)             { opts.Codec = w.codec }
func (w withCodec) BeforeRun(opts *options.RunOptions)             { opts.Codec = w.codec }
func (w withCodec) BeforeAwakeable(opts *options.AwakeableOptions) { opts.Codec = w.codec }
func (w withCodec) BeforePromise(opts *options.PromiseOptions)     { opts.Codec = w.codec }
func (w withCodec) BeforeResolveAwakeable(opts *options.ResolveAwakeableOptions) {
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

type withHeaders struct {
	headers map[string]string
}

var _ options.RequestOption = withHeaders{}
var _ options.SendOption = withHeaders{}
var _ options.IngressRequestOption = withHeaders{}
var _ options.IngressSendOption = withHeaders{}

func (w withHeaders) BeforeRequest(opts *options.RequestOptions) {
	opts.Headers = w.headers
}

func (w withHeaders) BeforeSend(opts *options.SendOptions) {
	opts.Headers = w.headers
}

func (w withHeaders) BeforeIngressRequest(opts *options.IngressRequestOptions) {
	opts.Headers = w.headers
}

func (w withHeaders) BeforeIngressSend(opts *options.IngressSendOptions) {
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
var _ options.IngressRequestOption = withIdempotencyKey{}
var _ options.IngressSendOption = withIdempotencyKey{}

func (w withIdempotencyKey) BeforeRequest(opts *options.RequestOptions) {
	opts.IdempotencyKey = w.idempotencyKey
}

func (w withIdempotencyKey) BeforeSend(opts *options.SendOptions) {
	opts.IdempotencyKey = w.idempotencyKey
}

func (w withIdempotencyKey) BeforeIngressRequest(opts *options.IngressRequestOptions) {
	opts.IdempotencyKey = w.idempotencyKey
}

func (w withIdempotencyKey) BeforeIngressSend(opts *options.IngressSendOptions) {
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
var _ options.IngressSendOption = withDelay{}

func (w withDelay) BeforeSend(opts *options.SendOptions) {
	opts.Delay = w.delay
}

func (w withDelay) BeforeIngressSend(opts *options.IngressSendOptions) {
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

// WithMaxRetryAttempts sets the MaxRetryAttempts (including the initial attempt) before giving up.
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
var _ options.SleepOption = withName{}

func (w withName) BeforeRun(opts *options.RunOptions) {
	opts.Name = w.name
}

func (w withName) BeforeSleep(opts *options.SleepOptions) {
	opts.Name = w.name
}

// WithName sets the operation name, shown in the UI and other Restate observability tools.
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

type withDocumentation struct {
	documentation string
}

var _ options.ServiceDefinitionOption = withDocumentation{}
var _ options.HandlerOption = withDocumentation{}

func (w withDocumentation) BeforeServiceDefinition(opts *options.ServiceDefinitionOptions) {
	opts.Documentation = w.documentation
}

func (w withDocumentation) BeforeHandler(opts *options.HandlerOptions) {
	opts.Documentation = w.documentation
}

// WithDocumentation sets the handler/service documentation, shown in the UI and other Restate observability tools.
func WithDocumentation(documentation string) withDocumentation {
	return withDocumentation{documentation}
}

type withAbortTimeout struct {
	abortTimeout time.Duration
}

var _ options.ServiceDefinitionOption = withAbortTimeout{}
var _ options.HandlerOption = withAbortTimeout{}

func (w withAbortTimeout) BeforeServiceDefinition(opts *options.ServiceDefinitionOptions) {
	opts.AbortTimeout = &w.abortTimeout
}

func (w withAbortTimeout) BeforeHandler(opts *options.HandlerOptions) {
	opts.AbortTimeout = &w.abortTimeout
}

// WithAbortTimeout sets the abort timeout duration for a service/handler.
//
// This timer guards against stalled service/handler invocations that are supposed to terminate. The
// abort timeout is started after the inactivity timeout has expired and the service/handler
// invocation has been asked to gracefully terminate. Once the timer expires, it will abort the
// service/handler invocation.
//
// This timer potentially *interrupts* user code. If the user code needs longer to gracefully
// terminate, then this value needs to be set accordingly.
//
// This overrides the default abort timeout configured in the restate-server for all invocations to
// this service.
//
// NOTE: You can set this field only if you register this service against restate-server >= 1.4,
// otherwise the service discovery will fail.
func WithAbortTimeout(abortTimeout time.Duration) withAbortTimeout {
	return withAbortTimeout{abortTimeout}
}

type withEnableLazyState struct {
	enableLazyState bool
}

var _ options.ServiceDefinitionOption = withEnableLazyState{}
var _ options.HandlerOption = withEnableLazyState{}

func (w withEnableLazyState) BeforeServiceDefinition(opts *options.ServiceDefinitionOptions) {
	opts.EnableLazyState = &w.enableLazyState
}

func (w withEnableLazyState) BeforeHandler(opts *options.HandlerOptions) {
	opts.EnableLazyState = &w.enableLazyState
}

// WithEnableLazyState enables or disables lazy state for a service/handler.
//
// When set to true, lazy state will be enabled for all invocations to this service/handler. This is
// relevant only for workflows and virtual objects.
//
// NOTE: You can set this field only if you register this service against restate-server >= 1.4,
// otherwise the service discovery will fail.
func WithEnableLazyState(enableLazyState bool) withEnableLazyState {
	return withEnableLazyState{enableLazyState}
}

type withIdempotencyRetention struct {
	idempotencyRetention time.Duration
}

var _ options.ServiceDefinitionOption = withIdempotencyRetention{}
var _ options.HandlerOption = withIdempotencyRetention{}

func (w withIdempotencyRetention) BeforeServiceDefinition(opts *options.ServiceDefinitionOptions) {
	opts.IdempotencyRetention = &w.idempotencyRetention
}

func (w withIdempotencyRetention) BeforeHandler(opts *options.HandlerOptions) {
	opts.IdempotencyRetention = &w.idempotencyRetention
}

// WithIdempotencyRetention sets the idempotency retention duration for a service/handler.
//
// NOTE: You can set this field only if you register this service against restate-server >= 1.4,
// otherwise the service discovery will fail.
func WithIdempotencyRetention(idempotencyRetention time.Duration) withIdempotencyRetention {
	return withIdempotencyRetention{idempotencyRetention}
}

type withInactivityTimeout struct {
	inactivityTimeout time.Duration
}

var _ options.ServiceDefinitionOption = withInactivityTimeout{}
var _ options.HandlerOption = withInactivityTimeout{}

func (w withInactivityTimeout) BeforeServiceDefinition(opts *options.ServiceDefinitionOptions) {
	opts.InactivityTimeout = &w.inactivityTimeout
}

func (w withInactivityTimeout) BeforeHandler(opts *options.HandlerOptions) {
	opts.InactivityTimeout = &w.inactivityTimeout
}

// WithInactivityTimeout sets the inactivity timeout duration for a service/handler.
//
// This timer guards against stalled invocations. Once it expires, Restate triggers a graceful
// termination by asking the invocation to suspend (which preserves intermediate progress).
//
// The abort timeout is used to abort the invocation, in case it doesn't react to the request to
// suspend.
//
// This overrides the default inactivity timeout configured in the restate-server for all
// invocations to this service.
//
// NOTE: You can set this field only if you register this service against restate-server >= 1.4,
// otherwise the service discovery will fail.
func WithInactivityTimeout(inactivityTimeout time.Duration) withInactivityTimeout {
	return withInactivityTimeout{inactivityTimeout}
}

type withIngressPrivate struct {
	ingressPrivate bool
}

var _ options.ServiceDefinitionOption = withIngressPrivate{}
var _ options.HandlerOption = withIngressPrivate{}

func (w withIngressPrivate) BeforeServiceDefinition(opts *options.ServiceDefinitionOptions) {
	opts.IngressPrivate = &w.ingressPrivate
}

func (w withIngressPrivate) BeforeHandler(opts *options.HandlerOptions) {
	opts.IngressPrivate = &w.ingressPrivate
}

// WithIngressPrivate sets whether the service/handler is private (not accessible from HTTP or Kafka ingress).
//
// When set to true this service/handler cannot be invoked from the restate-server
// HTTP and Kafka ingress, but only from other services.
//
// NOTE: You can set this field only if you register this service against restate-server >= 1.4,
// otherwise the service discovery will fail.
func WithIngressPrivate(ingressPrivate bool) withIngressPrivate {
	return withIngressPrivate{ingressPrivate}
}

type withJournalRetention struct {
	journalRetention time.Duration
}

var _ options.ServiceDefinitionOption = withJournalRetention{}
var _ options.HandlerOption = withJournalRetention{}

func (w withJournalRetention) BeforeServiceDefinition(opts *options.ServiceDefinitionOptions) {
	opts.JournalRetention = &w.journalRetention
}

func (w withJournalRetention) BeforeHandler(opts *options.HandlerOptions) {
	opts.JournalRetention = &w.journalRetention
}

// WithJournalRetention sets the journal retention duration for a service/handler.
//
// The journal retention for invocations to this service/handler.
//
// In case the request has an idempotency key, the idempotency retention caps the journal retention
// time.
//
// NOTE: You can set this field only if you register this service against restate-server >= 1.4,
// otherwise the service discovery will fail.
func WithJournalRetention(journalRetention time.Duration) withJournalRetention {
	return withJournalRetention{journalRetention}
}

type withWorkflowRetention struct {
	workflowRetention time.Duration
}

var _ options.ServiceDefinitionOption = withWorkflowRetention{}
var _ options.HandlerOption = withWorkflowRetention{}

func (w withWorkflowRetention) BeforeServiceDefinition(opts *options.ServiceDefinitionOptions) {
	opts.WorkflowRetention = &w.workflowRetention
}

func (w withWorkflowRetention) BeforeHandler(opts *options.HandlerOptions) {
	opts.WorkflowRetention = &w.workflowRetention
}

// WithWorkflowRetention sets the workflow completion retention duration for a handler.
//
// The retention duration for this workflow handler.
//
// This is only valid when HandlerType == WORKFLOW.
//
// NOTE: You can set this field only if you register this service against restate-server >= 1.4,
// otherwise the service discovery will fail.
func WithWorkflowRetention(workflowCompletionRetention time.Duration) withWorkflowRetention {
	return withWorkflowRetention{workflowCompletionRetention}
}

type withInvocationRetryPolicy struct {
	policy options.InvocationRetryPolicy
}

var _ options.ServiceDefinitionOption = withInvocationRetryPolicy{}
var _ options.HandlerOption = withInvocationRetryPolicy{}

func (w withInvocationRetryPolicy) BeforeServiceDefinition(opts *options.ServiceDefinitionOptions) {
	opts.InvocationRetryPolicy = &w.policy
}

func (w withInvocationRetryPolicy) BeforeHandler(opts *options.HandlerOptions) {
	opts.InvocationRetryPolicy = &w.policy
}

// WithInvocationRetryPolicy sets the invocation retry policy used by Restate when invoking this service/handler.
//
// NOTE: You can set this field only if you register this service against restate-server >= 1.5,
// otherwise the service discovery will fail.
//
// Unset fields inherit server defaults. The policy controls an exponential backoff with optional capping and a terminal action:
//   - initial interval before the first retry attempt
//   - exponentiation factor to compute the next retry delay
//   - maximum interval cap
//   - maximum attempts (initial call counts as the first attempt)
//   - behavior when max attempts is reached (OnMaxAttempts: PAUSE | KILL)
func WithInvocationRetryPolicy(opts ...InvocationRetryPolicyOption) withInvocationRetryPolicy {
	p := options.InvocationRetryPolicy{}
	for _, o := range opts {
		if o != nil {
			o.BeforeRetryPolicy(&p)
		}
	}
	return withInvocationRetryPolicy{policy: p}
}

// WithInitialInterval sets the initial delay before the first retry attempt. If unset, server defaults apply.
func WithInitialInterval(d time.Duration) options.InvocationRetryPolicyOption {
	return options.InvokeRetryWithInitialInterval(d)
}

// WithExponentiationFactor sets the exponential backoff multiplier used to compute the next retry delay.
func WithExponentiationFactor(f float64) options.InvocationRetryPolicyOption {
	return options.InvokeRetryWithExponentiationFactor(f)
}

// WithMaxInterval sets the upper bound for any computed retry delay.
func WithMaxInterval(d time.Duration) options.InvocationRetryPolicyOption {
	return options.InvokeRetryWithMaxInterval(d)
}

// WithMaxAttempts sets the maximum number of attempts before giving up retrying. The initial call counts as the first attempt.
func WithMaxAttempts(n int) options.InvocationRetryPolicyOption {
	return options.InvokeRetryWithMaxAttempts(n)
}

// PauseOnMaxAttempts sets the behavior to pause when reaching max attempts.
func PauseOnMaxAttempts() options.InvocationRetryPolicyOption {
	return options.InvokeRetryWithOnMaxAttempts(options.OnMaxAttemptsPause)
}

// KillOnMaxAttempts sets the behavior to kill when reaching max attempts.
func KillOnMaxAttempts() options.InvocationRetryPolicyOption {
	return options.InvokeRetryWithOnMaxAttempts(options.OnMaxAttemptsKill)
}

func WithHttpClient(c *http.Client) withHttpClient {
	return withHttpClient{c}
}

type withHttpClient struct {
	httpClient *http.Client
}

func (w withHttpClient) BeforeIngress(opts *options.IngressClientOptions) {
	opts.HttpClient = w.httpClient
}

func WithAuthKey(authKey string) withAuthKey {
	return withAuthKey{authKey}
}

type withAuthKey struct {
	authKey string
}

func (w withAuthKey) BeforeIngress(opts *options.IngressClientOptions) {
	opts.AuthKey = w.authKey
}
