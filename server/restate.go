package server

import (
	"cmp"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"slices"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/internal"
	restateerrors "github.com/restatedev/sdk-go/internal/errors"
	"github.com/restatedev/sdk-go/internal/identity"
	"github.com/restatedev/sdk-go/internal/log"
	"go.opentelemetry.io/otel/propagation"
)

// retryableCode is the HTTP status to report for a non-terminal error from the state
// machine, defaulting to 500 when the error carries no code.
func retryableCode(err error) int {
	if re := restateerrors.AsRetryableError(err); re != nil {
		return int(re.Code())
	}
	return http.StatusInternalServerError
}

type ServiceProtocolVersion int32
type ServiceDiscoveryProtocolVersion int32

const (
	ServiceDiscoveryProtocolVersion_UNKNOWN ServiceDiscoveryProtocolVersion = 0
	ServiceDiscoveryProtocolVersion_V1      ServiceDiscoveryProtocolVersion = 1
	ServiceDiscoveryProtocolVersion_V2      ServiceDiscoveryProtocolVersion = 2
	ServiceDiscoveryProtocolVersion_V3      ServiceDiscoveryProtocolVersion = 3
	ServiceDiscoveryProtocolVersion_V4      ServiceDiscoveryProtocolVersion = 4
	minServiceDiscoveryProtocolVersion                                      = ServiceDiscoveryProtocolVersion_V2
	maxServiceDiscoveryProtocolVersion                                      = ServiceDiscoveryProtocolVersion_V4
	minServiceProtocolVersion                                               = 5
	maxServiceProtocolVersion                                               = 7
)

var xRestateServer = `restate-sdk-go/unknown`

var tracePropagator = propagation.TraceContext{}

func init() {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	for _, dep := range bi.Deps {
		if dep.Path == "github.com/restatedev/sdk-go" {
			xRestateServer = "restate-sdk-go/" + dep.Version
			break
		}
	}
}

// Restate represents a Restate HTTP handler to which services or virtual objects may be attached.
type Restate struct {
	logHandler     slog.Handler
	dropReplayLogs bool
	systemLog      *slog.Logger
	definitions    map[string]restate.ServiceDefinition
	keyIDs         []string
	keySet         identity.KeySetV1
	protocolMode   internal.ProtocolMode
}

// NewRestate creates a new instance of Restate server
func NewRestate() *Restate {
	handler := slog.Default().Handler()
	return &Restate{
		logHandler:     handler,
		systemLog:      slog.New(log.NewRestateContextHandler(handler)),
		dropReplayLogs: true,
		definitions:    make(map[string]restate.ServiceDefinition),
		protocolMode:   internal.ProtocolMode_BIDI_STREAM,
	}
}

// WithLogger overrides the slog handler used by the SDK (which defaults to the slog Default())
// You may specify with dropReplayLogs whether to drop logs that originated from handler code
// while the invocation was replaying. If they are not dropped, you may still determine the replay
// status in a slog.Handler using [github.com/restatedev/sdk-go/logging.LogContextFrom]
func (r *Restate) WithLogger(h slog.Handler, dropReplayLogs bool) *Restate {
	r.dropReplayLogs = dropReplayLogs
	r.systemLog = slog.New(log.NewRestateContextHandler(h))
	r.logHandler = h
	return r
}

// WithIdentityV1 attaches v1 request identity public keys to this server. All incoming requests will be validated
// against one of these keys.
func (r *Restate) WithIdentityV1(keys ...string) *Restate {
	r.keyIDs = append(r.keyIDs, keys...)
	return r
}

// Bidirectional is used to change the protocol mode advertised to Restate on discovery
// In bidirectional mode, Restate will keep the request body open even after we have started to respond,
// allowing for more work to be done without suspending.
// This is supported over HTTP2 and, in some cases (where there is no buffering proxy), with HTTP1.1.
// When serving over a non-bidirectional channel (eg, Cloudflare Workers), use .Bidirectional(false) otherwise your handlers may get stuck.
func (r *Restate) Bidirectional(bidi bool) *Restate {
	if bidi {
		r.protocolMode = internal.ProtocolMode_BIDI_STREAM
	} else {
		r.protocolMode = internal.ProtocolMode_REQUEST_RESPONSE
	}
	return r
}

