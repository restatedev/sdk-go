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
	"strings"
	"sync"
	"time"
)

// minUptimeForBackoffReset is how long a connection must stay healthy before we
// trust it enough to reset the reconnect backoff — otherwise a server that
// accepts then immediately drops us would drive a zero-delay reconnect storm.
const minUptimeForBackoffReset = 5 * time.Second

// resolveRetryCap bounds the wait before retrying a failed target resolution.
const resolveRetryCap = 5 * time.Second

// errClosed is the readiness error when the tunnel is closed before it ever
// completes a handshake.
var errClosed = errors.New("tunnel: closed before ready")

// manager multi-homes a tunnel: it resolves the tunnel-server set and runs one
// connection "slot" per resolved server, reconciling the slot set as DNS changes
// and re-resolving periodically for SRV discovery. Each slot runs its own
// dial→handshake→serve→reconnect loop with jittered backoff. A fatal handshake
// outcome on ANY slot (bad credentials/name — shared across slots) stops the
// whole tunnel.
type manager struct {
	cfg        config
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
	fatalOnce    sync.Once

	mu            sync.Mutex
	connCount     int
	deploymentURL string
	learnedName   string
	fatalErr      error
	shuttingDown  bool
	slots         map[string]*slot
	activeConns   map[*connection]struct{}

	slotsWG sync.WaitGroup
}

// slot is a running per-server connection loop.
type slot struct {
	cancel context.CancelFunc
	target target
}

func newManager(ctx context.Context, cfg config, sdkHandler http.Handler) *manager {
	mctx, cancel := context.WithCancel(ctx)
	return &manager{
		cfg:         cfg,
		sdkHandler:  sdkHandler,
		logger:      cfg.logger,
		ctx:         mctx,
		cancel:      cancel,
		readyCh:     make(chan struct{}),
		shutdownCh:  make(chan struct{}),
		doneCh:      make(chan struct{}),
		slots:       make(map[string]*slot),
		activeConns: make(map[*connection]struct{}),
	}
}

// run drives the resolve/reconcile loop. It waits for all slots to settle before
// signalling done.
func (m *manager) run() {
	// Defers run LIFO: wait for slots, then finalize readiness, then signal done.
	defer close(m.doneCh)
	defer m.failReady(errClosed)
	defer m.slotsWG.Wait()

	for {
		if m.ctx.Err() != nil || m.isShuttingDown() {
			return
		}

		targets, err := m.resolveTargets()
		if err != nil {
			m.logger.Warn("Tunnel target resolution failed, retrying", "error", err)
			if !m.sleep(minDur(resolveRetryCap, m.cfg.resolveInterval)) {
				return
			}
			continue
		}
		if m.ctx.Err() != nil || m.isShuttingDown() {
			return
		}

		m.reconcile(targets)

		if !m.usesSRV() {
			return // explicit servers: fixed set — slots keep running (awaited by the deferred Wait)
		}
		if !m.sleep(m.cfg.resolveInterval) {
			return
		}
	}
}

