package main

import (
	"context"
	"os"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/server"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	server := server.NewRestate().
		Bind(restate.Object(&userSession{})).
		Bind(restate.Object(&ticketService{})).
		Bind(restate.Service(&checkout{}))

	if err := server.Start(context.Background(), ":9080"); err != nil {
		log.Error().Err(err).Msg("application exited unexpectedly")
		os.Exit(1)
	}
}
