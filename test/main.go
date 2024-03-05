package main

import (
	"context"
	"fmt"

	"github.com/muhamadazmy/restate-sdk-go"
	"github.com/rs/zerolog"
)

func Echo(ctx restate.Context, name string) (string, error) {
	return name, nil
}

func SayHi(ctx restate.Context, key string, name string) (string, error) {
	return fmt.Sprintf("Hi: %s", name), nil
}

func main() {

	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	r := restate.NewRestate()

	unKeyed := restate.NewUnKeyedRouter().
		Handler("Echo", restate.NewUnKeyedHandler(Echo))

	keyed := restate.NewKeyedRouter().
		Handler("SayHi", restate.NewKeyedHandler(SayHi))
	r.
		Bind("Test", unKeyed).
		Bind("TestKeyed", keyed)

	if err := r.Start(context.Background(), ":9080"); err != nil {
		panic(err)
	}
}
