package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnectRequiresConnectKey(t *testing.T) {
	t.Setenv("RESTATE_CONNECT_KEY", "")
	t.Setenv("RESTATE_INPROC_ENVIRONMENT_ID", "")

	err := NewRestate().Connect(context.Background(), "greeter")
	require.ErrorContains(t, err, "set RESTATE_CONNECT_KEY with a connect key from https://cloud.restate.dev/")
}

func TestConnectRequiresName(t *testing.T) {
	// envid=env_201kxe0kdaynq7h1fsn2chhcbwb, apikey=key_01abc.secret, region=eu.restate.cloud, signingkey=publickeyv1_ChjE...
	t.Setenv("RESTATE_CONNECT_KEY", "rstcloudv1_KSBjZMu7W8fVXfpAMA44aHYpZLkWJXtsUqkuphzRQaSC9Y9ouB16bhAiaanbQSJm1RYcTr1WVerD8ndbzRJ5zAXGpJn7bbtD9kkHCCV2BkLJxsFNvp9ooP74EadYASB5vND9EutYmZbcgYLAsgL8f6ByksWUbWgzx7XdCKSiEFeuhCmWQzVmwdyUgEMpgUCgBx4n5Y7xPLukqZf4B")
	t.Setenv("RESTATE_CONNECT_NAME", "")
	t.Setenv("RESTATE_INPROC_TUNNEL_NAME", "")

	err := NewRestate().Connect(context.Background(), "")
	require.ErrorContains(t, err, "a deployment name is required")
}
