package statemachine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	// envid=env_201kxe0kdaynq7h1fsn2chhcbwb, apikey=key_01abc.secret, region=eu.restate.cloud, signingkey=publickeyv1_ChjE...
	testConnectKey = "rstcloudv1_KSBjZMu7W8fVXfpAMA44aHYpZLkWJXtsUqkuphzRQaSC9Y9ouB16bhAiaanbQSJm1RYcTr1WVerD8ndbzRJ5zAXGpJn7bbtD9kkHCCV2BkLJxsFNvp9ooP74EadYASB5vND9EutYmZbcgYLAsgL8f6ByksWUbWgzx7XdCKSiEFeuhCmWQzVmwdyUgEMpgUCgBx4n5Y7xPLukqZf4B"
	// envid=env_201kxe0kdaynq7h1fsn2chhcbwb, apikey=key_01abc.secret, region=eu.restate.cloud
	testIngressKey = "rstcloudv1_YvZh5tXwJQ4iu5nMEyWie3jTeAwAAx7ejWUuPHeT6x4EQ7iHr7rA8K6aBm1FCdLVS9bSPxXoexm6W7rUBC8m7tztSC2CNJB2cp85UJf54s5a2HvJL7T1"
)

func TestParseConnectKey(t *testing.T) {
	key, err := parseConnectKey(context.Background(), testConnectKey)
	require.NoError(t, err)
	require.Equal(t, "env_201kxe0kdaynq7h1fsn2chhcbwb", key.GetEnvironmentId())
	require.Equal(t, "key_01abc.secret", key.GetApiKey())
	require.Equal(t, "eu.restate.cloud", key.GetRegion())
	require.Equal(t, "publickeyv1_ChjENKeMvCtRnqG2mrBK1HmPKufgFUc98K8B3ononQvp", key.GetSigningPublicKey())

	_, err = parseConnectKey(context.Background(), testIngressKey)
	require.ErrorContains(t, err, "RESTATE_CONNECT_KEY carries no signing key")
}

func TestParseIngressKey(t *testing.T) {
	key, err := parseIngressKey(context.Background(), testIngressKey)
	require.NoError(t, err)
	require.Equal(t, "key_01abc.secret", key.GetApiKey())
	require.Equal(t, "https://201kxe0kdaynq7h1fsn2chhcbwb.env.eu.restate.cloud:8080", key.GetIngressUrl())
}
