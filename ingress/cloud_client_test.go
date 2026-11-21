package ingress_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/restatedev/sdk-go/ingress"
)

// envid=env_201kxe0kdaynq7h1fsn2chhcbwb, apikey=key_01abc.secret, region=eu.restate.cloud
const testIngressKey = "rstcloudv1_YvZh5tXwJQ4iu5nMEyWie3jTeAwAAx7ejWUuPHeT6x4EQ7iHr7rA8K6aBm1FCdLVS9bSPxXoexm6W7rUBC8m7tztSC2CNJB2cp85UJf54s5a2HvJL7T1"

type captureTransport struct{ req *http.Request }

func (c *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.req = req
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`"ok"`)),
		Request:    req,
	}, nil
}

func TestNewCloudClient(t *testing.T) {
	t.Setenv("RESTATE_INGRESS_KEY", testIngressKey)

	transport := &captureTransport{}
	c, err := ingress.NewCloudClient(ingress.WithHttpClient(&http.Client{Transport: transport}))
	require.NoError(t, err)

	out, err := ingress.Service[string, string](c, myService, myHandler).Request(context.Background(), "hi")
	require.NoError(t, err)
	require.Equal(t, "ok", out)
	require.Equal(t, "https://201kxe0kdaynq7h1fsn2chhcbwb.env.eu.restate.cloud:8080/MyService/myHandler", transport.req.URL.String())
	require.Equal(t, "Bearer key_01abc.secret", transport.req.Header.Get("Authorization"))

	// An explicit auth key wins.
	c, err = ingress.NewCloudClient(ingress.WithHttpClient(&http.Client{Transport: transport}), ingress.WithAuthKey("other"))
	require.NoError(t, err)
	_, err = ingress.Service[string, string](c, myService, myHandler).Request(context.Background(), "hi")
	require.NoError(t, err)
	require.Equal(t, "Bearer other", transport.req.Header.Get("Authorization"))
}

func TestNewCloudClientErrors(t *testing.T) {
	t.Setenv("RESTATE_INGRESS_KEY", "")
	_, err := ingress.NewCloudClient()
	require.ErrorContains(t, err, "RESTATE_INGRESS_KEY is not set")

	t.Setenv("RESTATE_INGRESS_KEY", "not-a-key")
	_, err = ingress.NewCloudClient()
	require.ErrorContains(t, err, "RESTATE_INGRESS_KEY is not a Restate Cloud key")
}
