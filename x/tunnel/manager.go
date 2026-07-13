package tunnel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// minUptimeForBackoffReset is how long a connection must stay healthy before we
// trust it enough to reset the reconnect backoff — otherwise a server that
// accepts then immediately drops (or drain-spams) us would drive a zero-delay
// reconnect storm.
const minUptimeForBackoffReset = 5 * time.Second

// errClosed is the readiness error when the tunnel is closed before it ever
// completes a handshake.
var errClosed = errors.New("tunnel: closed before ready")

// manager owns the reconnect loop for a tunnel: it resolves targets, dials,
// drives the handshake, serves while the connection is up, and reconnects with
// jittered backoff. A fatal handshake outcome (bad credentials/name) stops the
// whole loop.
type manager struct {
	cfg        resolvedConfig
	sdkHandler http.Handler
	logger     *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc

	readyOnce sync.Once
	readyCh   chan struct{}
	readyErr  error

	shutdownOnce sync.Once
	shutdownCh   chan struct{}
	doneCh       chan struct{}

	mu            sync.Mutex
	connCount     int
	deploymentURL string
	learnedName   string
	fatalErr      error
	activeConn    *connection
	shuttingDown  bool

	targets []target
	nextIdx int
}

func newManager(ctx context.Context, cfg resolvedConfig, sdkHandler http.Handler) *manager {
	mctx, cancel := context.WithCancel(ctx)
	return &manager{
		cfg:        cfg,
		sdkHandler: sdkHandler,
		logger:     cfg.logger,
		ctx:        mctx,
		cancel:     cancel,
		readyCh:    make(chan struct{}),
		shutdownCh: make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
}

func (m *manager) run() {
	defer close(m.doneCh)
	defer m.failReady(errClosed)

	b := newBackoff(m.cfg.reconnectInitial, m.cfg.reconnectMax)

	for {
		if m.ctx.Err() != nil || m.isShuttingDown() {
			return
		}

		start := time.Now()
		outcome, served := m.attempt()

		if outcome.kind == handshakeFatal {
			m.setFatal(fmt.Errorf("tunnel: %s", outcome.reason))
			return
		}
		if served && time.Since(start) >= minUptimeForBackoffReset {
			b.reset()
		}
		if !served {
			m.logger.Debug("Tunnel connection attempt failed, will retry", "reason", outcome.reason)
		}

		if m.ctx.Err() != nil || m.isShuttingDown() {
			return
		}

		select {
		case <-time.After(b.next()):
		case <-m.ctx.Done():
			return
		case <-m.shutdownCh:
			return
		}
	}
}

// attempt runs one dial→handshake→serve cycle. It returns the handshake outcome
// and whether the connection reached the serving state.
func (m *manager) attempt() (handshakeOutcome, bool) {
	t, err := m.pickTarget()
	if err != nil {
		return retryableOutcome("resolve targets: %v", err), false
	}

	dialer := &net.Dialer{Timeout: m.cfg.connectTimeout, KeepAlive: 30 * time.Second}
	conn, err := dial(dialer, t, m.cfg.tlsConfig, m.cfg.connectTimeout)
	if err != nil {
		return retryableOutcome("dial %s: %v", t, err), false
	}

	// Restate Cloud sends the handshake trailers unannounced; the SDK's HTTP/2
	// server would drop them, so inject the "Trailer" announcement before serving.
	conn, err = injectTrailerAnnouncement(conn, m.cfg.connectTimeout+m.cfg.handshakeTimeout)
	if err != nil {
		_ = conn.Close()
		return retryableOutcome("prepare connection to %s: %v", t, err), false
	}

	creds, err := m.buildCreds()
	if err != nil {
		_ = conn.Close()
		return retryableOutcome("credentials: %v", err), false
	}

	c := newConnection(conn, creds, m.sdkHandler, m.logger, m.cfg.handshakeTimeout, m.cfg.drainGrace)

	serveDone := make(chan struct{})
	go func() {
		c.serve(m.cfg.maxConcurrentStreams)
		close(serveDone)
	}()

	// Bound the wait for the server to open /_/start-tunnel and finish the
	// handshake; performHandshake also has its own trailer timeout once started.
	deadline := time.NewTimer(m.cfg.connectTimeout + m.cfg.handshakeTimeout)
	defer deadline.Stop()

	select {
	case out := <-c.outcomeCh:
		if out.kind != handshakeOK {
			c.close()
			<-serveDone
			return out, false
		}
		m.onHandshakeOK(c, out.info)

		// Serve until the connection ends (drain, error, or teardown).
		select {
		case <-serveDone:
		case <-m.ctx.Done():
			c.close()
			<-serveDone
		}
		m.clearActive(c)
		return out, true

	case <-serveDone:
		return retryableOutcome("connection closed before handshake"), false
	case <-deadline.C:
		c.close()
		<-serveDone
		return retryableOutcome("handshake not completed within deadline"), false
	case <-m.ctx.Done():
		c.close()
		<-serveDone
		return retryableOutcome("context cancelled"), false
	}
}

func (m *manager) onHandshakeOK(c *connection, info handshakeInfo) {
	m.mu.Lock()
	m.connCount++
	m.deploymentURL = computeDeploymentURL(info.proxyURL)
	m.learnedName = info.tunnelName
	m.activeConn = c
	shuttingDown := m.shuttingDown
	m.mu.Unlock()

	m.markReady()
	m.logger.Info("Tunnel connected",
		"tunnelName", info.tunnelName,
		"deploymentURL", m.deploymentURL)

	// If shutdown was requested while we were mid-handshake, drain immediately.
	if shuttingDown {
		c.beginDrain()
	}
}

func (m *manager) clearActive(c *connection) {
	m.mu.Lock()
	if m.activeConn == c {
		m.activeConn = nil
	}
	m.mu.Unlock()
}

// pickTarget returns the next target to dial, (re-)resolving the target list when
// it is exhausted.
func (m *manager) pickTarget() (target, error) {
	if m.nextIdx >= len(m.targets) {
		ts, err := m.resolveTargets()
		if err != nil {
			return target{}, err
		}
		if len(ts) == 0 {
			return target{}, errors.New("no tunnel servers resolved")
		}
		m.targets = ts
		m.nextIdx = 0
	}
	t := m.targets[m.nextIdx]
	m.nextIdx++
	return t, nil
}

// resolveTargets produces the tunnel servers to dial. Explicit servers are used
// verbatim; a region is resolved via DNS SRV (tunnel.<region>.restate.cloud),
// falling back to the A record at :9080 if SRV yields nothing.
func (m *manager) resolveTargets() ([]target, error) {
	if len(m.cfg.servers) > 0 {
		return m.cfg.servers, nil
	}

	srvName := m.cfg.serversSRV
	if srvName == "" {
		srvName = "tunnel." + m.cfg.region + ".restate.cloud"
	}
	ctx, cancel := context.WithTimeout(m.ctx, m.cfg.connectTimeout)
	defer cancel()

	_, addrs, err := net.DefaultResolver.LookupSRV(ctx, "", "", srvName)
	if err == nil && len(addrs) > 0 {
		targets := make([]target, 0, len(addrs))
		for _, a := range addrs {
			host := strings.TrimSuffix(a.Target, ".")
			targets = append(targets, target{
				address: net.JoinHostPort(host, strconv.Itoa(int(a.Port))),
				// SNI uses the SRV query name, not the per-node target host.
				serverName: srvName,
			})
		}
		return targets, nil
	}

	// Fall back to a plain A record on the conventional tunnel port.
	return []target{{address: net.JoinHostPort(srvName, "9080"), serverName: srvName}}, nil
}

func (m *manager) buildCreds() (handshakeCredentials, error) {
	// A literal token wins; otherwise read the token file (re-read every reconnect
	// so rotations are picked up).
	token := m.cfg.authToken
	if token == "" && m.cfg.authTokenFile != "" {
		b, err := os.ReadFile(m.cfg.authTokenFile)
		if err != nil {
			return handshakeCredentials{}, err
		}
		token = strings.TrimSpace(string(b))
		if token == "" {
			return handshakeCredentials{}, fmt.Errorf("auth token file %s is empty", m.cfg.authTokenFile)
		}
	}

	return handshakeCredentials{
		authToken:           token,
		environmentID:       m.cfg.environmentID,
		tunnelName:          m.cfg.tunnelName,
		tunnelWorkerID:      m.cfg.workerID,
		tunnelConnectionID:  newConnectionID(),
		supportsDrain:       true,
		supportsClientDrain: true,
	}, nil
}

func (m *manager) markReady() {
	m.readyOnce.Do(func() { close(m.readyCh) })
}

func (m *manager) failReady(err error) {
	m.readyOnce.Do(func() {
		m.readyErr = err
		close(m.readyCh)
	})
}

func (m *manager) setFatal(err error) {
	m.mu.Lock()
	m.fatalErr = err
	m.mu.Unlock()
	m.logger.Error("Tunnel stopped with a fatal error", "error", err)
	m.failReady(err)
}

func (m *manager) isShuttingDown() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.shuttingDown
}

// beginShutdown stops reconnecting and drains the active connection so in-flight
// invocations finish before it closes.
func (m *manager) beginShutdown() {
	m.mu.Lock()
	m.shuttingDown = true
	c := m.activeConn
	m.mu.Unlock()

	m.shutdownOnce.Do(func() { close(m.shutdownCh) })

	if c != nil {
		c.beginDrain()
	}
}

// computeDeploymentURL derives the URL to register with `restate dep register`
// from the handshake's proxy URL: default the port to 9080, strip any trailing
// slash, then append the constant in-process destination segment.
func computeDeploymentURL(proxyURL string) string {
	u, err := url.Parse(proxyURL)
	if err != nil || u.Host == "" {
		return ""
	}
	if u.Port() == "" {
		u.Host = net.JoinHostPort(u.Hostname(), "9080")
	}
	base := strings.TrimSuffix(u.String(), "/")
	return base + "/http/in-process/9080/"
}
