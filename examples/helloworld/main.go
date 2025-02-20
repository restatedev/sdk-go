package main

import (
	"context"
	"fmt"
	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/server"
	"log"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"
)

type Greeter struct{}

func (Greeter) Greet(ctx restate.Context, name string) (string, error) {
	return "You said hi to " + name + "!", nil
}

type GreeterCounter struct{}

func (GreeterCounter) Greet(ctx restate.ObjectContext, name string) (string, error) {
	count, err := restate.Get[uint32](ctx, "count")
	if err != nil {
		return "", err
	}
	count++

	restate.Set[uint32](ctx, "count", count)

	runRes, err := restate.Run(ctx, func(ctx restate.RunContext) (string, error) {
		return "myass", nil
	})
	if err != nil {
		return "", err
	}

	// Sleep my ass
	if err := restate.Sleep(ctx, 1*time.Minute); err != nil {
		return "", err
	}

	return fmt.Sprintf("You said hi to %s for the %d time! Run returned %s", name, count, runRes), nil
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	server := server.NewRestate().
		// Handlers can be inferred from object methods
		Bind(restate.Reflect(Greeter{})).
		Bind(restate.Reflect(GreeterCounter{}))

	if err := server.Start(context.Background(), ":9080"); err != nil {
		slog.Error("application exited unexpectedly", "err", err.Error())
		os.Exit(1)
	}
}
