package server

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/internal"
	pbinternal "github.com/restatedev/sdk-go/internal/generated"
	"github.com/restatedev/sdk-go/internal/identity"
	"github.com/restatedev/sdk-go/internal/log"
	"github.com/restatedev/sdk-go/internal/restatecontext"
	"github.com/restatedev/sdk-go/internal/statemachine"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type ServiceProtocolVersion int32
type ServiceDiscoveryProtocolVersion int32

const (
	ServiceDiscoveryProtocolVersion_UNKNOWN ServiceDiscoveryProtocolVersion = 0
	ServiceDiscoveryProtocolVersion_V1      ServiceDiscoveryProtocolVersion = 1
	ServiceDiscoveryProtocolVersion_V2      ServiceDiscoveryProtocolVersion = 2
	ServiceDiscoveryProtocolVersion_V3      ServiceDiscoveryProtocolVersion = 3
	minServiceDiscoveryProtocolVersion                                      = ServiceDiscoveryProtocolVersion_V2
	maxServiceDiscoveryProtocolVersion                                      = ServiceDiscoveryProtocolVersion_V3
	minServiceProtocolVersion                                               = 5
	maxServiceProtocolVersion                                               = 5
)

var xRestateServer = `restate-sdk-go/unknown`

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
// status in a slog.Handler using [github.com/restatedev/sdk-go/rcontext.LogContextFrom]
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
		}
		service := internal.Service{
			Name:                 serviceName,
			Ty:                   definition.Type(),
			Handlers:             make([]internal.Handler, 0, len(definition.Handlers())),
			Metadata:             metadata,
			Documentation:        &documentation,
			AbortTimeout:         abortTimeout,
			EnableLazyState:      enableLazyState,
			IdempotencyRetention: idempotencyRetention,
			InactivityTimeout:    inactivityTimeout,
			IngressPrivate:       ingressPrivate,
			JournalRetention:     journalRetention,
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
					if handler.GetOptions().WorkflowRetention != nil {
						return nil, fmt.Errorf("handler %s/%s: WorkflowRetention option requires discovery protocol version 3 or higher", serviceName, handlerName)
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
				if handler.GetOptions().WorkflowRetention != nil {
					workflowCompletionRetentionMs := int(handler.GetOptions().WorkflowRetention.Milliseconds())
					workflowCompletionRetention = &workflowCompletionRetentionMs
				}
			}
			service.Handlers = append(service.Handlers, internal.Handler{
				Name:                        handlerName,
				Input:                       handler.InputPayload(),
				Output:                      handler.OutputPayload(),
				Ty:                          handler.HandlerType(),
				Metadata:                    metadata,
				Documentation:               &documentation,
				AbortTimeout:                abortTimeout,
				EnableLazyState:             enableLazyState,
				IdempotencyRetention:        idempotencyRetention,
				InactivityTimeout:           inactivityTimeout,
				IngressPrivate:              ingressPrivate,
				JournalRetention:            journalRetention,
				WorkflowCompletionRetention: workflowCompletionRetention,
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

func (r *Restate) handleHealthRequest(writer http.ResponseWriter) {
	writer.Header().Set("Content-Type", "text/plain")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte("ok"))
}

func (r *Restate) handleDiscoveryRequest(writer http.ResponseWriter, req *http.Request) {
	r.systemLog.DebugContext(req.Context(), "Processing discovery request")

	acceptVersionsString := req.Header.Get("accept")
	if acceptVersionsString == "" {
		writer.WriteHeader(http.StatusUnsupportedMediaType)
		writer.Write([]byte("missing accept header"))

		return
	}

	serviceDiscoveryProtocolVersion := selectSupportedServiceDiscoveryProtocolVersion(acceptVersionsString)

	if serviceDiscoveryProtocolVersion == ServiceDiscoveryProtocolVersion_UNKNOWN {
		writer.WriteHeader(http.StatusUnsupportedMediaType)
		writer.Write([]byte(fmt.Sprintf("Unsupported service discovery protocol version '%s'", acceptVersionsString)))
		return
	}

	response, err := r.discover(serviceDiscoveryProtocolVersion)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		writer.Write([]byte(err.Error()))

		return
	}

	bytes, err := json.Marshal(response)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		writer.Write([]byte(err.Error()))

		return
	}

	writer.Header().Add("Content-Type", serviceDiscoveryProtocolVersionToHeaderValue(serviceDiscoveryProtocolVersion))
	if _, err := writer.Write(bytes); err != nil {
		r.systemLog.LogAttrs(req.Context(), slog.LevelError, "Failed to write discovery information", log.Error(err))
	}
}

func selectSupportedServiceDiscoveryProtocolVersion(accept string) ServiceDiscoveryProtocolVersion {
	maxVersion := ServiceDiscoveryProtocolVersion_UNKNOWN

	for _, versionString := range strings.Split(accept, ",") {
		version := parseServiceDiscoveryProtocolVersion(versionString)
		if isServiceDiscoveryProtocolVersionSupported(version) && version > maxVersion {
			maxVersion = version
		}
	}

	return maxVersion
}

