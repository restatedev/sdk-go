package internal

import "github.com/restatedev/sdk-go/encoding"

type ProtocolMode string

const (
	ProtocolMode_BIDI_STREAM      ProtocolMode = "BIDI_STREAM"
	ProtocolMode_REQUEST_RESPONSE ProtocolMode = "REQUEST_RESPONSE"
)

type ServiceType string

const (
	ServiceType_VIRTUAL_OBJECT ServiceType = "VIRTUAL_OBJECT"
	ServiceType_SERVICE        ServiceType = "SERVICE"
	ServiceType_WORKFLOW       ServiceType = "WORKFLOW"
)

type ServiceHandlerType string

const (
	ServiceHandlerType_WORKFLOW  ServiceHandlerType = "WORKFLOW"
	ServiceHandlerType_EXCLUSIVE ServiceHandlerType = "EXCLUSIVE"
	ServiceHandlerType_SHARED    ServiceHandlerType = "SHARED"
)

type Handler struct {
	// Abort timeout duration, expressed in milliseconds.
	AbortTimeout *int `json:"abortTimeout,omitempty" yaml:"abortTimeout,omitempty" mapstructure:"abortTimeout,omitempty"`

	// Documentation for this handler definition. No format is enforced, but generally
	// Markdown is assumed.
	Documentation *string `json:"documentation,omitempty" yaml:"documentation,omitempty" mapstructure:"documentation,omitempty"`

	// If true, lazy state is enabled.
	EnableLazyState *bool `json:"enableLazyState,omitempty" yaml:"enableLazyState,omitempty" mapstructure:"enableLazyState,omitempty"`

	// Idempotency retention duration, expressed in milliseconds. This is NOT VALID
	// when HandlerType == WORKFLOW
	IdempotencyRetention *int `json:"idempotencyRetention,omitempty" yaml:"idempotencyRetention,omitempty" mapstructure:"idempotencyRetention,omitempty"`

	// Inactivity timeout duration, expressed in milliseconds.
	InactivityTimeout *int `json:"inactivityTimeout,omitempty" yaml:"inactivityTimeout,omitempty" mapstructure:"inactivityTimeout,omitempty"`

	// If true, the service cannot be invoked from the HTTP nor Kafka ingress.
	IngressPrivate *bool `json:"ingressPrivate,omitempty" yaml:"ingressPrivate,omitempty" mapstructure:"ingressPrivate,omitempty"`

	// Retry policy fields (protocol v4+)
	RetryPolicyInitialInterval      *int     `json:"retryPolicyInitialInterval,omitempty" yaml:"retryPolicyInitialInterval,omitempty" mapstructure:"retryPolicyInitialInterval,omitempty"`
	RetryPolicyMaxInterval          *int     `json:"retryPolicyMaxInterval,omitempty" yaml:"retryPolicyMaxInterval,omitempty" mapstructure:"retryPolicyMaxInterval,omitempty"`
	RetryPolicyMaxAttempts          *int     `json:"retryPolicyMaxAttempts,omitempty" yaml:"retryPolicyMaxAttempts,omitempty" mapstructure:"retryPolicyMaxAttempts,omitempty"`
	RetryPolicyExponentiationFactor *float64 `json:"retryPolicyExponentiationFactor,omitempty" yaml:"retryPolicyExponentiationFactor,omitempty" mapstructure:"retryPolicyExponentiationFactor,omitempty"`
	RetryPolicyOnMaxAttempts        *string  `json:"retryPolicyOnMaxAttempts,omitempty" yaml:"retryPolicyOnMaxAttempts,omitempty" mapstructure:"retryPolicyOnMaxAttempts,omitempty"`

	// Description of an input payload. This will be used by Restate to validate
	// incoming requests.
	Input *encoding.InputPayload `json:"input,omitempty" yaml:"input,omitempty" mapstructure:"input,omitempty"`

	// Journal retention duration, expressed in milliseconds.
	JournalRetention *int `json:"journalRetention,omitempty" yaml:"journalRetention,omitempty" mapstructure:"journalRetention,omitempty"`

	// Custom metadata of this handler definition. This metadata is shown on the Admin
	// API when querying the service/handler definition.
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty" mapstructure:"metadata,omitempty"`

	// Name corresponds to the JSON schema field "name".
	Name string `json:"name" yaml:"name" mapstructure:"name"`

	// Description of an output payload.
	Output *encoding.OutputPayload `json:"output,omitempty" yaml:"output,omitempty" mapstructure:"output,omitempty"`

	// If unspecified, defaults to EXCLUSIVE for Virtual Object or WORKFLOW for
	// Workflows. This should be unset for Services.
	Ty *ServiceHandlerType `json:"ty,omitempty" yaml:"ty,omitempty" mapstructure:"ty,omitempty"`

	// Workflow completion retention duration, expressed in milliseconds. This is
	// valid ONLY when HandlerType == WORKFLOW
	WorkflowCompletionRetention *int `json:"workflowCompletionRetention,omitempty" yaml:"workflowCompletionRetention,omitempty" mapstructure:"workflowCompletionRetention,omitempty"`
}

