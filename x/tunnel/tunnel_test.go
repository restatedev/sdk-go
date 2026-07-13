package tunnel

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/server"
	"github.com/restatedev/sdk-go/x/tunnel/internal/fakecloud"
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

func testTunnel(srv *server.Restate, cloud *fakecloud.Cloud, key *fakecloud.IdentityKey, name string) *Tunnel {
	return NewTunnel(srv,
		WithServers("http://"+cloud.Addr),
		WithEnvironment("env_test", key.PublicKey),
		WithAuthToken("tok"),
		WithTunnelName(name),
		WithLogger(testLogger),
		WithTimeouts(2*time.Second, 500*time.Millisecond),
		WithReconnectBackoff(time.Millisecond, 20*time.Millisecond),
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

// TestDrainingRefusesNewStreams checks the connection refuses forwarded streams
// while draining, with the deselection sentinel.
func TestDrainingRefusesNewStreams(t *testing.T) {
	c := newConnection(&memConn{}, handshakeCredentials{}, nil, testLogger, time.Second, time.Second)
	c.mu.Lock()
	c.draining = true
	c.mu.Unlock()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, forwardPrefix+"/invoke/Greeter/Greet", nil)
	req.RequestURI = forwardPrefix + "/invoke/Greeter/Greet"
	c.serveForwarded(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Equal(t, "true", rec.Header().Get(drainingHeader))
}
