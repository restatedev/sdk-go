package main

import (
	"context"
	"fmt"

	"github.com/muhamadazmy/restate-sdk-go"
	"github.com/muhamadazmy/restate-sdk-go/router"
	"github.com/rs/zerolog"
)

func Echo(ctx router.Context, name string) (string, error) {
	return fmt.Sprintf("echo: %s", name), nil
}

func SayHi(ctx router.Context, key string, name string) (string, error) {
	return fmt.Sprintf("Hi: %s", name), nil
}

func main() {

	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	r := restate.NewRestate()

	unKeyed := router.NewUnKeyedRouter().
		Handler("Echo", router.NewUnKeyedHandler(Echo))

	keyed := router.NewKeyedRouter().
		Handler("SayHi", router.NewKeyedHandler(SayHi))
	r.
		Bind("Test", unKeyed).
		Bind("TestKeyed", keyed)

	if err := r.Start(context.Background(), ":9080"); err != nil {
		panic(err)
	}
}
