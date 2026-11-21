// Package tunnel serves a Restate SDK deployment over an outbound connection to
// Restate Cloud's tunnel servers. It backs server.Restate.Connect.
package tunnel

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// config holds the raw option inputs before validation and env fallback.
type config struct {
	region     string
	serversSRV string
	servers    []string

	environmentID    string
	authToken        string
	authTokenFile    string
	signingPublicKey string
	tunnelName       string
	workerID         string

	tlsConfig *tls.Config
	logger    *slog.Logger

	connectTimeout   time.Duration
	handshakeTimeout time.Duration
	reconnectInitial time.Duration
	reconnectMax     time.Duration
	drainGrace       time.Duration
	pingInterval     time.Duration
	pingTimeout      time.Duration
	resolveInterval  time.Duration

	// Derived by resolveConfig from the raw inputs above.
	serverTargets        []target
	maxConcurrentStreams uint32
}

// Option configures a Tunnel. Pass options to NewTunnel.
type Option func(*config)

// WithRegion selects Restate Cloud's tunnel servers, resolved via DNS as
// tunnel.<region>.restate.cloud. Env: RESTATE_INPROC_CLOUD_REGION. Set exactly
// one discovery source (WithRegion, WithServersSRV, or WithServers).
func WithRegion(region string) Option {
	return func(c *config) { c.region = region }
}

// WithServersSRV sets a raw DNS SRV name to resolve tunnel servers from (e.g.
// "tunnel.eu.restate.cloud"), instead of deriving it from a region. Set exactly
// one discovery source. Env: RESTATE_TUNNEL_SERVERS_SRV.
func WithServersSRV(name string) Option {
	return func(c *config) { c.serversSRV = name }
}

// WithServers sets explicit tunnel servers to dial, for development or
// self-hosting: "host:port" or "https://host:port" (TLS), or "http://host:port"
// (plaintext h2). Set exactly one discovery source. No environment fallback.
func WithServers(servers ...string) Option {
	return func(c *config) { c.servers = servers }
}

// WithEnvironment sets the Restate Cloud environment id (env_...) and the signing
// public key (publickeyv1_...) that verifies forwarded requests originate from
// that environment. Env: RESTATE_INPROC_ENVIRONMENT_ID, RESTATE_INPROC_SIGNING_PUBLIC_KEY.
func WithEnvironment(environmentID, signingPublicKey string) Option {
	return func(c *config) {
		c.environmentID = environmentID
		c.signingPublicKey = signingPublicKey
	}
}

// WithAuthToken sets the Restate Cloud API token presented on the handshake.
// Use this or WithAuthTokenFile; a literal token takes precedence over a token
// file. Env: RESTATE_AUTH_TOKEN.
func WithAuthToken(token string) Option {
	return func(c *config) { c.authToken = token }
}

// WithAuthTokenFile sets a path to a file whose contents are the API token; it is
// re-read (and trimmed) on every reconnect so rotations are picked up. Env:
// RESTATE_INPROC_AUTH_TOKEN_FILE.
func WithAuthTokenFile(path string) Option {
	return func(c *config) { c.authTokenFile = path }
}

// WithTunnelName sets the rendezvous/load-balancing key; replicas sharing a name
// are load-balanced by the tunnel server. Env: RESTATE_INPROC_TUNNEL_NAME.
func WithTunnelName(name string) Option {
	return func(c *config) { c.tunnelName = name }
}

// WithWorkerID sets an optional diagnostic id for this worker. Env:
// RESTATE_TUNNEL_WORKER_ID; when unset it defaults to a sanitized hostname
// ($HOSTNAME, else the OS hostname) plus a random suffix.
func WithWorkerID(id string) Option {
	return func(c *config) { c.workerID = id }
}

// WithTLS customizes the outbound TLS (custom roots, mTLS, servername). ALPN is
// always forced to h2. Nil uses the system roots. No environment fallback.
func WithTLS(cfg *tls.Config) Option {
	return func(c *config) { c.tlsConfig = cfg }
}

// WithLogger sets the logger for tunnel lifecycle logs; defaults to
// slog.Default(). No environment fallback.
func WithLogger(l *slog.Logger) Option {
	return func(c *config) { c.logger = l }
}

// WithReconnectBackoff tunes the jittered exponential reconnect backoff.
// Defaults: 10ms initial, 2m max. No environment fallback.
func WithReconnectBackoff(initial, max time.Duration) Option {
	return func(c *config) {
		c.reconnectInitial = initial
		c.reconnectMax = max
	}
}

// WithTimeouts tunes the per-attempt connect and handshake timeouts.
// Defaults: 5s each. No environment fallback.
func WithTimeouts(connect, handshake time.Duration) Option {
	return func(c *config) {
		c.connectTimeout = connect
		c.handshakeTimeout = handshake
	}
}

// WithDrainGrace sets how long in-flight invocations may finish during a drain
// before the connection is force-closed. Default: 2m. No environment fallback.
func WithDrainGrace(d time.Duration) Option {
	return func(c *config) { c.drainGrace = d }
}

// WithLivenessPing tunes the server-initiated HTTP/2 keepalive that detects a
// half-open peer: after the connection has been read-idle for interval the SDK
// sends a PING, and if it isn't acked within timeout the connection is closed and
// reconnected. Defaults: 75s interval, 10s timeout. No environment fallback.
func WithLivenessPing(interval, timeout time.Duration) Option {
	return func(c *config) {
		c.pingInterval = interval
		c.pingTimeout = timeout
	}
}

