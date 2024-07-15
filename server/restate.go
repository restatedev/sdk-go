package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/posener/h2conn"
	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/generated/proto/discovery"
	"github.com/restatedev/sdk-go/generated/proto/protocol"
	"github.com/restatedev/sdk-go/internal"
	"github.com/restatedev/sdk-go/internal/identity"
	"github.com/restatedev/sdk-go/internal/log"
	"github.com/restatedev/sdk-go/internal/state"
	"golang.org/x/net/http2"
)

const MIN_SERVICE_PROTOCOL_VERSION protocol.ServiceProtocolVersion = protocol.ServiceProtocolVersion_V1
const MAX_SERVICE_PROTOCOL_VERSION protocol.ServiceProtocolVersion = protocol.ServiceProtocolVersion_V1
const MIN_SERVICE_DISCOVERY_PROTOCOL_VERSION discovery.ServiceDiscoveryProtocolVersion = discovery.ServiceDiscoveryProtocolVersion_V1
const MAX_SERVICE_DISCOVERY_PROTOCOL_VERSION discovery.ServiceDiscoveryProtocolVersion = discovery.ServiceDiscoveryProtocolVersion_V1

var X_RESTATE_SERVER = `restate-sdk-go/unknown`

func init() {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	for _, dep := range bi.Deps {
		if dep.Path == "github.com/restatedev/sdk-go" {
			X_RESTATE_SERVER = "restate-sdk-go/" + dep.Version
			break
		}
	}
}

type Restate struct {
	logHandler     slog.Handler
	dropReplayLogs bool
	systemLog      *slog.Logger
	routers        map[string]restate.Router
	keyIDs         []string
	keySet         identity.KeySetV1
}

// NewRestate creates a new instance of Restate server
func NewRestate() *Restate {
	handler := slog.Default().Handler()
	return &Restate{
		logHandler:     handler,
		systemLog:      slog.New(log.NewRestateContextHandler(handler)),
		dropReplayLogs: true,
		routers:        make(map[string]restate.Router),
	}
}

// WithLogger overrides the slog handler used by the SDK (which defaults to the slog Default())
// You may specify with dropReplayLogs whether to drop logs that originated from handler code
// while the invocation was replaying. If they are not dropped, you may still determine the replay
// status in a slog.Handler using rcontext.LogContextFrom(ctx)
func (r *Restate) WithLogger(h slog.Handler, dropReplayLogs bool) *Restate {
	r.dropReplayLogs = dropReplayLogs
	r.systemLog = slog.New(log.NewRestateContextHandler(h))
	r.logHandler = h
	return r
}

func (r *Restate) WithIdentityV1(keys ...string) *Restate {
	r.keyIDs = append(r.keyIDs, keys...)
	return r
}

func (r *Restate) Bind(router restate.Router) *Restate {
	if _, ok := r.routers[router.Name()]; ok {
		// panic because this is a programming error
		// to register multiple router with the same name
		panic("router with the same name exists")
	}

	r.routers[router.Name()] = router

	return r
}

func (r *Restate) discover() (resource *internal.Endpoint, err error) {
	resource = &internal.Endpoint{
		ProtocolMode:       internal.ProtocolMode_BIDI_STREAM,
		MinProtocolVersion: 1,
		MaxProtocolVersion: 2,
		Services:           make([]internal.Service, 0, len(r.routers)),
	}

	for name, router := range r.routers {
		service := internal.Service{
			Name:     name,
			Ty:       router.Type(),
			Handlers: make([]internal.Handler, 0, len(router.Handlers())),
		}

		for name, handler := range router.Handlers() {
			service.Handlers = append(service.Handlers, internal.Handler{
				Name:   name,
				Input:  handler.InputPayload(),
				Output: handler.OutputPayload(),
			})
		}
		resource.Services = append(resource.Services, service)
	}

	return
}

