package tunnel

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/restatedev/sdk-go/internal/statemachine"
)

// Environment variables that fill unset options. RESTATE_CONNECT_KEY (a connect
// key from the Restate Cloud UI) provides the environment id, signing key, API
// token and region, and takes precedence over the variables below. The RESTATE_INPROC_* names are
// the ones the restate-operator injects in in-process mode; RESTATE_AUTH_TOKEN
// and RESTATE_TUNNEL_SERVERS_SRV match the standalone restate-cloud-tunnel-client.
// Explicitly-set options always take precedence over the environment.
const (
	envConnectName      = "RESTATE_CONNECT_NAME"
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

// resolveConfig applies environment fallback and defaults to the raw option
// inputs, validates them, and returns the ready-to-use config (with the derived
// serverTargets / maxConcurrentStreams filled in). It operates on a copy, so the
// caller's config is left untouched.
func resolveConfig(c config) (config, error) {
	// A connect key (RESTATE_CONNECT_KEY, from the Restate Cloud UI) fills the
	// environment coordinates the options left unset. It takes precedence over
	// the other environment variables.
	connectKey, err := statemachine.ConnectKeyFromEnv(context.Background())
	if err != nil {
		return config{}, fmt.Errorf("tunnel: %w", err)
	}
	var connectKeySRV, shadowedEnvironmentID string
	if connectKey != nil {
		if injected := strings.TrimSpace(os.Getenv(envEnvironmentID)); injected != "" && c.environmentID == "" && injected != connectKey.GetEnvironmentId() {
			shadowedEnvironmentID = injected
		}
		if c.environmentID == "" {
			c.environmentID = connectKey.GetEnvironmentId()
		}
		if c.signingPublicKey == "" {
			c.signingPublicKey = connectKey.GetSigningPublicKey()
		}
		if c.authToken == "" && c.authTokenFile == "" && connectKey.HasApiKey() {
			c.authToken = connectKey.GetApiKey()
		}
		// The key's region is a domain (e.g. eu.restate.cloud).
		connectKeySRV = "tunnel." + connectKey.GetRegion()
	}

	// Env fallback for unset options.
	c.environmentID = orEnv(c.environmentID, envEnvironmentID)
	c.signingPublicKey = orEnv(c.signingPublicKey, envSigningPublicKey)
	c.tunnelName = orEnv(orEnv(c.tunnelName, envConnectName), envTunnelName)
	c.authToken = strings.TrimSpace(orEnv(c.authToken, envAuthToken))
	c.authTokenFile = orEnv(c.authTokenFile, envAuthTokenFile)
	c.workerID = orEnv(c.workerID, envWorkerID)

	// Without even an environment, there is no configuration at all: point at the connect key.
	if c.environmentID == "" {
		return config{}, fmt.Errorf("set %s with a connect key from %s", statemachine.ConnectKeyEnv, statemachine.ConnectKeyURL)
	}

	// Defaults.
	c.connectTimeout = orDur(c.connectTimeout, defaultConnectTimeout)
	c.handshakeTimeout = orDur(c.handshakeTimeout, defaultHandshakeTimeout)
	c.reconnectInitial = orDur(c.reconnectInitial, defaultReconnectInitial)
	c.reconnectMax = orDur(c.reconnectMax, defaultReconnectMax)
	c.drainGrace = orDur(c.drainGrace, defaultDrainGrace)
	c.pingInterval = orDur(c.pingInterval, defaultPingInterval)
	c.pingTimeout = orDur(c.pingTimeout, defaultPingTimeout)
	c.resolveInterval = orDur(c.resolveInterval, defaultResolveInterval)
	c.maxConcurrentStreams = defaultMaxConcurrentStream
	if c.tlsConfig == nil {
		c.tlsConfig = &tls.Config{}
	}
	if c.logger == nil {
		c.logger = slog.Default()
	}
	if shadowedEnvironmentID != "" {
		// A key for one environment next to an operator's variables for another is almost certainly a stray developer key.
		c.logger.Warn("The connect key is for a different environment than "+envEnvironmentID+"; the key wins",
			"key_environment_id", connectKey.GetEnvironmentId(), envEnvironmentID, shadowedEnvironmentID)
	}
	if c.workerID == "" {
		c.workerID = defaultWorkerID()
	}

	// Discovery: exactly one of servers, serversSRV, or region. When none is set
	// explicitly, fall back to the environment (SRV first, then region).
	explicit := 0
	if len(c.servers) > 0 {
		explicit++
	}
	if c.serversSRV != "" {
		explicit++
	}
	if c.region != "" {
		explicit++
	}
	if explicit > 1 {
		return config{}, fmt.Errorf("tunnel: set exactly one discovery source (WithServers, WithServersSRV, or WithRegion)")
	}
	if explicit == 0 && connectKeySRV != "" {
		c.serversSRV = connectKeySRV
	} else if explicit == 0 {
		if c.serversSRV = strings.TrimSpace(os.Getenv(envServersSRV)); c.serversSRV == "" {
			c.region = strings.TrimSpace(os.Getenv(envCloudRegion))
		}
		if c.serversSRV == "" && c.region == "" {
			return config{}, fmt.Errorf("tunnel: set a discovery source: WithRegion (or %s), WithServersSRV (or %s), or WithServers", envCloudRegion, envServersSRV)
		}
	}
	if c.region != "" && !regionRe.MatchString(c.region) {
		return config{}, fmt.Errorf("tunnel: invalid region %q", c.region)
	}
	c.serverTargets = nil
	for _, s := range c.servers {
		t, err := parseServer(s)
		if err != nil {
			return config{}, err
		}
		c.serverTargets = append(c.serverTargets, t)
	}

	// Credentials / identity.
	if !environmentIDRe.MatchString(c.environmentID) {
		return config{}, fmt.Errorf("tunnel: environment id %q must match ^env_", c.environmentID)
	}
	if c.signingPublicKey == "" {
		return config{}, fmt.Errorf("tunnel: signing public key is required (WithEnvironment or %s)", envSigningPublicKey)
	}
	if !strings.HasPrefix(c.signingPublicKey, "publickeyv1_") {
		return config{}, fmt.Errorf("tunnel: signing public key must start with 'publickeyv1_'")
	}
	if c.tunnelName == "" {
		return config{}, fmt.Errorf("a deployment name is required: pass it to Connect, or set %s", envConnectName)
	}
	if !tunnelNameRe.MatchString(c.tunnelName) {
		return config{}, fmt.Errorf("tunnel: invalid tunnel name %q", c.tunnelName)
	}
	if c.authToken == "" && c.authTokenFile == "" {
		return config{}, fmt.Errorf("tunnel: set WithAuthToken (or %s) or WithAuthTokenFile (or %s)", envAuthToken, envAuthTokenFile)
	}
	if c.authToken != "" && !authTokenRe.MatchString(c.authToken) {
		return config{}, fmt.Errorf("tunnel: auth token contains characters that aren't valid in an HTTP header")
	}

	return c, nil
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

// cloudUIURL is the Restate Cloud UI, linked in the registration hint.
const cloudUIURL = "https://cloud.restate.dev"

// registrationIdentifier is the deployment identifier the CLI and the Cloud UI accept.
func registrationIdentifier(name string) string {
	return "connect://" + name
}

// registrationDeepLink is the Cloud UI link that opens the register dialog pre-filled.
func registrationDeepLink(name string) string {
	return cloudUIURL + "/to/overview?register-deployment=" + registrationIdentifier(name)
}

// regionLabel is the human-readable discovery target logged when connecting:
// the region domain (eu.restate.cloud), else the SRV name without its tunnel.
// prefix, else the explicit servers.
func regionLabel(c config) string {
	switch {
	case c.region != "":
		return c.region + ".restate.cloud"
	case c.serversSRV != "":
		return strings.TrimPrefix(c.serversSRV, "tunnel.")
	default:
		return strings.Join(c.servers, ",")
	}
}
