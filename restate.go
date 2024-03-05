package restate

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/muhamadazmy/restate-sdk-go/generated/discovery"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
	"google.golang.org/protobuf/proto"
)

type Restate struct {
	routers map[string]Router
}

func NewRestate() *Restate {
	return &Restate{
		routers: make(map[string]Router),
	}
}

func (r *Restate) Bind(name string, router Router) *Restate {
	if _, ok := r.routers[name]; ok {
		// panic because this is a programming error
		// to register multiple router with the same name
		panic("router with the same name exists")
	}

	r.routers[name] = router

	return r
}

func (r *Restate) discover() (resource *discovery.ServiceDiscoveryResponse, err error) {
	ds := NewDynRpcDescriptorSet()
	resource = &discovery.ServiceDiscoveryResponse{
		ProtocolMode: discovery.ProtocolMode_BIDI_STREAM,
		Files:        ds.Inner(),
	}

	for name, router := range r.routers {
		resource.Services = append(resource.Services, name)
		var service *DynRpcService
		if router.Keyed() {
			service, err = ds.AddKeyedService(name)
		} else {
			service, err = ds.AddUnKeyedService(name)
		}

		if err != nil {
			return resource, fmt.Errorf("failed to build service '%s': %w", name, err)
		}

		for name := range router.Handlers() {
			service.AddHandler(name)
		}
	}

	resource.Files = ds.Inner()

	return
}

func (r *Restate) discoverHandler(writer http.ResponseWriter, _ *http.Request) {
	log.Debug().Msg("discover called")
	writer.Header().Add("Content-Type", "application/proto")

	response, err := r.discover()
	if err != nil {
		writer.Write([]byte(err.Error()))
		writer.WriteHeader(http.StatusInternalServerError)

		return
	}

	bytes, err := proto.Marshal(response)
	if err != nil {
		writer.Write([]byte(err.Error()))
		writer.WriteHeader(http.StatusInternalServerError)

		return
	}

	writer.WriteHeader(200)
	if _, err := writer.Write(bytes); err != nil {
		log.Error().Err(err).Msg("failed to write discovery information")
	}
}

func (r *Restate) handler(writer http.ResponseWriter, request *http.Request) {
	log.Info().Str("proto", request.Proto).Str("method", request.Method).Str("path", request.RequestURI).Msg("got request")

	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if request.RequestURI == "/discover" {
		r.discoverHandler(writer, request)
		return
	}

	// handle method!
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