func (r *Restate) discoverHandler(writer http.ResponseWriter, req *http.Request) {
	r.systemLog.DebugContext(req.Context(), "Processing discovery request")

	acceptVersionsString := req.Header.Get("accept")
	if acceptVersionsString == "" {
		writer.WriteHeader(http.StatusUnsupportedMediaType)
		writer.Write([]byte("missing accept header"))

		return
	}

	serviceDiscoveryProtocolVersion := selectSupportedServiceDiscoveryProtocolVersion(acceptVersionsString)

	if serviceDiscoveryProtocolVersion == discovery.ServiceDiscoveryProtocolVersion_SERVICE_DISCOVERY_PROTOCOL_VERSION_UNSPECIFIED {
		writer.WriteHeader(http.StatusUnsupportedMediaType)
		writer.Write([]byte(fmt.Sprintf("Unsupported service discovery protocol version '%s'", acceptVersionsString)))
		return
	}

	response, err := r.discover()
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

func selectSupportedServiceDiscoveryProtocolVersion(accept string) discovery.ServiceDiscoveryProtocolVersion {
	maxVersion := discovery.ServiceDiscoveryProtocolVersion_SERVICE_DISCOVERY_PROTOCOL_VERSION_UNSPECIFIED

	for _, versionString := range strings.Split(accept, ",") {
		version := parseServiceDiscoveryProtocolVersion(versionString)
		if isServiceDiscoveryProtocolVersionSupported(version) && version > maxVersion {
			maxVersion = version
		}
	}

	return maxVersion
}

func parseServiceDiscoveryProtocolVersion(versionString string) discovery.ServiceDiscoveryProtocolVersion {
	if strings.TrimSpace(versionString) == "application/vnd.restate.endpointmanifest.v1+json" {
		return discovery.ServiceDiscoveryProtocolVersion_V1
	}

	return discovery.ServiceDiscoveryProtocolVersion_SERVICE_DISCOVERY_PROTOCOL_VERSION_UNSPECIFIED
}

func isServiceDiscoveryProtocolVersionSupported(version discovery.ServiceDiscoveryProtocolVersion) bool {
	return version >= MIN_SERVICE_DISCOVERY_PROTOCOL_VERSION && version <= MAX_SERVICE_DISCOVERY_PROTOCOL_VERSION
}

func serviceDiscoveryProtocolVersionToHeaderValue(serviceDiscoveryProtocolVersion discovery.ServiceDiscoveryProtocolVersion) string {
	switch serviceDiscoveryProtocolVersion {
	case discovery.ServiceDiscoveryProtocolVersion_V1:
		return "application/vnd.restate.endpointmanifest.v1+json"
	}
	panic(fmt.Sprintf("unexpected service discovery protocol version %d", serviceDiscoveryProtocolVersion))
}

func parseServiceProtocolVersion(versionString string) protocol.ServiceProtocolVersion {
	if strings.TrimSpace(versionString) == "application/vnd.restate.invocation.v1" {
		return protocol.ServiceProtocolVersion_V1
	}

	return protocol.ServiceProtocolVersion_SERVICE_PROTOCOL_VERSION_UNSPECIFIED
}

func isServiceProtocolVersionSupported(version protocol.ServiceProtocolVersion) bool {
	return version >= MIN_SERVICE_PROTOCOL_VERSION && version <= MAX_SERVICE_PROTOCOL_VERSION
}

func serviceProtocolVersionToHeaderValue(serviceProtocolVersion protocol.ServiceProtocolVersion) string {
	switch serviceProtocolVersion {
	case protocol.ServiceProtocolVersion_V1:
		return "application/vnd.restate.invocation.v1"
	}
	panic(fmt.Sprintf("unexpected service protocol version %d", serviceProtocolVersion))
}

type serviceMethod struct {
	service string
	method  string
}

// takes care of function call
func (r *Restate) callHandler(serviceProtocolVersion protocol.ServiceProtocolVersion, service, method string, writer http.ResponseWriter, request *http.Request) {
	logger := r.systemLog.With("method", slog.StringValue(fmt.Sprintf("%s/%s", service, method)))

	writer.Header().Add("x-restate-server", X_RESTATE_SERVER)
	writer.Header().Add("content-type", serviceProtocolVersionToHeaderValue(serviceProtocolVersion))

	router, ok := r.routers[service]
	if !ok {
		logger.WarnContext(request.Context(), "Service not found")
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	handler, ok := router.Handlers()[method]
	if !ok {
		logger.WarnContext(request.Context(), "Method not found on service")
		writer.WriteHeader(http.StatusNotFound)
	}

	writer.WriteHeader(200)

	conn, err := h2conn.Accept(writer, request)
	if err != nil {
		logger.LogAttrs(request.Context(), slog.LevelError, "Failed to upgrade connection", log.Error(err))
		return
	}

	defer conn.Close()

	machine := state.NewMachine(handler, conn)

	if err := machine.Start(request.Context(), r.dropReplayLogs, r.logHandler); err != nil {
		machine.Log().LogAttrs(request.Context(), slog.LevelError, "Failed to handle invocation", log.Error(err))
	}
}

func (r *Restate) handler(writer http.ResponseWriter, request *http.Request) {
	if r.keySet != nil {
		if err := identity.ValidateRequestIdentity(r.keySet, request.RequestURI, request.Header); err != nil {
			r.systemLog.LogAttrs(request.Context(), slog.LevelError, "Rejecting request as its JWT did not validate", log.Error(err))

			writer.WriteHeader(http.StatusUnauthorized)
			writer.Write([]byte("Unauthorized"))

			return
		}
	}

	if request.RequestURI == "/discover" {
		r.discoverHandler(writer, request)
		return
	}

	serviceProtocolVersionString := request.Header.Get("content-type")
	if serviceProtocolVersionString == "" {
		r.systemLog.ErrorContext(request.Context(), "Missing content-type header")

		writer.WriteHeader(http.StatusUnsupportedMediaType)
		writer.Write([]byte("missing content-type header"))

		return
	}

	serviceProtocolVersion := parseServiceProtocolVersion(serviceProtocolVersionString)

	if !isServiceProtocolVersionSupported(serviceProtocolVersion) {
		r.systemLog.LogAttrs(request.Context(), slog.LevelError, "Unsupported service protocol version", slog.String("version", serviceProtocolVersionString))

		writer.WriteHeader(http.StatusUnsupportedMediaType)
		writer.Write([]byte(fmt.Sprintf("Unsupported service protocol version '%s'", serviceProtocolVersionString)))

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

	r.callHandler(serviceProtocolVersion, parts[0], parts[1], writer, request)
}

func (r *Restate) Start(ctx context.Context, address string) error {
	if r.keyIDs == nil {
		r.systemLog.WarnContext(ctx, "Accepting requests without validating request signatures; handler access must be restricted")
	} else {
		ks, err := identity.ParseKeySetV1(r.keyIDs)
		if err != nil {
			return fmt.Errorf("invalid request identity keys: %w", err)
		}
		r.keySet = ks
		r.systemLog.LogAttrs(ctx, slog.LevelInfo, "Validating requests using signing keys", slog.Any("keys", r.keyIDs))
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on address %s: %w", address, err)
	}

	var h2server http2.Server

	opts := &http2.ServeConnOpts{
		Context: ctx,
		Handler: http.HandlerFunc(r.handler),
	}

	for {
		con, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("failed to accept connection: %w", err)
		}

		go h2server.ServeConn(con, opts)
	}
}
