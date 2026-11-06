package tunnel_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/internal/tunnel"
	"github.com/restatedev/sdk-go/internal/tunnel/internal/fakecloud"
	"github.com/restatedev/sdk-go/server"
	"github.com/stretchr/testify/require"
)

const forwardPrefix = "/http/in-process/9080"

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// Greeter is a trivial service used to exercise discovery over the tunnel.
type Greeter struct{}

func (Greeter) Greet(ctx restate.Context, name string) (string, error) {
	return "hi " + name, nil
}

func okTrailers(name string) map[string]string {
	return map[string]string{
		"tunnel-status": "ok",
		"tunnel-name":   name,
		"proxy-url":     "https://proxy.restate.cloud",
		"tunnel-url":    "https://tunnel.restate.cloud",
	}
}

// handlerOf serves srv through the tunnel, as server.Restate.Connect does.
func handlerOf(srv *server.Restate) tunnel.HandlerFactory {
	return func(signingPublicKey string) (http.Handler, error) {
		return srv.Bidirectional(true).WithIdentityV1(signingPublicKey).Handler()
	}
}

func testTunnel(srv *server.Restate, cloud *fakecloud.Cloud, key *fakecloud.IdentityKey, name string) *tunnel.Tunnel {
	return tunnel.NewTunnel(handlerOf(srv),
		tunnel.WithServers("http://"+cloud.Addr),
		tunnel.WithEnvironment("env_test", key.PublicKey),
		tunnel.WithAuthToken("tok"),
		tunnel.WithTunnelName(name),
		tunnel.WithLogger(testLogger),
		tunnel.WithTimeouts(2*time.Second, 500*time.Millisecond),
		tunnel.WithReconnectBackoff(time.Millisecond, 20*time.Millisecond),
	)
}

func TestHandshakeAndDeploymentURL(t *testing.T) {
	key, err := fakecloud.GenerateIdentityKey()
	require.NoError(t, err)

	cloud, err := fakecloud.Start(nil, func(int) map[string]string { return okTrailers("greeter-v1") })
	require.NoError(t, err)
	defer cloud.Close()

	srv := server.NewRestate().Bind(restate.Reflect(Greeter{}))
	tun, err := testTunnel(srv, cloud, key, "greeter-v1").Connect(context.Background())
	require.NoError(t, err)
	defer tun.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tun.Ready(ctx))

	require.Equal(t, "https://proxy.restate.cloud:9080/http/in-process/9080/", tun.DeploymentURL())
	require.Equal(t, "greeter-v1", tun.TunnelName())
	require.GreaterOrEqual(t, tun.ConnectionCount(), 1)

	// The credentials we presented on the handshake.
	conn, err := cloud.WaitForConnection(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "Bearer tok", conn.Creds.Get("authorization"))
	require.Equal(t, "env_test", conn.Creds.Get("environment-id"))
	require.Equal(t, "greeter-v1", conn.Creds.Get("tunnel-name"))
	require.NotEmpty(t, conn.Creds.Get("tunnel-worker-id"))
	require.NotEmpty(t, conn.Creds.Get("tunnel-connection-id"))
	require.Equal(t, "true", conn.Creds.Get("supports-drain"))
}

func TestForwardedDiscoveryWithIdentity(t *testing.T) {
	key, err := fakecloud.GenerateIdentityKey()
	require.NoError(t, err)

	cloud, err := fakecloud.Start(nil, func(int) map[string]string { return okTrailers("greeter-v1") })
	require.NoError(t, err)
	defer cloud.Close()

	srv := server.NewRestate().Bind(restate.Reflect(Greeter{}))
	tun, err := testTunnel(srv, cloud, key, "greeter-v1").Connect(context.Background())
	require.NoError(t, err)
	defer tun.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tun.Ready(ctx))

	conn, err := cloud.WaitForConnection(ctx, 0)
	require.NoError(t, err)

	// A signed /discover forwarded through the tunnel: exercises path stripping,
	// identity verification (aud == stripped path), and handler reuse.
	jwt, err := key.Sign("/discover")
	require.NoError(t, err)
	resp, err := conn.Roundtrip(http.MethodGet, forwardPrefix+"/discover", http.Header{
		"X-Restate-Signature-Scheme": {"v1"},
		"X-Restate-Jwt-V1":           {jwt},
		"accept":                     {"application/vnd.restate.endpointmanifest.v2+json"},
	}, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Status)
	require.Contains(t, string(resp.Body), "Greeter")

	// Unsigned request is rejected by the reused identity check.
	resp, err = conn.Roundtrip(http.MethodGet, forwardPrefix+"/discover", nil, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.Status)

	// Control path handled locally.
	resp, err = conn.Roundtrip(http.MethodGet, "/_/health", nil, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Status)

	// A path without the forwarded prefix is rejected before the SDK handler.
	resp, err = conn.Roundtrip(http.MethodGet, "/bogus/host/notaport/discover", nil, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.Status)
}

func TestFatalHandshakeStopsTunnel(t *testing.T) {
	key, err := fakecloud.GenerateIdentityKey()
	require.NoError(t, err)

	cloud, err := fakecloud.Start(nil, func(int) map[string]string {
		return map[string]string{"tunnel-status": "unauthorized"}
	})
	require.NoError(t, err)
	defer cloud.Close()

	srv := server.NewRestate().Bind(restate.Reflect(Greeter{}))
	tun, err := testTunnel(srv, cloud, key, "greeter-v1").Connect(context.Background())
	require.NoError(t, err)
	defer tun.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = tun.Ready(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unauthorized")
	require.Error(t, tun.Err())
}

func TestRetryableThenSuccess(t *testing.T) {
	key, err := fakecloud.GenerateIdentityKey()
	require.NoError(t, err)

	cloud, err := fakecloud.Start(nil, func(index int) map[string]string {
		if index == 0 {
			return map[string]string{"tunnel-status": "too-many-tunnels"} // retryable
		}
		return okTrailers("greeter-v1")
	})
	require.NoError(t, err)
	defer cloud.Close()

	srv := server.NewRestate().Bind(restate.Reflect(Greeter{}))
	tun, err := testTunnel(srv, cloud, key, "greeter-v1").Connect(context.Background())
	require.NoError(t, err)
	defer tun.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tun.Ready(ctx))
	require.GreaterOrEqual(t, cloud.ConnectionCount(), 2)
	require.Equal(t, 1, tun.ConnectionCount())
}

