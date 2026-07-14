package tunnel

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"
)

// Environment variables that fill unset options. The RESTATE_INPROC_* names are
// the ones the restate-operator injects in in-process mode; RESTATE_AUTH_TOKEN
// and RESTATE_TUNNEL_SERVERS_SRV match the standalone restate-cloud-tunnel-client.
// Explicitly-set options always take precedence over the environment.
const (
	envTunnelName       = "RESTATE_INPROC_TUNNEL_NAME"
	envEnvironmentID    = "RESTATE_INPROC_ENVIRONMENT_ID"
	envCloudRegion      = "RESTATE_INPROC_CLOUD_REGION"
	envSigningPublicKey = "RESTATE_INPROC_SIGNING_PUBLIC_KEY"
	envAuthToken        = "RESTATE_AUTH_TOKEN"
	envAuthTokenFile    = "RESTATE_INPROC_AUTH_TOKEN_FILE"
	envServersSRV       = "RESTATE_TUNNEL_SERVERS_SRV"
	envWorkerID         = "RESTATE_TUNNEL_WORKER_ID"
	envHostname         = "HOSTNAME"
)

// Defaults for the tunable knobs (mirroring the TS SDK).
const (
	defaultConnectTimeout      = 5 * time.Second
	defaultReconnectInitial    = 10 * time.Millisecond
	defaultReconnectMax        = 2 * time.Minute
	defaultDrainGrace          = 2 * time.Minute
	defaultPingInterval        = 75 * time.Second
	defaultPingTimeout         = 10 * time.Second
	defaultResolveInterval     = 30 * time.Second
	defaultMaxConcurrentStream = 4096
)

var (
	environmentIDRe = regexp.MustCompile(`^env_[A-Za-z0-9_-]+$`)
	tunnelNameRe    = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	regionRe        = regexp.MustCompile(`^[a-z0-9-]+(\.[a-z0-9-]+)*$`)
	authTokenRe     = regexp.MustCompile(`^[\x21-\x7e]+$`)
)

// resolvedConfig is the validated, defaults-applied form of Config.
type resolvedConfig struct {
	region     string
	serversSRV string
	servers    []target

	environmentID    string
	authToken        string
	authTokenFile    string
	signingPublicKey string
	tunnelName       string
	workerID         string

	tlsConfig *tls.Config
	logger    *slog.Logger

	connectTimeout       time.Duration
	handshakeTimeout     time.Duration
	reconnectInitial     time.Duration
	reconnectMax         time.Duration
	drainGrace           time.Duration
	pingInterval         time.Duration
	pingTimeout          time.Duration
	resolveInterval      time.Duration
	maxConcurrentStreams uint32
}

