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
	IdempotencyKey string
	Headers        map[string]string
}

type RequestOption interface {
	BeforeRequest(*RequestOptions)
}

type SendOptions struct {
	IdempotencyKey string
	Headers        map[string]string
	Delay          time.Duration
}

type SendOption interface {
	BeforeSend(*SendOptions)
}

type RunOptions struct {
	// MaxRetryAttempts before giving up.
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

	// Name used for observability
	Name string

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
