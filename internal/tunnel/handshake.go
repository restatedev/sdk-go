package tunnel

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// startTunnelPath is the control path the tunnel server opens as its first stream
// to establish the tunnel.
const startTunnelPath = "/_/start-tunnel"

// defaultHandshakeTimeout mirrors the tunnel server's own handshake deadline.
const defaultHandshakeTimeout = 5 * time.Second

// handshakeInfo is what the tunnel server tells us about the established tunnel.
type handshakeInfo struct {
	tunnelName string
	proxyURL   string
	tunnelURL  string
}

type handshakeOutcomeKind int

const (
	// handshakeOK: the tunnel is established.
	handshakeOK handshakeOutcomeKind = iota
	// handshakeFatal: a configuration error redialing cannot fix (unauthorized,
	// bad-tunnel-name, tunnel-name mismatch). Stops the whole tunnel.
	handshakeFatal
	// handshakeRetryable: transient (too-many-tunnels, timeout, malformed/missing
	// trailers, stream errors, unknown statuses). Redial with backoff.
	handshakeRetryable
)

type handshakeOutcome struct {
	kind   handshakeOutcomeKind
	info   handshakeInfo // set when kind == handshakeOK
	reason string        // set when kind != handshakeOK
}

func fatalOutcome(format string, args ...any) handshakeOutcome {
	return handshakeOutcome{kind: handshakeFatal, reason: fmt.Sprintf(format, args...)}
}

func retryableOutcome(format string, args ...any) handshakeOutcome {
	return handshakeOutcome{kind: handshakeRetryable, reason: fmt.Sprintf(format, args...)}
}

// handshakeCredentials are the credentials we present to the tunnel server.
type handshakeCredentials struct {
	authToken          string
	environmentID      string
	tunnelName         string
	tunnelWorkerID     string // stable-ish per SDK worker/process, for diagnostics
	tunnelConnectionID string // unique per h2 connection attempt, for diagnostics

	// supportsDrain advertises that we implement the /_/drain-tunnel handover:
	// on drain we open a replacement connection while this one finishes in-flight.
	supportsDrain bool
	// supportsClientDrain advertises that on shutdown we refuse raced streams with
	// the x-restate-tunnel-draining sentinel rather than dropping them, so the
	// server can trust that sentinel to deselect this connection.
	supportsClientDrain bool
}

// performHandshake runs the receiver side of the /_/start-tunnel exchange on its
// stream. The tunnel server (the HTTP/2 client on the role-flipped connection)
// opens GET /_/start-tunnel as its first stream, keeps the request body open, and:
//
//  1. We answer immediately with 200 whose response headers carry our credentials.
//  2. The server validates them, then completes the handshake by sending HTTP/2
//     trailers on its still-open request body:
//     tunnel-status: ok | unauthorized | bad-tunnel-name | too-many-tunnels
//     plus, on ok: proxy-url, tunnel-url, tunnel-name.
//
// It never returns until the outcome is known, the timeout elapses, or the stream
// closes. It does not itself tear down the connection; the caller acts on the
// outcome.
func performHandshake(w http.ResponseWriter, r *http.Request, creds handshakeCredentials, timeout time.Duration) handshakeOutcome {
	h := w.Header()
	h.Set("authorization", "Bearer "+creds.authToken)
	h.Set("environment-id", creds.environmentID)
	h.Set("tunnel-name", creds.tunnelName)
	h.Set("tunnel-worker-id", creds.tunnelWorkerID)
	h.Set("tunnel-connection-id", creds.tunnelConnectionID)
	if creds.supportsDrain {
		h.Set("supports-drain", "true")
	}
	if creds.supportsClientDrain {
		h.Set("supports-client-drain", "true")
	}
	w.WriteHeader(http.StatusOK)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Drain the (empty) request body so the trailers frame is delivered, then read
	// the trailers. We run it in a goroutine and race a timer + the request context
	// so a stalled server can't block us forever; on timeout/cancel the caller
	// closes the connection, which unblocks the read.
	done := make(chan handshakeOutcome, 1)
	go func() {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			done <- retryableOutcome("handshake stream error: %v", err)
			return
		}
		done <- classifyTrailers(r.Trailer, creds.tunnelName)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case out := <-done:
		return out
	case <-timer.C:
		return retryableOutcome("handshake trailers not received within %s", timeout)
	case <-r.Context().Done():
		return retryableOutcome("handshake stream closed before trailers")
	}
}

func classifyTrailers(trailer http.Header, requestedName string) handshakeOutcome {
	status := trailer.Get("tunnel-status")
	switch status {
	case "ok":
		name := trailer.Get("tunnel-name")
		proxyURL := trailer.Get("proxy-url")
		tunnelURL := trailer.Get("tunnel-url")
		if name == "" || proxyURL == "" || tunnelURL == "" {
			return retryableOutcome("handshake ok but proxy-url/tunnel-url/tunnel-name missing")
		}
		// We requested a specific name; the server must echo it. A different name
		// means our registration URL would not route here.
		if name != requestedName {
			return fatalOutcome("tunnel-name mismatch: requested %q, got %q", requestedName, name)
		}
		return handshakeOutcome{kind: handshakeOK, info: handshakeInfo{tunnelName: name, proxyURL: proxyURL, tunnelURL: tunnelURL}}
	case "unauthorized", "bad-tunnel-name":
		return fatalOutcome("tunnel-status: %s", status)
	case "":
		return retryableOutcome("tunnel-status: <missing>")
	default:
		return retryableOutcome("tunnel-status: %s", status)
	}
}
