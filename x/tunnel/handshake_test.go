package tunnel

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/server"
	"github.com/restatedev/sdk-go/x/tunnel/internal/fakecloud"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

const rawHelloTunnelName = "greeter-v1"

// rawHello plays the tunnel server's side of /_/start-tunnel with raw HTTP/2
// frames, so a test controls exactly which frames follow the SDK's response and
// in which write they reach the wire. That is what the real tunnel server does
// differently from fakecloud's http2.Transport: since restatedev/restate-cloud#2195
// it sends the result trailers and an RST_STREAM in the same write.
type rawHello struct {
	cloud net.Conn // the tunnel server's end
	conn  *connection
	enc   *hpack.Encoder
	hbuf  bytes.Buffer
}

// startRawHello serves a connection on one end of a loopback TCP pair, opens
// GET /_/start-tunnel from the other, and returns once the SDK has answered it.
func startRawHello(t *testing.T, handshakeTimeout time.Duration) *rawHello {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	cloud, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	sdk, err := ln.Accept()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = cloud.Close()
		_ = sdk.Close()
	})

	creds := handshakeCredentials{authToken: "tok", environmentID: "env_test", tunnelName: rawHelloTunnelName}
	c := newConnection(sdk, creds, http.NotFoundHandler(), config{logger: testLogger, handshakeTimeout: handshakeTimeout})
	go c.serve()

	h := openHello(t, cloud)
	h.conn = c
	return h
}

// openHello drives the tunnel server's end of an h2 connection: it opens
// GET /_/start-tunnel and returns once the SDK has answered it.
func openHello(t *testing.T, cloud net.Conn) *rawHello {
	t.Helper()
	h := &rawHello{cloud: cloud}
	h.enc = hpack.NewEncoder(&h.hbuf)

	fr := http2.NewFramer(cloud, cloud)
	fr.ReadMetaHeaders = hpack.NewDecoder(4096, nil)
	_, err := cloud.Write([]byte(http2.ClientPreface))
	require.NoError(t, err)
	require.NoError(t, fr.WriteSettings())
	require.NoError(t, fr.WriteHeaders(http2.HeadersFrameParam{
		StreamID: 1,
		BlockFragment: h.headerBlock(t,
			":method", "GET",
			":scheme", "http",
			":authority", "tunnel",
			":path", startTunnelPath,
			// Announced, as net/http only surfaces announced request trailers.
			"trailer", "tunnel-status, tunnel-name, proxy-url, tunnel-url",
		),
		EndHeaders: true,
	}))

	// Ack the SDK's SETTINGS and wait for its response to the hello.
	_ = cloud.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		f, err := fr.ReadFrame()
		require.NoError(t, err, "waiting for the SDK's response to /_/start-tunnel")
		switch f := f.(type) {
		case *http2.SettingsFrame:
			if !f.IsAck() {
				require.NoError(t, fr.WriteSettingsAck())
			}
		case *http2.MetaHeadersFrame:
			if f.StreamID == 1 {
				require.Equal(t, "200", f.PseudoValue("status"))
				_ = cloud.SetReadDeadline(time.Time{})
				// Keep reading so the SDK's writes never block; we don't need them.
				go func() {
					for {
						if _, err := fr.ReadFrame(); err != nil {
							return
						}
					}
				}()
				return h
			}
		}
	}
}

func (h *rawHello) headerBlock(t *testing.T, kv ...string) []byte {
	t.Helper()
	h.hbuf.Reset()
	for i := 0; i < len(kv); i += 2 {
		require.NoError(t, h.enc.WriteField(hpack.HeaderField{Name: kv[i], Value: kv[i+1]}))
	}
	return append([]byte(nil), h.hbuf.Bytes()...)
}

// sendInOneWrite encodes frames into a buffer and writes them with a single
// Write, so they reach the SDK together, as the tunnel server sends them.
func (h *rawHello) sendInOneWrite(t *testing.T, build func(fr *http2.Framer)) {
	t.Helper()
	var buf bytes.Buffer
	build(http2.NewFramer(&buf, nil))
	_, err := h.cloud.Write(buf.Bytes())
	require.NoError(t, err)
}

func (h *rawHello) trailers(t *testing.T, fr *http2.Framer, kv ...string) {
	t.Helper()
	require.NoError(t, fr.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      1,
		BlockFragment: h.headerBlock(t, kv...),
		EndStream:     true,
		EndHeaders:    true,
	}))
}

func (h *rawHello) outcome(t *testing.T, within time.Duration) handshakeOutcome {
	t.Helper()
	select {
	case out := <-h.conn.outcomeCh:
		return out
	case <-time.After(within):
		t.Fatalf("no handshake outcome within %s", within)
		return handshakeOutcome{}
	}
}

