package options

import (
	"net/http"
	"time"

	"github.com/restatedev/sdk-go/encoding"
)

// OnMaxAttempts determines behavior when max attempts is reached.
type OnMaxAttempts string

const (
	OnMaxAttemptsPause OnMaxAttempts = "PAUSE"
	OnMaxAttemptsKill  OnMaxAttempts = "KILL"
)

// InvocationRetryPolicy exposed in discovery manifest (protocol v4+)
// Unset fields inherit server defaults.
type InvocationRetryPolicy struct {
	InitialInterval      *time.Duration
	ExponentiationFactor *float64
	MaxInterval          *time.Duration
	MaxAttempts          *int
	OnMaxAttempts        *OnMaxAttempts
}

// InvocationRetryPolicyOption configures fields of InvocationRetryPolicy.
type InvocationRetryPolicyOption interface {
	BeforeRetryPolicy(*InvocationRetryPolicy)
}

// all options interfaces should be re-exported in the top-level options.go

type SleepOptions struct {
	// Name used for observability.
	Name string
}

type SleepOption interface {
	BeforeSleep(*SleepOptions)
}

type AwakeableOptions struct {
	Codec encoding.Codec
}

type AwakeableOption interface {
	BeforeAwakeable(*AwakeableOptions)
}

type SignalOptions struct {
	Codec encoding.Codec
}

type SignalOption interface {
	BeforeSignal(*SignalOptions)
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

type ResolveSignalOptions struct {
	Codec encoding.Codec
}

type ResolveSignalOption interface {
	BeforeResolveSignal(*ResolveSignalOptions)
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
	Scope string
}

type ClientOption interface {
	BeforeClient(*ClientOptions)
}

type RequestOptions struct {
	IdempotencyKey string
	Headers        map[string]string
	Scope          string
	LimitKey       string
}

type RequestOption interface {
	BeforeRequest(*RequestOptions)
}

type IngressRequestOptions struct {
	RequestOptions
	Codec encoding.PayloadCodec
}

type IngressRequestOption interface {
	BeforeIngressRequest(*IngressRequestOptions)
}

type SendOptions struct {
	IdempotencyKey string
	Headers        map[string]string
	Delay          time.Duration
	Scope          string
	LimitKey       string
}

type SendOption interface {
	BeforeSend(*SendOptions)
}

type IngressSendOptions struct {
	SendOptions
	Codec encoding.PayloadCodec
}

type IngressSendOption interface {
	BeforeIngressSend(*IngressSendOptions)
}

type IngressInvocationHandleOptions struct {
	Codec encoding.PayloadCodec
}

type IngressInvocationHandleOption interface {
	BeforeIngressInvocationHandle(*IngressInvocationHandleOptions)
}

type RunOptions struct {
	// MaxRetryAttempts (including the initial attempt) before giving up.
	//
	// When giving up, Run will return a TerminalError wrapping the original error message.
	MaxRetryAttempts *uint

	// MaxRetryDuration before giving up.
	//
	// When giving up, Run will return a TerminalError wrapping the original error message.
	MaxRetryDuration *time.Duration

	// InitialRetryInterval for the first retry attempt.
	//
	// The retry interval will grow by a factor specified in RetryIntervalFactor.
	//
	// If any of the other retry options are set, this will be set by default to 50 milliseconds.
	InitialRetryInterval *time.Duration

	// RetryIntervalFactor to use when computing the next retry delay.
	//
	// If any of the other retry options are set, this will be set by default to 2, meaning retry interval will double at each attempt.
	RetryIntervalFactor *float32

	// MaxRetryInterval between retries.
	// Retry interval will grow by a factor specified in RetryIntervalFactor up to the interval specified in this value.
	//
	// If any of the other retry options are set, this will be set by default to 2 seconds.
	MaxRetryInterval *time.Duration

	// Name used for observability.
	Name string

	// Codec used to encode/decode the run result.
	Codec encoding.Codec
}

type RunOption interface {
	BeforeRun(*RunOptions)
}

type AttachOptions struct {
	Codec encoding.Codec
}

type AttachOption interface {
	BeforeAttach(*AttachOptions)
}

type HandlerOptions struct {
	Codec                 encoding.PayloadCodec
	Metadata              map[string]string
	Documentation         string
	AbortTimeout          *time.Duration
	EnableLazyState       *bool
	IdempotencyRetention  *time.Duration
	InactivityTimeout     *time.Duration
	IngressPrivate        *bool
	JournalRetention      *time.Duration
	WorkflowRetention     *time.Duration
	InvocationRetryPolicy *InvocationRetryPolicy
}

type HandlerOption interface {
	BeforeHandler(*HandlerOptions)
}

type ServiceDefinitionOptions struct {
	DefaultCodec          encoding.PayloadCodec
	Metadata              map[string]string
	Documentation         string
	AbortTimeout          *time.Duration
	EnableLazyState       *bool
	IdempotencyRetention  *time.Duration
	WorkflowRetention     *time.Duration
	InactivityTimeout     *time.Duration
	IngressPrivate        *bool
	JournalRetention      *time.Duration
	InvocationRetryPolicy *InvocationRetryPolicy
}

type ServiceDefinitionOption interface {
	BeforeServiceDefinition(*ServiceDefinitionOptions)
}

type IngressClientOptions struct {
	HttpClient *http.Client
	AuthKey    string
	Codec      encoding.PayloadCodec
}

type IngressClientOption interface {
	BeforeIngress(*IngressClientOptions)
}