func parseServiceDiscoveryProtocolVersion(versionString string) ServiceDiscoveryProtocolVersion {
	if strings.TrimSpace(versionString) == "application/vnd.restate.endpointmanifest.v1+json" {
		return ServiceDiscoveryProtocolVersion_V1
	}
	if strings.TrimSpace(versionString) == "application/vnd.restate.endpointmanifest.v2+json" {
		return ServiceDiscoveryProtocolVersion_V2
	}
	if strings.TrimSpace(versionString) == "application/vnd.restate.endpointmanifest.v3+json" {
		return ServiceDiscoveryProtocolVersion_V3
	}

	return ServiceDiscoveryProtocolVersion_UNKNOWN
}

func isServiceDiscoveryProtocolVersionSupported(version ServiceDiscoveryProtocolVersion) bool {
	return version >= minServiceDiscoveryProtocolVersion && version <= maxServiceDiscoveryProtocolVersion
}

func serviceDiscoveryProtocolVersionToHeaderValue(serviceDiscoveryProtocolVersion ServiceDiscoveryProtocolVersion) string {
	switch serviceDiscoveryProtocolVersion {
	case ServiceDiscoveryProtocolVersion_V1:
		return "application/vnd.restate.endpointmanifest.v1+json"
	case ServiceDiscoveryProtocolVersion_V2:
		return "application/vnd.restate.endpointmanifest.v2+json"
	case ServiceDiscoveryProtocolVersion_V3:
		return "application/vnd.restate.endpointmanifest.v3+json"
	}
	panic(fmt.Sprintf("unexpected service discovery protocol version %d", serviceDiscoveryProtocolVersion))
}

