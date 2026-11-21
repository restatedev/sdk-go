package tunnel

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	// envid=env_201kxe0kdaynq7h1fsn2chhcbwb, apikey=key_01abc.secret, region=eu.restate.cloud, signingkey=publickeyv1_ChjE...
	testConnectKey = "rstcloudv1_KSBjZMu7W8fVXfpAMA44aHYpZLkWJXtsUqkuphzRQaSC9Y9ouB16bhAiaanbQSJm1RYcTr1WVerD8ndbzRJ5zAXGpJn7bbtD9kkHCCV2BkLJxsFNvp9ooP74EadYASB5vND9EutYmZbcgYLAsgL8f6ByksWUbWgzx7XdCKSiEFeuhCmWQzVmwdyUgEMpgUCgBx4n5Y7xPLukqZf4B"
	// envid=env_abc, region=eu.restate.cloud, signingkey=publickeyv1_ChjE... (no apikey)
	testConnectKeyNoAPIKey = "rstcloudv1_2xrd1DUuyBENsUorp1u9iPSRvC7BpQMQVzsYMXsYPkWg4hwMvLXMPiDtFbou1nexhddQYupXZgFmD8ZX6EdjaKSMYFdpU6utvbTFJsz1oG1rCBzfrZm4AmzvbEu3NFSS8bZ4WcscZLdV91rP"
)

func TestResolveConfigConnectKey(t *testing.T) {
	t.Setenv("RESTATE_CONNECT_KEY", testConnectKey)
	// The key wins over the other environment variables.
	t.Setenv(envEnvironmentID, "env_other")
	t.Setenv(envAuthToken, "env-token")
	t.Setenv(envCloudRegion, "us")

	rc, err := resolveConfig(config{tunnelName: "t"})
	require.NoError(t, err)
	require.Equal(t, "env_201kxe0kdaynq7h1fsn2chhcbwb", rc.environmentID)
	require.Equal(t, "publickeyv1_ChjENKeMvCtRnqG2mrBK1HmPKufgFUc98K8B3ononQvp", rc.signingPublicKey)
	require.Equal(t, "key_01abc.secret", rc.authToken)
	require.Equal(t, "tunnel.eu.restate.cloud", rc.serversSRV)
	require.Empty(t, rc.region)

	// Explicit options win over the key.
	rc, err = resolveConfig(config{
		tunnelName:       "t",
		region:           "us",
		environmentID:    "env_explicit",
		signingPublicKey: "publickeyv1_explicit",
		authToken:        "explicit-token",
	})
	require.NoError(t, err)
	require.Equal(t, "env_explicit", rc.environmentID)
	require.Equal(t, "publickeyv1_explicit", rc.signingPublicKey)
	require.Equal(t, "explicit-token", rc.authToken)
	require.Equal(t, "us", rc.region)
	require.Empty(t, rc.serversSRV)
}

func TestResolveConfigConnectKeyWithoutAPIKey(t *testing.T) {
	t.Setenv("RESTATE_CONNECT_KEY", testConnectKeyNoAPIKey)
	t.Setenv(envAuthToken, "env-token")

	rc, err := resolveConfig(config{tunnelName: "t"})
	require.NoError(t, err)
	require.Equal(t, "env_abc", rc.environmentID)
	require.Equal(t, "env-token", rc.authToken)
}

func TestResolveConfigInvalidConnectKey(t *testing.T) {
	t.Setenv("RESTATE_CONNECT_KEY", "not-a-key")
	_, err := resolveConfig(config{tunnelName: "t"})
	require.ErrorContains(t, err, "RESTATE_CONNECT_KEY is not a Restate Cloud key")
}