func TestStartBlocksAndShutsDownOnCancel(t *testing.T) {
	key, err := fakecloud.GenerateIdentityKey()
	require.NoError(t, err)

	cloud, err := fakecloud.Start(nil, func(int) map[string]string { return okTrailers("greeter-v1") })
	require.NoError(t, err)
	defer cloud.Close()

	srv := server.NewRestate().Bind(restate.Reflect(Greeter{}))
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- testTunnel(srv, cloud, key, "greeter-v1").Start(ctx) }()

	// Wait for the tunnel to connect, then cancel and expect a clean return.
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	_, err = cloud.WaitForConnection(waitCtx, 0)
	require.NoError(t, err)

	cancel()
	select {
	case err := <-done:
		require.NoError(t, err) // clean, ctx-driven shutdown
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after ctx cancel")
	}
}

func TestStartReturnsFatalError(t *testing.T) {
	key, err := fakecloud.GenerateIdentityKey()
	require.NoError(t, err)

	cloud, err := fakecloud.Start(nil, func(int) map[string]string {
		return map[string]string{"tunnel-status": "unauthorized"}
	})
	require.NoError(t, err)
	defer cloud.Close()

	srv := server.NewRestate().Bind(restate.Reflect(Greeter{}))
	err = testTunnel(srv, cloud, key, "greeter-v1").Start(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "unauthorized")
}

func TestGracefulShutdown(t *testing.T) {
	key, err := fakecloud.GenerateIdentityKey()
	require.NoError(t, err)

	cloud, err := fakecloud.Start(nil, func(int) map[string]string { return okTrailers("greeter-v1") })
	require.NoError(t, err)
	defer cloud.Close()

	srv := server.NewRestate().Bind(restate.Reflect(Greeter{}))
	tun, err := testTunnel(srv, cloud, key, "greeter-v1").Connect(context.Background())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tun.Ready(ctx))

	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	require.NoError(t, tun.Shutdown(shutdownCtx))
}

// TestMultiHoming checks that one connection is opened per resolved server.
func TestMultiHoming(t *testing.T) {
	key, err := fakecloud.GenerateIdentityKey()
	require.NoError(t, err)

	cloud1, err := fakecloud.Start(nil, func(int) map[string]string { return okTrailers("t") })
	require.NoError(t, err)
	defer cloud1.Close()
	cloud2, err := fakecloud.Start(nil, func(int) map[string]string { return okTrailers("t") })
	require.NoError(t, err)
	defer cloud2.Close()

	srv := server.NewRestate().Bind(restate.Reflect(Greeter{}))
	tun, err := tunnel.NewTunnel(handlerOf(srv),
		tunnel.WithServers("http://"+cloud1.Addr, "http://"+cloud2.Addr),
		tunnel.WithEnvironment("env_test", key.PublicKey),
		tunnel.WithAuthToken("tok"),
		tunnel.WithTunnelName("t"),
		tunnel.WithLogger(testLogger),
		tunnel.WithTimeouts(2*time.Second, 500*time.Millisecond),
		tunnel.WithReconnectBackoff(time.Millisecond, 20*time.Millisecond),
	).Connect(context.Background())
	require.NoError(t, err)
	defer tun.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tun.Ready(ctx))

	// One slot per server: both servers get a connection.
	_, err = cloud1.WaitForConnection(ctx, 0)
	require.NoError(t, err)
	_, err = cloud2.WaitForConnection(ctx, 0)
	require.NoError(t, err)
}

// TestLivenessPingReconnectsHalfOpen checks the ping watchdog tears down a
// half-open connection and reconnects.
func TestLivenessPingReconnectsHalfOpen(t *testing.T) {
	key, err := fakecloud.GenerateIdentityKey()
	require.NoError(t, err)

	cloud, err := fakecloud.Start(nil, func(int) map[string]string { return okTrailers("t") })
	require.NoError(t, err)
	defer cloud.Close()
	// The first connection goes silent right after handshaking; its PINGs won't be
	// acked, so the watchdog must tear it down and reconnect.
	cloud.SetHalfOpen(func(index int) bool { return index == 0 })

	srv := server.NewRestate().Bind(restate.Reflect(Greeter{}))
	tun, err := tunnel.NewTunnel(handlerOf(srv),
		tunnel.WithServers("http://"+cloud.Addr),
		tunnel.WithEnvironment("env_test", key.PublicKey),
		tunnel.WithAuthToken("tok"),
		tunnel.WithTunnelName("t"),
		tunnel.WithLogger(testLogger),
		tunnel.WithTimeouts(2*time.Second, 500*time.Millisecond),
		tunnel.WithReconnectBackoff(time.Millisecond, 20*time.Millisecond),
		tunnel.WithLivenessPing(100*time.Millisecond, 100*time.Millisecond),
	).Connect(context.Background())
	require.NoError(t, err)
	defer tun.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, tun.Ready(ctx))

	// Connection 0 is half-open; the watchdog reconnects to a healthy connection 1.
	_, err = cloud.WaitForConnection(ctx, 1)
	require.NoError(t, err)
}
