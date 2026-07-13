package tunnel

import (
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	healthPath      = "/_/health"
	drainTunnelPath = "/_/drain-tunnel"

	// drainingHeader is the sentinel we set on forwarded streams we refuse while
	// draining, so the server deselects this connection instead of treating the
	// refusal as a failure.
	drainingHeader = "x-restate-tunnel-draining"
)

// connection is one dial→handshake→serve cycle. It implements http.Handler: the
// role-flipped HTTP/2 server (see connection.serve) dispatches every stream the
// tunnel server opens to ServeHTTP — the handshake, the control paths, and the
// forwarded invocations (which, after the path prefix is stripped, are delegated
// to the SDK handler).
type connection struct {
	netConn    net.Conn
	creds      handshakeCredentials
	sdkHandler http.Handler
	logger     *slog.Logger

	handshakeTimeout time.Duration
	drainGrace       time.Duration

	// outcome is published exactly once, when the handshake resolves.
	outcomeOnce sync.Once
	outcomeCh   chan handshakeOutcome

	mu        sync.Mutex
	draining  bool
	inflight  int
	drainedCh chan struct{} // closed when draining && inflight == 0
	closeOnce sync.Once
}

func newConnection(conn net.Conn, creds handshakeCredentials, sdkHandler http.Handler, logger *slog.Logger, handshakeTimeout, drainGrace time.Duration) *connection {
	return &connection{
		netConn:          conn,
		creds:            creds,
		sdkHandler:       sdkHandler,
		logger:           logger,
		handshakeTimeout: handshakeTimeout,
		drainGrace:       drainGrace,
		outcomeCh:        make(chan handshakeOutcome, 1),
		drainedCh:        make(chan struct{}),
	}
}

func (c *connection) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case startTunnelPath:
		out := performHandshake(w, r, c.creds, c.handshakeTimeout)
		c.publishOutcome(out)
		return
	case healthPath:
		w.WriteHeader(http.StatusOK)
		return
	case drainTunnelPath:
		w.WriteHeader(http.StatusOK)
		c.beginDrain()
		return
	}

	c.serveForwarded(w, r)
}

func (c *connection) serveForwarded(w http.ResponseWriter, r *http.Request) {
	tail, ok := forwardedTail(r.RequestURI)
	if !ok {
		http.Error(w, "tunnel: malformed forwarded path", http.StatusBadRequest)
		return
	}

	// Refuse new streams while draining, so the server deselects us rather than
	// dropping raced invocations.
	if !c.inflightStart() {
		w.Header().Set(drainingHeader, "true")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	defer c.inflightEnd()

	// The SDK handler routes and verifies the identity JWT off RequestURI, so
	// rewrite it (and URL) to the stripped, service-relative tail.
	r.RequestURI = tail
	if u, err := url.ParseRequestURI(tail); err == nil {
		r.URL = u
	}

	c.sdkHandler.ServeHTTP(w, r)
}

func (c *connection) publishOutcome(out handshakeOutcome) {
	c.outcomeOnce.Do(func() { c.outcomeCh <- out })
}

// inflightStart registers a new forwarded invocation, unless we're draining.
func (c *connection) inflightStart() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.draining {
		return false
	}
	c.inflight++
	return true
}

func (c *connection) inflightEnd() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.inflight--
	if c.draining && c.inflight == 0 {
		c.signalDrainedLocked()
	}
}

// signalDrainedLocked closes drainedCh once; callers must hold c.mu.
func (c *connection) signalDrainedLocked() {
	select {
	case <-c.drainedCh:
		// already closed
	default:
		close(c.drainedCh)
	}
}

// beginDrain stops accepting new forwarded streams and closes the connection once
// in-flight invocations finish or the drain grace elapses. Idempotent.
func (c *connection) beginDrain() {
	c.mu.Lock()
	if c.draining {
		c.mu.Unlock()
		return
	}
	c.draining = true
	if c.inflight == 0 {
		c.signalDrainedLocked()
	}
	c.mu.Unlock()

	go func() {
		timer := time.NewTimer(c.drainGrace)
		defer timer.Stop()
		select {
		case <-c.drainedCh:
		case <-timer.C:
			c.logger.Warn("Tunnel drain grace elapsed with in-flight invocations still running; closing connection")
		}
		c.close()
	}()
}

// close tears down the underlying connection, which makes the role-flipped h2
// server (serve) return. Idempotent.
func (c *connection) close() {
	c.closeOnce.Do(func() { _ = c.netConn.Close() })
}
