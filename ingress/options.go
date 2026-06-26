package ingress

import (
	"net/http"

	"github.com/restatedev/sdk-go/internal/options"
)

// ClientOption configures an ingress [Client]; pass it to [NewClient].
type ClientOption = options.IngressClientOption

// RequestOption is an option for a request made through the ingress.
type RequestOption = options.IngressRequestOption

// SendOption is an option for a one-way send made through the ingress.
type SendOption = options.IngressSendOption

// InvocationHandleOption is an option for resolving an invocation handle by id or idempotency key.
type InvocationHandleOption = options.IngressInvocationHandleOption

// WithHttpClient sets the HTTP client used by the ingress [Client].
func WithHttpClient(c *http.Client) withHttpClient {
	return withHttpClient{c}
}

type withHttpClient struct {
	httpClient *http.Client
}

func (w withHttpClient) BeforeIngress(opts *options.IngressClientOptions) {
	opts.HttpClient = w.httpClient
}

// WithAuthKey sets the authentication key sent with requests by the ingress [Client].
func WithAuthKey(authKey string) withAuthKey {
	return withAuthKey{authKey}
}

type withAuthKey struct {
	authKey string
}

func (w withAuthKey) BeforeIngress(opts *options.IngressClientOptions) {
	opts.AuthKey = w.authKey
}
