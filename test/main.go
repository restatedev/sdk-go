package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/muhamadazmy/restate-sdk-go"
	"github.com/muhamadazmy/restate-sdk-go/router"
	"github.com/rs/zerolog"
)

func Echo(ctx router.Context, name string) (string, error) {
	return fmt.Sprintf("echo: %s", name), nil
}

func SayHi(ctx router.Context, key string, _ router.Void) (string, error) {
	data, err := ctx.Get("count")
	if err != nil {
		return "", err
	}

	var count uint64
	if data != nil {
		if err := json.Unmarshal(data, &count); err != nil {
			return "", err
		}
	}

	count += 1
	if err := ctx.Set("count", []byte(fmt.Sprint(count))); err != nil {
		return "", err
	}

	return fmt.Sprintf("Hi: %s (%d)", key, count), nil
}

func Keys(ctx router.Context, key string, _ router.Void) (router.Void, error) {

	for i := 0; i < 100; i++ {
		ctx.Set(fmt.Sprintf("key.%d", i), []byte("value"))
	}

	return router.Void{}, nil
}

func main() {

	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	r := restate.NewRestate()

	unKeyed := router.NewUnKeyedRouter().
		Handler("Echo", router.NewUnKeyedHandler(Echo))

	keyed := router.NewKeyedRouter().
		Handler("SayHi", router.NewKeyedHandler(SayHi)).
		Handler("Keys", router.NewKeyedHandler(Keys))

	r.
		Bind("UnKeyed", unKeyed).
		Bind("Keyed", keyed)

	if err := r.Start(context.Background(), ":9080"); err != nil {
		panic(err)
	}
}
