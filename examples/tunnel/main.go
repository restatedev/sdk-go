// Command tunnel is an example of serving a Restate deployment over an outbound
// connection to Restate Cloud instead of listening for inbound connections.
//
// Copy a connect key from the Restate Cloud UI (Development → Configure SDK) and run:
//
//	RESTATE_CONNECT_KEY=rstcloudv1_... go run .
//
// Once connected, it logs how to register the deployment, once:
//
//	restate deployments register connect://greeter
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/server"
)

type Greeter struct{}

func (Greeter) Greet(ctx restate.Context, name string) (string, error) {
	greeting := restate.RunAsync(ctx, func(ctx restate.RunContext) (string, error) {
		return "You said hi to " + name + "!", nil
	})

	timeout := restate.After(ctx, 5*time.Second)

	first, err := restate.WaitFirst(ctx, greeting, timeout)
	if err != nil {
		return "", err
	}

	switch first {
	case greeting:
		return greeting.Result()
	case timeout:
		return "", restate.TerminalErrorf("timed out generating a greeting for %s", name)
	default:
		return "", restate.TerminalErrorf("unexpected future")
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := server.NewRestate().
		Bind(restate.Reflect(Greeter{}))

	// Connect blocks until ctx is cancelled (SIGINT/SIGTERM), then drains and closes.
	err := srv.Connect(ctx, "greeter")
	if err != nil {
		slog.Error("connect exited with error", "err", err.Error())
		os.Exit(1)
	}
}
