package restate

import (
	"time"

	"github.com/restatedev/sdk-go/internal/options"
)

// HandlerOption is an option applied to a single handler at registration.
type HandlerOption = options.HandlerOption

// ServiceDefinitionOption is an option applied to a whole service/object/workflow at registration.
type ServiceDefinitionOption = options.ServiceDefinitionOption

// Retry policy types
type InvocationRetryPolicy = options.InvocationRetryPolicy
type OnMaxAttempts = options.OnMaxAttempts

// InvocationRetryPolicyOption configures an [InvocationRetryPolicy].
type InvocationRetryPolicyOption = options.InvocationRetryPolicyOption

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
