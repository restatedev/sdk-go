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

// streamEndGrace bounds how long, once the hello request is canceled, we keep
// waiting for the body read to report the trailers that arrived before the end.
//
//	tunnel server (h2 client)                 SDK (h2 server)
//	HEADERS GET /_/start-tunnel ──────────▶
//	                               ◀────────── HEADERS 200 + credentials
//	HEADERS trailers + END_STREAM ────────▶    tunnel-status: ok | unauthorized | …
//	RST_STREAM CANCEL ────────────────────▶    (1) since restate-cloud#2195, if our
//	                                               half of the stream is still open
//
// TS and the Rust core (Python SDK) send END_STREAM with their 200, so the stream
// is closed once the trailers land: (1) never happens, and they read frames in
// order anyway.
//
// In Go the response ends only when the handler returns, and the handler is still
// waiting for the trailers, so (1) happens. net/http then cancels r.Context(), and
// the body read returns EOF with r.Trailer set. Godoc doesn't say which comes first:
//
//	r.Context().Done() ──┐
//	                     ├── race: deciding on the context alone drops a completed
//	Body read EOF ───────┘   handshake (0/100 against restate-cloud#2195)
//
// So a canceled context only means "the stream ended": wait up to streamEndGrace
// for the body read to say how.
const streamEndGrace = time.Second

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
// The server may reset the stream right after the trailers (valid HTTP/2), so a
// closed stream alone is not a failure: the trailers decide whenever they arrived.
//
// It returns once the outcome is known, the timeout elapses, or the stream has
// ended without trailers (after a short grace). It does not itself tear down the
// connection; the caller acts on the outcome.
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

	// Read the (empty) request body to EOF: per net/http, only then does r.Trailer
	// hold the trailers the server sent. The read runs in a goroutine so a timer
	// can bound it.
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

	var fallback handshakeOutcome
	select {
	case out := <-done:
		return out
	case <-timer.C:
		fallback = retryableOutcome("handshake trailers not received within %s", timeout)
	case <-r.Context().Done():
		// The request was canceled (stream reset or connection closed). That says
		// the stream ended, not how: the server may have sent the trailers right
		// before resetting it, which is valid HTTP/2 and what the tunnel server
		// does. Let the body read report what arrived, within a bounded grace.
		grace := time.NewTimer(streamEndGrace)
		defer grace.Stop()
		select {
		case out := <-done:
			return out
		case <-grace.C:
		case <-timer.C:
		}
		fallback = retryableOutcome("handshake stream closed before trailers")
	}

	// The read is still pending. Close the body to unblock it (net/http requires
	// Close to unblock a waiting Read), and wait for it: the handler must not
	// return while it is still reading the body.
	_ = r.Body.Close()
	if out := <-done; out.kind == handshakeOK {
		return out
	}
	return fallback
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
