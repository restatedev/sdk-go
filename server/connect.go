package server

import (
	"context"
	"net/http"

	"github.com/restatedev/sdk-go/internal/tunnel"
)

// Connect serves the bound services over an outbound connection to Restate Cloud,
// so the deployment needs no inbound listener and no public ingress. It blocks
// until ctx is cancelled, then drains in-flight invocations before returning.
//
// Copy a connect key from the Restate Cloud UI (Development → Configure SDK) into
// the environment:
//
//	export RESTATE_CONNECT_KEY=rstcloudv1_...
//
// and connect with a deployment name, shared by the replicas of the same deployment:
//
//	err := server.NewRestate().
//		Bind(restate.Reflect(Greeter{})).
//		Connect(ctx, "greeter")
//
// When name is empty, it is read from RESTATE_CONNECT_NAME. Once connected, Connect
// logs how to register the deployment, once:
//
//	restate deployments register connect://greeter
//
// Connect returns nil after a ctx-driven shutdown, or the error that stopped it.
// Do not also call [Restate.Serve] on the same Restate.
func (r *Restate) Connect(ctx context.Context, name string) error {
	return tunnel.NewTunnel(
		func(signingPublicKey string) (http.Handler, error) {
			// The tunnel is full-duplex HTTP/2, and forwarded requests are signed by the environment.
			return r.Bidirectional(true).WithIdentityV1(signingPublicKey).Handler()
		},
		tunnel.WithTunnelName(name),
		tunnel.WithLogger(r.systemLog),
	).Start(ctx)
}