// takes care of function call
func (r *Restate) handleInvokeRequest(service, method string, writer http.ResponseWriter, request *http.Request) {
	// Make Request context cancellable, we'll need this to unblock reads later on
	ctx, cancel := context.WithCancel(request.Context())
	request = request.WithContext(ctx)

	// Create new connection. cancel will be invoked when the connection is closed.
	conn := newConnection(writer, request, cancel)

	serviceMethod := fmt.Sprintf("%s/%s", service, method)
	logger := r.systemLog.With("method", slog.StringValue(serviceMethod))

	definition, ok := r.definitions[service]
	if !ok {
		logger.WarnContext(ctx, "Service not found")
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	handler, ok := definition.Handlers()[method]
	if !ok {
		logger.WarnContext(ctx, "Method not found on service")
		writer.WriteHeader(http.StatusNotFound)
	}

	// Instantiate vm
	core, err := statemachine.NewCore(ctx)
	if err != nil {
		return
	}
	var headers []*pbinternal.Header
	for k, v := range request.Header {
		header := pbinternal.Header{}
		header.SetKey(k)
		header.SetValue(v[0])
		headers = append(headers, &header)
	}
	stateMachine, err := core.NewStateMachine(ctx, headers)
	if err != nil {
		logger.WarnContext(ctx, "Error when instantiating the state machine", slog.Any("err", err))
		writer.WriteHeader(int(restate.ErrorCode(err)))
		return
	}

	// Free state machine at the end of the request
	defer func() {
		if err = stateMachine.Free(ctx); err != nil {
			logger.WarnContext(ctx, "Error when freeing the state machine", slog.Any("err", err))
		}
		if err = core.Close(ctx); err != nil {
			logger.WarnContext(ctx, "Error when closing the core", slog.Any("err", err))
		}
	}()

	// Write response headers
	responseHeaders, err := stateMachine.GetResponseHead(ctx)
	if err != nil {
		logger.WarnContext(ctx, "Error when getting response head from the state machine", slog.Any("err", err))
		writer.WriteHeader(int(restate.ErrorCode(err)))
		return
	}
	for _, h := range responseHeaders.GetHeaders() {
		writer.Header().Add(h.GetKey(), h.GetValue())
	}

	// Now buffer input entries until the state machine is ready to execute
	buf := restatecontext.BufPool.Get().([]byte)
	for {
		isReadyToExecute, err := stateMachine.IsReadyToExecute(ctx)
		if err != nil {
			logger.WarnContext(ctx, "Error when preparing the state machine", slog.Any("err", err))
			writer.WriteHeader(int(restate.ErrorCode(err)))
			return
		}
		if isReadyToExecute {
			break
		}
		read, err := conn.Read(buf)
		// from the io.Reader docs:
		// Callers should always process the n > 0 bytes returned before considering the error err.
		// Doing so correctly handles I/O errors that happen after reading some bytes and also both of the allowed EOF behaviors.
		if read > 0 {
			if err = stateMachine.NotifyInput(ctx, buf[0:read]); err != nil {
				logger.WarnContext(ctx, "Error when notifying input to the state machine", slog.Any("err", err))
				writer.WriteHeader(int(restate.ErrorCode(err)))
				return
			}
		}
		if err == io.EOF {
			if err = stateMachine.NotifyInputClosed(ctx); err != nil {
				logger.WarnContext(ctx, "Error when notifying input closed to the state machine", slog.Any("err", err))
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else if err != nil {
			logger.WarnContext(ctx, "Error when reading the input stream", slog.Any("err", err))
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	// From this point on, we're good,
	// let's send back 200 and start processing
	writer.WriteHeader(200)

	logHandler := r.logHandler.WithAttrs([]slog.Attr{
		slog.String("method", serviceMethod),
	})

	restatecontext.BufPool.Put(buf)

	// Run the handler
	if err := restatecontext.ExecuteInvocation(ctx, logger, stateMachine, conn, handler, r.dropReplayLogs, logHandler, request.Header); err != nil {
		r.systemLog.LogAttrs(ctx, slog.LevelError, "Failed to handle invocation", log.Error(err))
	}
}

func (r *Restate) handler(writer http.ResponseWriter, request *http.Request) {
	ctx := otel.GetTextMapPropagator().Extract(request.Context(), propagation.HeaderCarrier(request.Header))
	request = request.WithContext(ctx)

	writer.Header().Add("x-restate-server", xRestateServer)

	if request.RequestURI == "/health" {
		r.handleHealthRequest(writer)
		return
	}

	if r.keySet != nil {
		if err := identity.ValidateRequestIdentity(r.keySet, request.RequestURI, request.Header); err != nil {
			r.systemLog.LogAttrs(request.Context(), slog.LevelError, "Rejecting request as its JWT did not validate", log.Error(err))

			writer.WriteHeader(http.StatusUnauthorized)
			writer.Write([]byte("Unauthorized"))

			return
		}
	}

	if request.RequestURI == "/discover" {
		r.handleDiscoveryRequest(writer, request)
		return
	}

	if r.protocolMode == internal.ProtocolMode_BIDI_STREAM && !request.ProtoAtLeast(2, 0) {
		// bidi http1.1 requires enabling full duplex
		rc := http.NewResponseController(writer)
		if err := rc.EnableFullDuplex(); err != nil {
			r.systemLog.LogAttrs(request.Context(), slog.LevelError, "Could not enable full duplex mode on the underlying HTTP1 transport, server must be created with .Bidirectional(false)", log.Error(err))

			writer.WriteHeader(http.StatusInternalServerError)
			writer.Write([]byte("BIDI_STREAM not supported"))

			return
		}
	}

	serviceProtocolVersionString := request.Header.Get("content-type")
	if serviceProtocolVersionString == "" {
		r.systemLog.ErrorContext(request.Context(), "Missing content-type header")

		writer.WriteHeader(http.StatusUnsupportedMediaType)
		writer.Write([]byte("missing content-type header"))

		return
	}

	// we expecting the uri to be something like `/invoke/{service}/{method}`
	// so
	if !strings.HasPrefix(request.RequestURI, "/invoke/") {
		r.systemLog.LogAttrs(request.Context(), slog.LevelError, "Invalid request path", slog.String("path", request.RequestURI))
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	parts := strings.Split(strings.TrimPrefix(request.RequestURI, "/invoke/"), "/")
	if len(parts) != 2 {
		r.systemLog.LogAttrs(request.Context(), slog.LevelError, "Invalid request path", slog.String("path", request.RequestURI))
		writer.WriteHeader(http.StatusNotFound)

		return
	}

	r.handleInvokeRequest(parts[0], parts[1], writer, request)
}

// Handler obtains a [http.HandlerFunc] representing the bound services which can be passed to other types of server.
// Ensure that .Bidirectional(false) is set when serving over a channel that doesn't support full-duplex request and response.
func (r *Restate) Handler() (http.HandlerFunc, error) {
	if r.keyIDs == nil {
		r.systemLog.Warn("Accepting requests without validating request signatures; handler access must be restricted")
	} else {
		ks, err := identity.ParseKeySetV1(r.keyIDs)
		if err != nil {
			return nil, fmt.Errorf("invalid request identity keys: %w", err)
		}
		r.keySet = ks
		r.systemLog.Info("Validating requests using signing keys", "keys", r.keyIDs)
	}

	return http.HandlerFunc(r.handler), nil
}

// Start starts a HTTP2 server serving the bound services
func (r *Restate) Start(ctx context.Context, address string) error {
	handler, err := r.Handler()
	if err != nil {
		return err
	}

	var protocols http.Protocols
	protocols.SetUnencryptedHTTP2(true)

	server := &http.Server{
		Handler:           handler,
		Protocols:         &protocols,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			r.systemLog.Error("Server shutdown error", "error", err)
		}
	}()

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on address %s: %w", address, err)
	}

	slog.Info(fmt.Sprintf("Restate SDK started listening on %s", listener.Addr()))

	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}
