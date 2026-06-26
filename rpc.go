package restate

import (
	"time"

	"github.com/restatedev/sdk-go/internal/genericfutures"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/restatecontext"
)

// ClientOption is an option for a request/send client, applied at construction
// (e.g. via [Service], [Object] or [Workflow]).
type ClientOption = options.ClientOption

// RequestOption is an option for a [Client.Request] or [Client.RequestFuture] call.
type RequestOption = options.RequestOption

// SendOption is an option for a [SendClient.Send] call.
type SendOption = options.SendOption

// AttachOption is an option for [AttachInvocation].
type AttachOption = options.AttachOption

// Service gets a Service request client by service and method name
func Service[O any](ctx Context, service string, method string, options ...options.ClientOption) Client[any, O] {
	return outputClient[O]{ctx.inner().Service(service, method, options...)}
}

// ServiceSend gets a Service send client by service and method name
func ServiceSend(ctx Context, service string, method string, options ...options.ClientOption) SendClient[any] {
	return ctx.inner().Service(service, method, options...)
}

// Object gets an Object request client by service name, key and method name
func Object[O any](ctx Context, service string, key string, method string, options ...options.ClientOption) Client[any, O] {
	return outputClient[O]{ctx.inner().Object(service, key, method, options...)}
}

// ObjectSend gets an Object send client by service name, key and method name
func ObjectSend(ctx Context, service string, key string, method string, options ...options.ClientOption) SendClient[any] {
	return ctx.inner().Object(service, key, method, options...)
}

// Workflow gets a Workflow request client by service name, workflow ID and method name
func Workflow[O any](ctx Context, service string, workflowID string, method string, options ...options.ClientOption) Client[any, O] {
	return outputClient[O]{ctx.inner().Workflow(service, workflowID, method, options...)}
}

// WorkflowSend gets a Workflow send client by service name, workflow ID and method name
func WorkflowSend(ctx Context, service string, workflowID string, method string, options ...options.ClientOption) SendClient[any] {
	return ctx.inner().Workflow(service, workflowID, method, options...)
}

// Client represents all the different ways you can invoke a particular service-method.
type Client[I any, O any] interface {
	// RequestFuture makes a call and returns a handle on a future response
	RequestFuture(input I, options ...options.RequestOption) ResponseFuture[O]
	// Request makes a call and blocks on getting the response
	Request(input I, options ...options.RequestOption) (O, TerminalError)
	SendClient[I]
}

// SendClient allows making one-way invocations
type SendClient[I any] interface {
	// Send makes a one-way call which is executed in the background
	Send(input I, options ...options.SendOption) Invocation
}

type outputClient[O any] struct {
	inner restatecontext.Client
}

func (t outputClient[O]) Request(input any, options ...options.RequestOption) (output O, err TerminalError) {
	err = t.inner.Request(input, &output, options...)
	return
}

func (t outputClient[O]) RequestFuture(input any, options ...options.RequestOption) ResponseFuture[O] {
	return genericfutures.ResponseFuture[O]{ResponseFuture: t.inner.RequestFuture(input, options...)}
}

func (t outputClient[O]) Send(input any, options ...options.SendOption) Invocation {
	return t.inner.Send(input, options...)
}

type client[I any, O any] struct {
	inner Client[any, O]
}

// WithRequestType is primarily intended to be called from generated code, to provide
// type safety of input types. In other contexts it's generally less cumbersome to use [Object] and [Service],
// as the output type can be inferred.
func WithRequestType[I any, O any](inner Client[any, O]) Client[I, O] {
	return client[I, O]{inner}
}

func (t client[I, O]) Request(input I, options ...options.RequestOption) (output O, err TerminalError) {
	output, err = t.inner.RequestFuture(input, options...).Response()
	return
}

func (t client[I, O]) RequestFuture(input I, options ...options.RequestOption) ResponseFuture[O] {
	return t.inner.RequestFuture(input, options...)
}

func (t client[I, O]) Send(input I, options ...options.SendOption) Invocation {
	return t.inner.Send(input, options...)
}

// ResponseFuture is a handle on a potentially not-yet completed outbound call.
type ResponseFuture[O any] interface {
	// Response blocks on the response to the call and returns it or the associated error
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Response() (O, TerminalError)
	Invocation
	restatecontext.Future
}

type Invocation = restatecontext.Invocation

// CancelInvocation cancels the invocation with the given invocationId.
// For more info about cancellations, see https://docs.restate.dev/operate/invocation/#cancelling-invocations
func CancelInvocation(ctx Context, invocationId string) {
	ctx.inner().CancelInvocation(invocationId)
}

// AttachFuture is a handle on a potentially not-yet completed call.
type AttachFuture[O any] interface {
	// Response blocks on the response to the call and returns it or the associated error
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Response() (O, TerminalError)
	restatecontext.Future
}

// AttachInvocation attaches to the invocation with the given invocation id.
func AttachInvocation[T any](ctx Context, invocationId string, options ...options.AttachOption) AttachFuture[T] {
	return genericfutures.AttachFuture[T]{AttachFuture: ctx.inner().AttachInvocation(invocationId, options...)}
}

type withScope struct {
	scope string
}

var _ options.ClientOption = withScope{}

func (w withScope) BeforeClient(opts *options.ClientOptions) {
	opts.Scope = w.scope
}

// WithScope sets the scope within which invocations made through this client are routed.
//
// It is a client-level option: pass it when constructing a client (e.g. via [Service],
// [Object] or [Workflow], or the equivalent ingress constructors) and it applies to
// every Request, RequestFuture and Send made through that client. An empty scope is a
// no-op, leaving the invocation unscoped.
func WithScope(scope string) withScope {
	return withScope{scope: scope}
}

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

type withLimitKey struct {
	limitKey string
}

var _ options.RequestOption = withLimitKey{}
var _ options.SendOption = withLimitKey{}
var _ options.IngressRequestOption = withLimitKey{}
var _ options.IngressSendOption = withLimitKey{}

func (w withLimitKey) BeforeRequest(opts *options.RequestOptions) {
	opts.LimitKey = w.limitKey
}

func (w withLimitKey) BeforeSend(opts *options.SendOptions) {
	opts.LimitKey = w.limitKey
}

func (w withLimitKey) BeforeIngressRequest(opts *options.IngressRequestOptions) {
	opts.LimitKey = w.limitKey
}

func (w withLimitKey) BeforeIngressSend(opts *options.IngressSendOptions) {
	opts.LimitKey = w.limitKey
}

// WithLimitKey sets the concurrency limit key when making a call.
func WithLimitKey(limitKey string) withLimitKey {
	return withLimitKey{limitKey}
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
