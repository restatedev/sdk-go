package main

import (
	"context"

	"github.com/muhamadazmy/restate-sdk-go"
	"github.com/rs/zerolog"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	var r restate.Restate

	if err := r.Start(context.Background(), ":9080"); err != nil {
		panic(err)
	}
}
