package ingress

import (
	"github.com/restatedev/sdk-go/internal/ingress"
	"github.com/restatedev/sdk-go/internal/options"
)

type Client = ingress.Client

// NewClient creates a new ingress client for calling Restate services from outside a Restate context.
// The baseUri should point to your Restate ingress endpoint (e.g., "http://localhost:8080").
//
// Options can be used to configure the client, such as setting a custom HTTP client, authentication key, or codec:
//
//	client := ingress.NewClient("http://localhost:8080",
//	    restate.WithAuthKey("my-auth-key"),
//	)
//
// To setup OpenTelemetry tracing, provide an HTTP client wrapped using the otel transport:
//
//	import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
//
//	client := ingress.NewClient("http://localhost:8080",
//	    // HTTP client wrapped with the otel transport.
//	    restate.WithHttpClient(&http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}),
//	)
func NewClient(baseUri string, opts ...options.IngressClientOption) *Client {
	clientOpts := options.IngressClientOptions{}
	for _, opt := range opts {
		opt.BeforeIngress(&clientOpts)
	}
	return ingress.NewClient(baseUri, clientOpts)
}
