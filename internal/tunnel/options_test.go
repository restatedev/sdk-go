package tunnel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResolveConfigTunables(t *testing.T) {
	base := config{
		region:           "us",
		environmentID:    "env_x",
		signingPublicKey: "publickeyv1_x",
		tunnelName:       "t",
		authToken:        "tok",
	}

	rc, err := resolveConfig(base)
	require.NoError(t, err)
	require.Equal(t, defaultPingInterval, rc.pingInterval)
	require.Equal(t, defaultPingTimeout, rc.pingTimeout)
	require.Equal(t, defaultResolveInterval, rc.resolveInterval)

	over := base
	over.pingInterval = 5 * time.Second
	over.pingTimeout = time.Second
	over.resolveInterval = 10 * time.Second
	rc, err = resolveConfig(over)
	require.NoError(t, err)
	require.Equal(t, 5*time.Second, rc.pingInterval)
	require.Equal(t, time.Second, rc.pingTimeout)
	require.Equal(t, 10*time.Second, rc.resolveInterval)
}