type Service struct {
	// Abort timeout duration, expressed in milliseconds.
	AbortTimeout *int `json:"abortTimeout,omitempty" yaml:"abortTimeout,omitempty" mapstructure:"abortTimeout,omitempty"`

	// Documentation for this service definition. No format is enforced, but generally
	// Markdown is assumed.
	Documentation *string `json:"documentation,omitempty" yaml:"documentation,omitempty" mapstructure:"documentation,omitempty"`

	// If true, lazy state is enabled.
	EnableLazyState *bool `json:"enableLazyState,omitempty" yaml:"enableLazyState,omitempty" mapstructure:"enableLazyState,omitempty"`

	// Handlers corresponds to the JSON schema field "handlers".
	Handlers []Handler `json:"handlers" yaml:"handlers" mapstructure:"handlers"`

	// Idempotency retention duration, expressed in milliseconds. When ServiceType ==
	// WORKFLOW, this option will be applied only to the shared handlers. See
	// workflowCompletionRetention for more details.
	IdempotencyRetention *int `json:"idempotencyRetention,omitempty" yaml:"idempotencyRetention,omitempty" mapstructure:"idempotencyRetention,omitempty"`

	// Inactivity timeout duration, expressed in milliseconds.
	InactivityTimeout *int `json:"inactivityTimeout,omitempty" yaml:"inactivityTimeout,omitempty" mapstructure:"inactivityTimeout,omitempty"`

	// If true, the service cannot be invoked from the HTTP nor Kafka ingress.
	IngressPrivate *bool `json:"ingressPrivate,omitempty" yaml:"ingressPrivate,omitempty" mapstructure:"ingressPrivate,omitempty"`

	// Retry policy fields (protocol v4+)
	RetryPolicyInitialInterval      *int     `json:"retryPolicyInitialInterval,omitempty" yaml:"retryPolicyInitialInterval,omitempty" mapstructure:"retryPolicyInitialInterval,omitempty"`
	RetryPolicyMaxInterval          *int     `json:"retryPolicyMaxInterval,omitempty" yaml:"retryPolicyMaxInterval,omitempty" mapstructure:"retryPolicyMaxInterval,omitempty"`
	RetryPolicyMaxAttempts          *int     `json:"retryPolicyMaxAttempts,omitempty" yaml:"retryPolicyMaxAttempts,omitempty" mapstructure:"retryPolicyMaxAttempts,omitempty"`
	RetryPolicyExponentiationFactor *float64 `json:"retryPolicyExponentiationFactor,omitempty" yaml:"retryPolicyExponentiationFactor,omitempty" mapstructure:"retryPolicyExponentiationFactor,omitempty"`
	RetryPolicyOnMaxAttempts        *string  `json:"retryPolicyOnMaxAttempts,omitempty" yaml:"retryPolicyOnMaxAttempts,omitempty" mapstructure:"retryPolicyOnMaxAttempts,omitempty"`

	// Journal retention duration, expressed in milliseconds.
	JournalRetention *int `json:"journalRetention,omitempty" yaml:"journalRetention,omitempty" mapstructure:"journalRetention,omitempty"`

	// Custom metadata of this service definition. This metadata is shown on the Admin
	// API when querying the service definition.
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty" mapstructure:"metadata,omitempty"`

	// Name corresponds to the JSON schema field "name".
	Name string `json:"name" yaml:"name" mapstructure:"name"`

	// Ty corresponds to the JSON schema field "ty".
	Ty ServiceType `json:"ty" yaml:"ty" mapstructure:"ty"`
}

// Endpoint manifest v3
type Endpoint struct {
	// Maximum supported protocol version
	MaxProtocolVersion int `json:"maxProtocolVersion" yaml:"maxProtocolVersion" mapstructure:"maxProtocolVersion"`

	// Minimum supported protocol version
	MinProtocolVersion int `json:"minProtocolVersion" yaml:"minProtocolVersion" mapstructure:"minProtocolVersion"`

	// ProtocolMode corresponds to the JSON schema field "protocolMode".
	ProtocolMode ProtocolMode `json:"protocolMode,omitempty" yaml:"protocolMode,omitempty" mapstructure:"protocolMode,omitempty"`

	// Services corresponds to the JSON schema field "services".
	Services []Service `json:"services" yaml:"services" mapstructure:"services"`
}
