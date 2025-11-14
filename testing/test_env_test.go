package testing

import (
	"testing"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/ingress"
	"github.com/restatedev/sdk-go/server"
	"github.com/stretchr/testify/require"
)

type Greeter struct{}

func (Greeter) Greet(ctx restate.Context, name string) (string, error) {
	// Respond to caller
	return "You said hi to " + name + "!", nil
}

func TestWithTestcontainers(t *testing.T) {
	tEnv := StartWithOptions(t, server.NewRestate().Bind(restate.Reflect(Greeter{})), WithRestateImage("ghcr.io/restatedev/restate:latest"))
	client := tEnv.Ingress()

	out, err := ingress.Service[string, string](client, "Greeter", "Greet").Request(t.Context(), "Francesco")
	require.NoError(t, err)
	require.Equal(t, "You said hi to Francesco!", out)
}
