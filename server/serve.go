package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/restatedev/sdk-go/internal"
	pbinternal "github.com/restatedev/sdk-go/internal/generated"
	"github.com/restatedev/sdk-go/internal/identity"
	"github.com/restatedev/sdk-go/internal/log"
	"github.com/restatedev/sdk-go/internal/restatecontext"
	"github.com/restatedev/sdk-go/internal/statemachine"
	"go.opentelemetry.io/otel/propagation"
)

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
	if strings.TrimSpace(versionString) == "application/vnd.restate.endpointmanifest.v4+json" {
		return ServiceDiscoveryProtocolVersion_V4
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
	case ServiceDiscoveryProtocolVersion_V4:
		return "application/vnd.restate.endpointmanifest.v4+json"
	}
	panic(fmt.Sprintf("unexpected service discovery protocol version %d", serviceDiscoveryProtocolVersion))
}

// takes care of function call
func (r *Restate) handleInvokeRequest(service, method string, writer http.ResponseWriter, request *http.Request) {
	// Make Request context cancellable, we'll need this to unblock reads later on
	ctx, cancel := context.WithCancel(request.Context())
	request = request.WithContext(ctx)

	// Create new connection. cancel will be invoked when the connection is closed.
	stream := newStream(writer, request)

	serviceMethod := fmt.Sprintf("%s/%s", service, method)
	logger := r.systemLog.With("method", slog.StringValue(serviceMethod))

	defer func() {
		cancel()

		if err := stream.Drain(); err != nil {
			logger.WarnContext(ctx, "Failed to drain request stream", log.Error(err))
		}
	}()

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
		writer.WriteHeader(retryableCode(err))
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
		writer.WriteHeader(retryableCode(err))
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
			writer.WriteHeader(retryableCode(err))
			return
		}
		if isReadyToExecute {
			break
		}
		read, err := stream.Read(buf)
		// from the io.Reader docs:
		// Callers should always process the n > 0 bytes returned before considering the error err.
		// Doing so correctly handles I/O errors that happen after reading some bytes and also both of the allowed EOF behaviors.
		if read > 0 {
			if err = stateMachine.NotifyInput(ctx, buf[0:read]); err != nil {
				logger.WarnContext(ctx, "Error when notifying input to the state machine", slog.Any("err", err))
				writer.WriteHeader(retryableCode(err))
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
	if err := restatecontext.ExecuteInvocation(ctx, logger, stateMachine, stream, handler, r.dropReplayLogs, logHandler, request.Header); err != nil {
		r.systemLog.LogAttrs(ctx, slog.LevelError, "Failed to handle invocation", log.Error(err))
	}
}

func (r *Restate) handler(writer http.ResponseWriter, request *http.Request) {
	ctx := tracePropagator.Extract(request.Context(), propagation.HeaderCarrier(request.Header))
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

// Start starts a HTTP2 server listening on address and serving the bound services,
// until ctx is cancelled. It is the same as [Restate.Serve].
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

	r.systemLog.Info(fmt.Sprintf("Restate SDK started listening on %s", listener.Addr()))

	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// Serve starts a HTTP2 server listening on address and serving the bound services,
// until ctx is cancelled. It is an alias of [Restate.Start], named like in the other Restate SDKs.
//
// To serve over an outbound connection to Restate Cloud instead, use [Restate.Connect].
func (r *Restate) Serve(ctx context.Context, address string) error {
	return r.Start(ctx, address)
}
