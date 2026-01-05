package testing

import (
	"fmt"
	"strings"
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

func (Greeter) CheckContextPropagation(ctx restate.Context, name string) (string, error) {
	// Bunch of test wrappings
	ctx = restate.WithValue(ctx, "name", name)
	ctx = restate.WithValue(ctx, "upper", strings.ToUpper(name))

	return restate.Run(ctx, func(ctx restate.RunContext) (string, error) {
		return fmt.Sprintf("%s-%s", ctx.Value("name").(string), ctx.Value("upper").(string)), nil
	})
}

func TestWithTestcontainers(t *testing.T) {
	// Initialize test environment
	tEnv := StartWithOptions(t, server.NewRestate().Bind(restate.Reflect(Greeter{})), WithRestateImage("ghcr.io/restatedev/restate:latest"))
	client := tEnv.Ingress()

	tests := []struct {
		name string
		test func(t *testing.T)
	}{{
		name: "smoke test",
		test: func(t *testing.T) {
			// Simple smoke test
			out, err := ingress.Service[string, string](client, "Greeter", "Greet").Request(t.Context(), "Francesco")
			require.NoError(t, err)
			require.Equal(t, "You said hi to Francesco!", out)
		},
	},
		{
			name: "context propagation",
			test: func(t *testing.T) {
				// Check context propagation works correctly
				out, err := ingress.Service[string, string](client, "Greeter", "CheckContextPropagation").Request(t.Context(), "Pippo")
				require.NoError(t, err)
				require.Equal(t, "Pippo-PIPPO", out)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.test(t)
		})
	}
}