// Trailers followed by a reset of the hello stream are a completed handshake:
// the reset ends the stream, but the outcome is what the trailers say. Repeated
// because the bug this guards against was a race the reset usually won.
func TestHandshakeTrailersThenReset(t *testing.T) {
	for i := 0; i < 20; i++ {
		h := startRawHello(t, 2*time.Second)
		h.sendInOneWrite(t, func(fr *http2.Framer) {
			h.trailers(t, fr,
				"tunnel-status", "ok",
				"tunnel-name", rawHelloTunnelName,
				"proxy-url", "https://proxy.restate.cloud",
				"tunnel-url", "https://tunnel.restate.cloud",
			)
			require.NoError(t, fr.WriteRSTStream(1, http2.ErrCodeCancel))
		})
		out := h.outcome(t, 5*time.Second)
		require.Equal(t, handshakeOK, out.kind, "attempt %d: %s", i, out.reason)
		require.Equal(t, rawHelloTunnelName, out.info.tunnelName)
	}
}

// A rejection followed by a reset stays fatal: it must not turn into a retry.
func TestHandshakeUnauthorizedThenReset(t *testing.T) {
	for i := 0; i < 20; i++ {
		h := startRawHello(t, 2*time.Second)
		h.sendInOneWrite(t, func(fr *http2.Framer) {
			h.trailers(t, fr, "tunnel-status", "unauthorized")
			require.NoError(t, fr.WriteRSTStream(1, http2.ErrCodeCancel))
		})
		out := h.outcome(t, 5*time.Second)
		require.Equal(t, handshakeFatal, out.kind, "attempt %d: %s", i, out.reason)
	}
}

// A reset without trailers is a retryable failure, reported promptly rather
// than after the full handshake timeout.
func TestHandshakeResetWithoutTrailers(t *testing.T) {
	const timeout = 10 * time.Second
	h := startRawHello(t, timeout)
	start := time.Now()
	h.sendInOneWrite(t, func(fr *http2.Framer) {
		require.NoError(t, fr.WriteRSTStream(1, http2.ErrCodeCancel))
	})
	out := h.outcome(t, timeout)
	require.Equal(t, handshakeRetryable, out.kind, out.reason)
	require.Less(t, time.Since(start), streamEndGrace+time.Second)
}

// The connection dying mid-handshake is a retryable failure, reported promptly.
func TestHandshakeConnectionClosed(t *testing.T) {
	const timeout = 10 * time.Second
	h := startRawHello(t, timeout)
	start := time.Now()
	require.NoError(t, h.cloud.Close())
	out := h.outcome(t, timeout)
	require.Equal(t, handshakeRetryable, out.kind, out.reason)
	require.Less(t, time.Since(start), streamEndGrace+time.Second)
}

// A server that never completes the handshake hits the handshake timeout.
func TestHandshakeTrailersTimeout(t *testing.T) {
	const timeout = 300 * time.Millisecond
	start := time.Now()
	h := startRawHello(t, timeout)
	out := h.outcome(t, 5*time.Second)
	require.Equal(t, handshakeRetryable, out.kind, out.reason)
	require.Contains(t, out.reason, "not received within")
	require.GreaterOrEqual(t, time.Since(start), timeout)
}

// A rejection followed by a reset and a closed connection (what the tunnel
// server does) stops the tunnel on that attempt: the closed connection must not
// win over the outcome and trigger another dial against the server.
func TestRejectionThenCloseStopsWithoutRedial(t *testing.T) {
	key, err := fakecloud.GenerateIdentityKey()
	require.NoError(t, err)
	srv := server.NewRestate().Bind(restate.Reflect(Greeter{}))

	for i := 0; i < 20; i++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		tun, err := NewTunnel(srv,
			WithServers("http://"+ln.Addr().String()),
			WithEnvironment("env_test", key.PublicKey),
			WithAuthToken("tok"),
			WithTunnelName(rawHelloTunnelName),
			WithLogger(testLogger),
			WithTimeouts(time.Second, time.Second),
			WithReconnectBackoff(time.Millisecond, time.Millisecond),
		).Connect(context.Background())
		require.NoError(t, err)

		cloud, err := ln.Accept()
		require.NoError(t, err)
		h := openHello(t, cloud)
		h.sendInOneWrite(t, func(fr *http2.Framer) {
			h.trailers(t, fr, "tunnel-status", "unauthorized")
			require.NoError(t, fr.WriteRSTStream(1, http2.ErrCodeCancel))
		})
		require.NoError(t, cloud.Close())

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = tun.Ready(ctx)
		cancel()
		require.ErrorContains(t, err, "unauthorized", "attempt %d", i)

		require.NoError(t, ln.(*net.TCPListener).SetDeadline(time.Now().Add(200*time.Millisecond)))
		if c, err := ln.Accept(); err == nil {
			_ = c.Close()
			t.Fatalf("attempt %d: redialed after a fatal rejection", i)
		}
		_ = tun.Close()
		_ = ln.Close()
	}
}