func resolveConfig(cfg config) (resolvedConfig, error) {
	// Env fallback for unset options. Discovery (region/serversSRV) falls back to
	// the env only when no discovery source was set explicitly (see below).
	region := cfg.region
	serversSRV := cfg.serversSRV
	environmentID := orEnv(cfg.environmentID, envEnvironmentID)
	signingPublicKey := orEnv(cfg.signingPublicKey, envSigningPublicKey)
	tunnelName := orEnv(cfg.tunnelName, envTunnelName)
	authToken := strings.TrimSpace(orEnv(cfg.authToken, envAuthToken))
	authTokenFile := orEnv(cfg.authTokenFile, envAuthTokenFile)
	workerID := orEnv(cfg.workerID, envWorkerID)

	rc := resolvedConfig{
		environmentID:        environmentID,
		authToken:            authToken,
		authTokenFile:        authTokenFile,
		signingPublicKey:     signingPublicKey,
		tunnelName:           tunnelName,
		workerID:             workerID,
		tlsConfig:            cfg.tlsConfig,
		logger:               cfg.logger,
		connectTimeout:       orDur(cfg.connectTimeout, defaultConnectTimeout),
		handshakeTimeout:     orDur(cfg.handshakeTimeout, defaultHandshakeTimeout),
		reconnectInitial:     orDur(cfg.reconnectInitial, defaultReconnectInitial),
		reconnectMax:         orDur(cfg.reconnectMax, defaultReconnectMax),
		drainGrace:           orDur(cfg.drainGrace, defaultDrainGrace),
		pingInterval:         orDur(cfg.pingInterval, defaultPingInterval),
		pingTimeout:          orDur(cfg.pingTimeout, defaultPingTimeout),
		resolveInterval:      orDur(cfg.resolveInterval, defaultResolveInterval),
		maxConcurrentStreams: defaultMaxConcurrentStream,
	}
	if rc.tlsConfig == nil {
		rc.tlsConfig = &tls.Config{}
	}
	if rc.logger == nil {
		rc.logger = slog.Default()
	}
	if rc.workerID == "" {
		rc.workerID = defaultWorkerID()
	}

	// Discovery: exactly one of servers, serversSRV, or region. When none is set
	// explicitly, fall back to the environment (SRV first, then region).
	explicit := 0
	if len(cfg.servers) > 0 {
		explicit++
	}
	if serversSRV != "" {
		explicit++
	}
	if region != "" {
		explicit++
	}
	if explicit > 1 {
		return resolvedConfig{}, fmt.Errorf("tunnel: set exactly one discovery source (WithServers, WithServersSRV, or WithRegion)")
	}
	if explicit == 0 {
		if serversSRV = strings.TrimSpace(os.Getenv(envServersSRV)); serversSRV == "" {
			region = strings.TrimSpace(os.Getenv(envCloudRegion))
		}
		if serversSRV == "" && region == "" {
			return resolvedConfig{}, fmt.Errorf("tunnel: set a discovery source: WithRegion (or %s), WithServersSRV (or %s), or WithServers", envCloudRegion, envServersSRV)
		}
	}
	rc.region = region
	rc.serversSRV = serversSRV
	if region != "" && !regionRe.MatchString(region) {
		return resolvedConfig{}, fmt.Errorf("tunnel: invalid region %q", region)
	}
	for _, s := range cfg.servers {
		t, err := parseServer(s)
		if err != nil {
			return resolvedConfig{}, err
		}
		rc.servers = append(rc.servers, t)
	}

	// Credentials / identity.
	if environmentID == "" {
		return resolvedConfig{}, fmt.Errorf("tunnel: environment id is required (WithEnvironment or %s)", envEnvironmentID)
	}
	if !environmentIDRe.MatchString(environmentID) {
		return resolvedConfig{}, fmt.Errorf("tunnel: environment id %q must match ^env_", environmentID)
	}
	if signingPublicKey == "" {
		return resolvedConfig{}, fmt.Errorf("tunnel: signing public key is required (WithEnvironment or %s)", envSigningPublicKey)
	}
	if !strings.HasPrefix(signingPublicKey, "publickeyv1_") {
		return resolvedConfig{}, fmt.Errorf("tunnel: signing public key must start with 'publickeyv1_'")
	}
	if tunnelName == "" {
		return resolvedConfig{}, fmt.Errorf("tunnel: tunnel name is required (WithTunnelName or %s)", envTunnelName)
	}
	if !tunnelNameRe.MatchString(tunnelName) {
		return resolvedConfig{}, fmt.Errorf("tunnel: invalid tunnel name %q", tunnelName)
	}
	if authToken == "" && authTokenFile == "" {
		return resolvedConfig{}, fmt.Errorf("tunnel: set WithAuthToken (or %s) or WithAuthTokenFile (or %s)", envAuthToken, envAuthTokenFile)
	}
	if authToken != "" && !authTokenRe.MatchString(authToken) {
		return resolvedConfig{}, fmt.Errorf("tunnel: AuthToken contains characters that aren't valid in an HTTP header")
	}

	return rc, nil
}

func orEnv(v, env string) string {
	if v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv(env))
}

func orDur(v, def time.Duration) time.Duration {
	if v > 0 {
		return v
	}
	return def
}
