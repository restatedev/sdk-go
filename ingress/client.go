package ingress

import (
	"context"
	"fmt"

	"github.com/restatedev/sdk-go/internal/ingress"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/statemachine"
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
func NewClient(baseUri string, opts ...ClientOption) *Client {
	clientOpts := options.IngressClientOptions{}
	for _, opt := range opts {
		opt.BeforeIngress(&clientOpts)
	}
	return ingress.NewClient(baseUri, clientOpts)
}

// NewCloudClient creates a new ingress client for a Restate Cloud environment, configured from the
// ingress key in the RESTATE_INGRESS_KEY environment variable. Get the key from the Restate Cloud UI
// (Development → Configure Client).
//
// The key provides the ingress URL of the environment and the API key sent as bearer token.
//
//	client, err := ingress.NewCloudClient()
//	if err != nil {
//	    return err
//	}
//
// It returns an error when RESTATE_INGRESS_KEY is unset, or when it is not a valid ingress key.
func NewCloudClient(opts ...ClientOption) (*Client, error) {
	key, err := statemachine.IngressKeyFromEnv(context.Background())
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, fmt.Errorf("%s is not set. Get an ingress key from %s", statemachine.IngressKeyEnv, statemachine.IngressKeyURL)
	}

	clientOpts := options.IngressClientOptions{}
	for _, opt := range opts {
		opt.BeforeIngress(&clientOpts)
	}
	if clientOpts.AuthKey == "" {
		clientOpts.AuthKey = key.GetApiKey()
	}
	return ingress.NewClient(key.GetIngressUrl(), clientOpts), nil
}
