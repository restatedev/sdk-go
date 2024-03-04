package restate

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
)

type Restate struct{}

func (r *Restate) discover(writer http.ResponseWriter, _ *http.Request) {
	log.Debug().Msg("discover called")
	writer.Header().Add("Content-Type", "application/proto")
	writer.WriteHeader(200)

}

func (r *Restate) handler(writer http.ResponseWriter, request *http.Request) {
	log.Info().Str("proto", request.Proto).Str("method", request.Method).Str("path", request.RequestURI).Msg("got request")

	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if request.RequestURI == "/discover" {
		r.discover(writer, request)
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