// reconcile starts a slot for every newly-appeared target and tears down slots
// whose targets have vanished.
func (m *manager) reconcile(targets []target) {
	desired := make(map[string]target, len(targets))
	for _, t := range targets {
		desired[t.key()] = t
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.shuttingDown || m.ctx.Err() != nil {
		return
	}
	for key, t := range desired {
		if _, ok := m.slots[key]; !ok {
			m.startSlotLocked(key, t)
		}
	}
	for key, s := range m.slots {
		if _, ok := desired[key]; !ok {
			m.logger.Debug("Tunnel target no longer resolves, tearing down", "target", key)
			s.cancel()
		}
	}
}

// startSlotLocked launches a per-server reconnect loop. The caller holds m.mu.
func (m *manager) startSlotLocked(key string, t target) {
	sctx, scancel := context.WithCancel(m.ctx)
	s := &slot{cancel: scancel, target: t}
	m.slots[key] = s
	m.slotsWG.Add(1)
	m.logger.Debug("Tunnel starting connection", "target", key)
	go func() {
		defer m.slotsWG.Done()
		m.runSlot(sctx, t)
		m.mu.Lock()
		// Guard: the key may have vanished and re-appeared with a newer slot.
		if m.slots[key] == s {
			delete(m.slots, key)
		}
		m.mu.Unlock()
	}()
}

// runSlot is the per-server loop: dial → handshake → serve → classify → backoff → redial.
func (m *manager) runSlot(ctx context.Context, t target) {
	b := newBackoff(m.cfg.reconnectInitial, m.cfg.reconnectMax)
	for {
		if ctx.Err() != nil || m.isShuttingDown() || m.hasFatal() {
			return
		}

		start := time.Now()
		outcome, served := m.attempt(ctx, t)

		if outcome.kind == handshakeFatal {
			// Shared credentials: a fatal outcome on any slot stops the tunnel.
			m.setFatal(fmt.Errorf("tunnel: %s", outcome.reason))
			return
		}
		if served && time.Since(start) >= minUptimeForBackoffReset {
			b.reset()
		}
		if !served {
			m.logger.Debug("Tunnel connection attempt failed, will retry", "target", t.key(), "reason", outcome.reason)
		}

		if ctx.Err() != nil || m.isShuttingDown() {
			return
		}
		select {
		case <-time.After(b.next()):
		case <-ctx.Done():
			return
		case <-m.shutdownCh:
			return
		}
	}
}

// attempt runs one dial→handshake→serve cycle against a target. It returns the
// handshake outcome and whether the connection reached the serving state.
func (m *manager) attempt(ctx context.Context, t target) (handshakeOutcome, bool) {
	dialer := &net.Dialer{Timeout: m.cfg.connectTimeout, KeepAlive: 30 * time.Second}
	conn, err := dial(dialer, t, m.cfg.tlsConfig)
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

	c := newConnection(conn, creds, m.sdkHandler, m.cfg)

	serveDone := make(chan struct{})
	go func() {
		c.serve()
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
		m.onHandshakeOK(c, t, out.info)

		// Serve until the connection ends: an error, teardown (ctx), or a drain
		// (server drain, or client drain via beginShutdown -> beginDrain) closing it.
		select {
		case <-serveDone:
		case <-ctx.Done():
			c.close()
			<-serveDone
		}
		m.removeActive(c)
		return out, true

	case <-serveDone:
		return retryableOutcome("connection to %s closed before handshake", t), false
	case <-deadline.C:
		c.close()
		<-serveDone
		return retryableOutcome("handshake to %s not completed within deadline", t), false
	case <-ctx.Done():
		c.close()
		<-serveDone
		return retryableOutcome("context cancelled"), false
	}
}

func (m *manager) onHandshakeOK(c *connection, t target, info handshakeInfo) {
	depURL := computeDeploymentURL(info.proxyURL)

	m.mu.Lock()
	m.connCount++
	m.deploymentURL = depURL
	m.learnedName = info.tunnelName
	m.activeConns[c] = struct{}{}
	shuttingDown := m.shuttingDown
	m.mu.Unlock()

	// Surface the deployment URL at INFO the first time the tunnel comes up (the
	// thing you register); later per-slot connections are just DEBUG churn.
	if m.markReady() {
		m.logger.Info("Tunnel connected",
			"tunnelName", info.tunnelName,
			"deploymentURL", depURL)
	} else {
		m.logger.Debug("Tunnel connection established",
			"target", t.key(),
			"tunnelName", info.tunnelName,
			"deploymentURL", depURL)
	}

	// If shutdown was requested while we were mid-handshake, drain immediately.
	if shuttingDown {
		c.beginDrain()
	}
}

func (m *manager) removeActive(c *connection) {
	m.mu.Lock()
	delete(m.activeConns, c)
	m.mu.Unlock()
}

// resolveTargets produces the current tunnel-server set. Explicit servers are
// used verbatim; otherwise the SRV name (given or region-derived) is resolved to
// one target per resolved IP.
func (m *manager) resolveTargets() ([]target, error) {
	if len(m.cfg.serverTargets) > 0 {
		return m.cfg.serverTargets, nil
	}
	srvName := m.cfg.serversSRV
	if srvName == "" {
		srvName = "tunnel." + m.cfg.region + ".restate.cloud"
	}
	ctx, cancel := context.WithTimeout(m.ctx, m.cfg.connectTimeout)
	defer cancel()
	return resolveSRVTargets(ctx, net.DefaultResolver, srvName)
}

// usesSRV reports whether the target set comes from DNS (and must be re-resolved)
// rather than a fixed, explicit list.
func (m *manager) usesSRV() bool {
	return len(m.cfg.serverTargets) == 0
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

// sleep waits d, or returns false early if the tunnel is torn down or shutting down.
func (m *manager) sleep(d time.Duration) bool {
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-m.ctx.Done():
		return false
	case <-m.shutdownCh:
		return false
	}
}

// markReady closes the ready channel on the first successful handshake and
// reports whether this call was that first one.
func (m *manager) markReady() bool {
	first := false
	m.readyOnce.Do(func() {
		first = true
		close(m.readyCh)
	})
	return first
}

func (m *manager) failReady(err error) {
	m.readyOnce.Do(func() {
		m.readyErr = err
		close(m.readyCh)
	})
}

// setFatal records a non-retryable failure and tears the whole tunnel down.
func (m *manager) setFatal(err error) {
	m.fatalOnce.Do(func() {
		m.mu.Lock()
		m.fatalErr = err
		m.mu.Unlock()
		m.logger.Error("Tunnel stopped with a fatal error", "error", err)
		m.failReady(err)
		m.cancel() // cascade to every slot and the resolve loop
	})
}

func (m *manager) hasFatal() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.fatalErr != nil
}

func (m *manager) isShuttingDown() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.shuttingDown
}

// beginShutdown stops resolving/reconnecting and drains every live connection so
// in-flight invocations finish before the connections close.
func (m *manager) beginShutdown() {
	m.mu.Lock()
	m.shuttingDown = true
	conns := make([]*connection, 0, len(m.activeConns))
	for c := range m.activeConns {
		conns = append(conns, c)
	}
	m.mu.Unlock()

	m.shutdownOnce.Do(func() { close(m.shutdownCh) })

	for _, c := range conns {
		c.beginDrain()
	}
}

// computeDeploymentURL derives the URL to register with `restate deployments
// register` from the handshake's proxy URL: default the port to 9080, strip any
// trailing slash, then append the constant in-process destination segment.
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

func minDur(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