// WithResolveInterval sets how often DNS-SRV discovery re-resolves the tunnel
// server set to pick up added/removed nodes (one connection is kept per resolved
// IP). Ignored for explicit servers. Default: 30s. No environment fallback.
func WithResolveInterval(d time.Duration) Option {
	return func(c *config) { c.resolveInterval = d }
}

// HandlerFactory builds the SDK handler served through the tunnel. It is called
// once the configuration is resolved, with the environment's signing public key,
// which the handler must use to verify forwarded requests. The handler must run
// in bidirectional mode.
type HandlerFactory func(signingPublicKey string) (http.Handler, error)

// Tunnel configures and runs a tunnel. Create it with NewTunnel, then call Start
// (blocking) or Connect (for a handle). Options left unset fall back to
// RESTATE_CONNECT_KEY, then to the RESTATE_INPROC_* environment variables.
type Tunnel struct {
	newHandler HandlerFactory
	cfg        config
}

// NewTunnel creates a tunnel serving the handler built by newHandler, configured by opts.
func NewTunnel(newHandler HandlerFactory, opts ...Option) *Tunnel {
	t := &Tunnel{newHandler: newHandler}
	for _, o := range opts {
		o(&t.cfg)
	}
	return t
}

// Start connects the tunnel and blocks until ctx is cancelled or a fatal error
// stops the tunnel, then drains in-flight invocations and closes. Once
// // connected, it logs how to register the deployment (connect://<tunnel name>).
// It returns nil on a clean, ctx-driven shutdown, or the fatal error otherwise.
func (t *Tunnel) Start(ctx context.Context) error {
	conn, err := t.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.Ready(ctx); err != nil {
		if ctx.Err() != nil {
			return nil // shutdown requested before we connected
		}
		return err
	}
	// How to register the deployment is logged at INFO by the manager on first connect.

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), conn.m.cfg.drainGrace)
		defer cancel()
		_ = conn.Shutdown(shutdownCtx)
		return nil
	case <-conn.Done():
		return conn.Err()
	}
}

// Connect validates the configuration (with environment fallback), builds the
// handler with the environment's signing key, and starts
// managing the outbound connection in the background. It returns immediately;
// use Connection.Ready to wait for the first successful handshake. Most callers
// want Start instead.
func (t *Tunnel) Connect(ctx context.Context) (*Connection, error) {
	if t.newHandler == nil {
		return nil, fmt.Errorf("tunnel: handler factory must not be nil")
	}

	rc, err := resolveConfig(t.cfg)
	if err != nil {
		return nil, err
	}

	// The tunnel is always full-duplex HTTP/2, and the handler verifies forwarded
	// requests against the environment's signing key.
	handler, err := t.newHandler(rc.signingPublicKey)
	if err != nil {
		return nil, fmt.Errorf("tunnel: build SDK handler: %w", err)
	}

	m := newManager(ctx, rc, handler)
	rc.logger.Info(fmt.Sprintf("Connecting to Restate Cloud environment %s (%s)", rc.environmentID, regionLabel(rc)))
	go m.run()

	return &Connection{m: m}, nil
}

// Connection is a handle to a running tunnel. It is safe for concurrent use.
type Connection struct {
	m         *manager
	closeOnce sync.Once
}

// Ready blocks until the tunnel completes its first handshake, a fatal error
// occurs, the tunnel is closed, or ctx is done. It returns nil once established.
func (c *Connection) Ready(ctx context.Context) error {
	select {
	case <-c.m.readyCh:
		return c.m.readyErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Done is closed when the tunnel stops managing connections (fatal error, or
// after Close/Shutdown).
func (c *Connection) Done() <-chan struct{} { return c.m.doneCh }

// DeploymentURL returns the URL to register with `restate deployments register`
// (or that the operator registers for you). Empty until the first handshake.
func (c *Connection) DeploymentURL() string {
	c.m.mu.Lock()
	defer c.m.mu.Unlock()
	return c.m.deploymentURL
}

// TunnelName returns the tunnel name confirmed by the server, or "" before the
// first handshake.
func (c *Connection) TunnelName() string {
	c.m.mu.Lock()
	defer c.m.mu.Unlock()
	return c.m.learnedName
}

// ConnectionCount returns the number of successful handshakes so far.
func (c *Connection) ConnectionCount() int {
	c.m.mu.Lock()
	defer c.m.mu.Unlock()
	return c.m.connCount
}

// Err returns the fatal error that stopped the tunnel, or nil if it is still
// running or was stopped cleanly.
func (c *Connection) Err() error {
	c.m.mu.Lock()
	defer c.m.mu.Unlock()
	return c.m.fatalErr
}

// Shutdown stops reconnecting and drains in-flight invocations before closing.
// If ctx is done first, it falls back to an immediate close and returns ctx.Err().
func (c *Connection) Shutdown(ctx context.Context) error {
	c.m.beginShutdown()
	select {
	case <-c.m.doneCh:
		return nil
	case <-ctx.Done():
		c.m.cancel()
		<-c.m.doneCh
		return ctx.Err()
	}
}

// Close immediately stops the tunnel and releases its resources. Safe to call
// more than once.
func (c *Connection) Close() error {
	c.closeOnce.Do(func() {
		c.m.cancel()
		<-c.m.doneCh
	})
	return nil
}
