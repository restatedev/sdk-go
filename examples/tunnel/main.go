// Command tunnel is an example of serving a Restate deployment over an outbound
// tunnel to Restate Cloud instead of listening for inbound connections.
//
// Running under the restate-operator in in-process tunnel mode, the operator
// injects the RESTATE_INPROC_* environment variables, so NewTunnel(srv).Start(ctx)
// with a mounted auth-token file is all you need. Locally, set the fields
// explicitly with the With* methods (or the same env vars) and register the
// deployment URL that Start logs on connect:
//
//	restate deployments register <deployment-url>
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
	"github.com/restatedev/sdk-go/x/tunnel"
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

	// Any option left unset falls back to the RESTATE_INPROC_* env vars the
	// operator injects. Start blocks until ctx is cancelled (SIGINT/SIGTERM),
	// then drains and closes.
	err := tunnel.NewTunnel(srv). // tunnel.WithRegion("us"),
		// tunnel.WithEnvironment("env_...", "publickeyv1_..."),
		// tunnel.WithAuthToken(os.Getenv("RESTATE_AUTH_TOKEN")),
		// tunnel.WithTunnelName("greeter-v1"),
		Start(ctx)
	if err != nil {
		slog.Error("tunnel exited with error", "err", err.Error())
		os.Exit(1)
	}
}