// Bind attaches a Service Definition (a Service or Virtual Object) to this server
func (r *Restate) Bind(definition restate.ServiceDefinition) *Restate {
	if _, ok := r.definitions[definition.Name()]; ok {
		// panic because this is a programming error
		// to register multiple definitions with the same name
		panic("service definition with the same name exists")
	}

	r.definitions[definition.Name()] = definition

	return r
}

func (r *Restate) discover(protocolVersion ServiceDiscoveryProtocolVersion) (resource *internal.Endpoint, err error) {
	resource = &internal.Endpoint{
		ProtocolMode:       r.protocolMode,
		MinProtocolVersion: minServiceProtocolVersion,
		MaxProtocolVersion: maxServiceProtocolVersion,
		Services:           make([]internal.Service, 0, len(r.definitions)),
	}

	for serviceName, definition := range r.definitions {
		var metadata map[string]string
		var documentation string
		var abortTimeout *int
		var enableLazyState *bool
		var idempotencyRetention *int
		var inactivityTimeout *int
		var ingressPrivate *bool
		var journalRetention *int
		var retryPolicyInitialInterval, retryPolicyMaxInterval, retryPolicyMaxAttempts *int
		var retryPolicyFactor *float64
		var retryPolicyOnMaxAttempts *string

		// Check if there's options to apply
		if definition.GetOptions() != nil {
			// Validate which fields are set
			if protocolVersion < ServiceDiscoveryProtocolVersion_V3 {
				if definition.GetOptions().AbortTimeout != nil {
					return nil, fmt.Errorf("service %s: AbortTimeout option requires discovery protocol version 3 or higher", serviceName)
				}
				if definition.GetOptions().EnableLazyState != nil {
					return nil, fmt.Errorf("service %s: EnableLazyState option requires discovery protocol version 3 or higher", serviceName)
				}
				if definition.GetOptions().IdempotencyRetention != nil {
					return nil, fmt.Errorf("service %s: IdempotencyRetention option requires discovery protocol version 3 or higher", serviceName)
				}
				if definition.GetOptions().InactivityTimeout != nil {
					return nil, fmt.Errorf("service %s: InactivityTimeout option requires discovery protocol version 3 or higher", serviceName)
				}
				if definition.GetOptions().IngressPrivate != nil {
					return nil, fmt.Errorf("service %s: IngressPrivate option requires discovery protocol version 3 or higher", serviceName)
				}
				if definition.GetOptions().JournalRetention != nil {
					return nil, fmt.Errorf("service %s: JournalRetention option requires discovery protocol version 3 or higher", serviceName)
				}
			}
			if protocolVersion < ServiceDiscoveryProtocolVersion_V4 {
				if definition.GetOptions().InvocationRetryPolicy != nil {
					return nil, fmt.Errorf("service %s: InvocationRetryPolicy option requires discovery protocol version 4 or higher", serviceName)
				}
			}

			metadata = definition.GetOptions().Metadata
			documentation = definition.GetOptions().Documentation
			if definition.GetOptions().AbortTimeout != nil {
				abortTimeoutMs := int(definition.GetOptions().AbortTimeout.Milliseconds())
				abortTimeout = &abortTimeoutMs
			}
			enableLazyState = definition.GetOptions().EnableLazyState
			if definition.GetOptions().IdempotencyRetention != nil {
				idempotencyRetentionMs := int(definition.GetOptions().IdempotencyRetention.Milliseconds())
				idempotencyRetention = &idempotencyRetentionMs
			}
			if definition.GetOptions().InactivityTimeout != nil {
				inactivityTimeoutMs := int(definition.GetOptions().InactivityTimeout.Milliseconds())
				inactivityTimeout = &inactivityTimeoutMs
			}
			ingressPrivate = definition.GetOptions().IngressPrivate
			if definition.GetOptions().JournalRetention != nil {
				journalRetentionMs := int(definition.GetOptions().JournalRetention.Milliseconds())
				journalRetention = &journalRetentionMs
			}
			if definition.GetOptions().InvocationRetryPolicy != nil {
				p := definition.GetOptions().InvocationRetryPolicy
				if p.InitialInterval != nil {
					v := int(p.InitialInterval.Milliseconds())
					retryPolicyInitialInterval = &v
				}
				if p.MaxInterval != nil {
					v := int(p.MaxInterval.Milliseconds())
					retryPolicyMaxInterval = &v
				}
				if p.ExponentiationFactor != nil {
					v := float64(*p.ExponentiationFactor)
					retryPolicyFactor = &v
				}
				if p.MaxAttempts != nil {
					v := int(*p.MaxAttempts)
					retryPolicyMaxAttempts = &v
				}
				if p.OnMaxAttempts != nil {
					v := string(*p.OnMaxAttempts)
					retryPolicyOnMaxAttempts = &v
				}
			}
		}
		service := internal.Service{
			Name:                            serviceName,
			Ty:                              definition.Type(),
			Handlers:                        make([]internal.Handler, 0, len(definition.Handlers())),
			Metadata:                        metadata,
			Documentation:                   &documentation,
			AbortTimeout:                    abortTimeout,
			EnableLazyState:                 enableLazyState,
			IdempotencyRetention:            idempotencyRetention,
			InactivityTimeout:               inactivityTimeout,
			IngressPrivate:                  ingressPrivate,
			RetryPolicyInitialInterval:      retryPolicyInitialInterval,
			RetryPolicyMaxInterval:          retryPolicyMaxInterval,
			RetryPolicyMaxAttempts:          retryPolicyMaxAttempts,
			RetryPolicyExponentiationFactor: retryPolicyFactor,
			RetryPolicyOnMaxAttempts:        retryPolicyOnMaxAttempts,
			JournalRetention:                journalRetention,
		}

		for handlerName, handler := range definition.Handlers() {
			var metadata map[string]string
			var documentation string
			var abortTimeout *int
			var enableLazyState *bool
			var idempotencyRetention *int
			var inactivityTimeout *int
			var ingressPrivate *bool
			var journalRetention *int
			var workflowCompletionRetention *int
			var handlerRetryPolicyInitialInterval, handlerRetryPolicyMaxInterval, handlerRetryPolicyMaxAttempts *int
			var handlerRetryPolicyFactor *float64
			var handlerRetryPolicyOnMaxAttempts *string

			// Check if there's options to apply
			if handler.GetOptions() != nil {
				// Validate which fields are set
				if protocolVersion < ServiceDiscoveryProtocolVersion_V3 {
					if handler.GetOptions().AbortTimeout != nil {
						return nil, fmt.Errorf("handler %s/%s: AbortTimeout option requires discovery protocol version 3 or higher", serviceName, handlerName)
					}
					if handler.GetOptions().EnableLazyState != nil {
						return nil, fmt.Errorf("handler %s/%s: EnableLazyState option requires discovery protocol version 3 or higher", serviceName, handlerName)
					}
					if handler.GetOptions().IdempotencyRetention != nil {
						return nil, fmt.Errorf("handler %s/%s: IdempotencyRetention option requires discovery protocol version 3 or higher", serviceName, handlerName)
					}
					if handler.GetOptions().InactivityTimeout != nil {
						return nil, fmt.Errorf("handler %s/%s: InactivityTimeout option requires discovery protocol version 3 or higher", serviceName, handlerName)
					}
					if handler.GetOptions().IngressPrivate != nil {
						return nil, fmt.Errorf("handler %s/%s: IngressPrivate option requires discovery protocol version 3 or higher", serviceName, handlerName)
					}
					if handler.GetOptions().JournalRetention != nil {
						return nil, fmt.Errorf("handler %s/%s: JournalRetention option requires discovery protocol version 3 or higher", serviceName, handlerName)
					}
					if handler.GetOptions().WorkflowRetention != nil || definition.GetOptions().WorkflowRetention != nil {
						return nil, fmt.Errorf("handler %s/%s: WorkflowRetention option requires discovery protocol version 3 or higher", serviceName, handlerName)
					}
				}
				if protocolVersion < ServiceDiscoveryProtocolVersion_V4 {
					if handler.GetOptions().InvocationRetryPolicy != nil {
						return nil, fmt.Errorf("handler %s/%s: InvocationRetryPolicy option requires discovery protocol version 4 or higher", serviceName, handlerName)
					}
				}

				metadata = handler.GetOptions().Metadata
				documentation = handler.GetOptions().Documentation
				if handler.GetOptions().AbortTimeout != nil {
					abortTimeoutMs := int(handler.GetOptions().AbortTimeout.Milliseconds())
					abortTimeout = &abortTimeoutMs
				}
				enableLazyState = handler.GetOptions().EnableLazyState
				if handler.GetOptions().IdempotencyRetention != nil {
					idempotencyRetentionMs := int(handler.GetOptions().IdempotencyRetention.Milliseconds())
					idempotencyRetention = &idempotencyRetentionMs
				}
				if handler.GetOptions().InactivityTimeout != nil {
					inactivityTimeoutMs := int(handler.GetOptions().InactivityTimeout.Milliseconds())
					inactivityTimeout = &inactivityTimeoutMs
				}
				ingressPrivate = handler.GetOptions().IngressPrivate
				if handler.GetOptions().JournalRetention != nil {
					journalRetentionMs := int(handler.GetOptions().JournalRetention.Milliseconds())
					journalRetention = &journalRetentionMs
				}
				if definition.GetOptions().WorkflowRetention != nil && *handler.HandlerType() == internal.ServiceHandlerType_WORKFLOW {
					// Apply service level configuration here
					workflowCompletionRetentionMs := int(definition.GetOptions().WorkflowRetention.Milliseconds())
					workflowCompletionRetention = &workflowCompletionRetentionMs
				}
				if handler.GetOptions().WorkflowRetention != nil {
					workflowCompletionRetentionMs := int(handler.GetOptions().WorkflowRetention.Milliseconds())
					workflowCompletionRetention = &workflowCompletionRetentionMs
				}
				if handler.GetOptions().InvocationRetryPolicy != nil {
					p := handler.GetOptions().InvocationRetryPolicy
					if p.InitialInterval != nil {
						v := int(p.InitialInterval.Milliseconds())
						handlerRetryPolicyInitialInterval = &v
					}
					if p.MaxInterval != nil {
						v := int(p.MaxInterval.Milliseconds())
						handlerRetryPolicyMaxInterval = &v
					}
					if p.ExponentiationFactor != nil {
						v := float64(*p.ExponentiationFactor)
						handlerRetryPolicyFactor = &v
					}
					if p.MaxAttempts != nil {
						v := int(*p.MaxAttempts)
						handlerRetryPolicyMaxAttempts = &v
					}
					if p.OnMaxAttempts != nil {
						v := string(*p.OnMaxAttempts)
						handlerRetryPolicyOnMaxAttempts = &v
					}
				}
			}
			service.Handlers = append(service.Handlers, internal.Handler{
				Name:                            handlerName,
				Input:                           handler.InputPayload(),
				Output:                          handler.OutputPayload(),
				Ty:                              handler.HandlerType(),
				Metadata:                        metadata,
				Documentation:                   &documentation,
				AbortTimeout:                    abortTimeout,
				EnableLazyState:                 enableLazyState,
				IdempotencyRetention:            idempotencyRetention,
				InactivityTimeout:               inactivityTimeout,
				IngressPrivate:                  ingressPrivate,
				RetryPolicyInitialInterval:      handlerRetryPolicyInitialInterval,
				RetryPolicyMaxInterval:          handlerRetryPolicyMaxInterval,
				RetryPolicyMaxAttempts:          handlerRetryPolicyMaxAttempts,
				RetryPolicyExponentiationFactor: handlerRetryPolicyFactor,
				RetryPolicyOnMaxAttempts:        handlerRetryPolicyOnMaxAttempts,
				JournalRetention:                journalRetention,
				WorkflowCompletionRetention:     workflowCompletionRetention,
			})
		}
		slices.SortFunc(service.Handlers, func(a, b internal.Handler) int {
			return cmp.Compare(a.Name, b.Name)
		})
		resource.Services = append(resource.Services, service)
	}
	slices.SortFunc(resource.Services, func(a, b internal.Service) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return
}
