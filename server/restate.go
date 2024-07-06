package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/posener/h2conn"
	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/generated/proto/discovery"
	"github.com/restatedev/sdk-go/generated/proto/protocol"
	"github.com/restatedev/sdk-go/internal"
	"github.com/restatedev/sdk-go/internal/state"
	"github.com/rs/zerolog/log"
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
	routers map[string]restate.Router
}

// NewRestate creates a new instance of Restate server
func NewRestate() *Restate {
	return &Restate{
		routers: make(map[string]restate.Router),
	}
}

func (r *Restate) Bind(name string, router restate.Router) *Restate {
	if _, ok := r.routers[name]; ok {
		// panic because this is a programming error
		// to register multiple router with the same name
		panic("router with the same name exists")
	}

	r.routers[name] = router

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

		for name := range router.Handlers() {
			service.Handlers = append(service.Handlers, internal.Handler{
				Name: name,
				Input: &internal.InputPayload{
					Required:    false,
					ContentType: "application/json", // TODO configurable handler encoding
				},
				Output: &internal.OutputPayload{
					SetContentTypeIfEmpty: false,
					ContentType:           "application/json",
				},
			})
		}
		resource.Services = append(resource.Services, service)
	}

	return
}

func (r *Restate) discoverHandler(writer http.ResponseWriter, req *http.Request) {
	log.Trace().Msg("discover called")

	acceptVersionsString := req.Header.Get("accept")
	if acceptVersionsString == "" {
		writer.Write([]byte("missing accept header"))
		writer.WriteHeader(http.StatusUnsupportedMediaType)

		return
	}

	serviceDiscoveryProtocolVersion := selectSupportedServiceDiscoveryProtocolVersion(acceptVersionsString)

	if serviceDiscoveryProtocolVersion == discovery.ServiceDiscoveryProtocolVersion_SERVICE_DISCOVERY_PROTOCOL_VERSION_UNSPECIFIED {
		writer.Write([]byte(fmt.Sprint("Unsupported service discovery protocol version '%s'", acceptVersionsString)))
		writer.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}

	response, err := r.discover()
	if err != nil {
		writer.Write([]byte(err.Error()))
		writer.WriteHeader(http.StatusInternalServerError)

		return
	}

	bytes, err := json.Marshal(response)
	if err != nil {
		writer.Write([]byte(err.Error()))
		writer.WriteHeader(http.StatusInternalServerError)

		return
	}

	writer.Header().Add("Content-Type", serviceDiscoveryProtocolVersionToHeaderValue(serviceDiscoveryProtocolVersion))
	writer.WriteHeader(200)
	if _, err := writer.Write(bytes); err != nil {
		log.Error().Err(err).Msg("failed to write discovery information")
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

// takes care of function call
func (r *Restate) callHandler(serviceProtocolVersion protocol.ServiceProtocolVersion, service, fn string, writer http.ResponseWriter, request *http.Request) {
	log.Debug().Str("service", service).Str("handler", fn).Msg("new request")

	writer.Header().Add("x-restate-server", X_RESTATE_SERVER)
	writer.Header().Add("content-type", serviceProtocolVersionToHeaderValue(serviceProtocolVersion))

	router, ok := r.routers[service]
	if !ok {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	handler, ok := router.Handlers()[fn]
	if !ok {
		writer.WriteHeader(http.StatusNotFound)
	}

	writer.WriteHeader(200)

	conn, err := h2conn.Accept(writer, request)

	if err != nil {
		log.Error().Err(err).Msg("failed to upgrade connection")
		return
	}

	defer conn.Close()

	machine := state.NewMachine(handler, conn)

	if err := machine.Start(request.Context(), fmt.Sprintf("%s/%s", service, fn)); err != nil {
		log.Error().Err(err).Msg("failed to handle invocation")
	}
}

func (r *Restate) handler(writer http.ResponseWriter, request *http.Request) {
	log.Trace().Str("proto", request.Proto).Str("method", request.Method).Str("path", request.RequestURI).Msg("got request")

	if request.RequestURI == "/discover" {
		r.discoverHandler(writer, request)
		return
	}

	serviceProtocolVersionString := request.Header.Get("content-type")
	if serviceProtocolVersionString == "" {
		writer.Write([]byte("missing content-type header"))
		writer.WriteHeader(http.StatusUnsupportedMediaType)

		return
	}

	serviceProtocolVersion := parseServiceProtocolVersion(serviceProtocolVersionString)

	if !isServiceProtocolVersionSupported(serviceProtocolVersion) {
		writer.Write([]byte(fmt.Sprintf("Unsupported service protocol version '%s'", serviceProtocolVersionString)))
		writer.WriteHeader(http.StatusUnsupportedMediaType)

		return
	}

	// we expecting the uri to be something like `/invoke/{service}/{method}`
	// so
	if !strings.HasPrefix(request.RequestURI, "/invoke/") {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	parts := strings.Split(strings.TrimPrefix(request.RequestURI, "/invoke/"), "/")
	if len(parts) != 2 {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	r.callHandler(serviceProtocolVersion, parts[0], parts[1], writer, request)
}

func (r *Restate) Start(ctx context.Context, address string) error {

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
