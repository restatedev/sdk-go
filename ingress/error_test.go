package ingress_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/ingress"
)

// doErrRequest configures the mock server to reply with the given status, headers and
// body, issues a Request through the ingress client, and returns the resulting error.
func doErrRequest(t *testing.T, status int, respHeaders map[string]string, body string) error {
	t.Helper()
	m := newMockIngressServer()
	defer m.Close()
	m.errStatus = status
	m.errHeaders = respHeaders
	m.errBody = []byte(body)

	c := newIngressClient(m.URL)
	_, err := ingress.Service[map[string]any, any](c, myService, myHandler).
		Request(context.Background(), map[string]any{})
	require.Error(t, err)
	return err
}

func TestInvocationError_BridgesToTerminalError(t *testing.T) {
	err := doErrRequest(t, http.StatusForbidden,
		map[string]string{"x-restate-error-source": "invocation"},
		`{"code":403,"message":"this is a test","stacktrace":"at foo"}`)

	var ie ingress.Error
	require.True(t, errors.As(err, &ie))
	require.True(t, ie.IsInvocationError())
	require.Equal(t, restate.Code(403), ie.Code())
	require.Equal(t, "this is a test", ie.Message())
	require.Equal(t, "at foo", ie.Stacktrace())

	// Invocation-sourced errors bridge to the SDK terminal-error model.
	require.True(t, restate.IsTerminalError(err))
	te := restate.AsTerminalError(err)
	require.NotNil(t, te)
	require.Equal(t, restate.Code(403), te.Code())
	require.Equal(t, "this is a test", te.Message())
}

func TestIngressError_NotTerminal(t *testing.T) {
	// No x-restate-error-source header: an ingress/transport error, not terminal.
	err := doErrRequest(t, http.StatusServiceUnavailable, nil,
		`{"code":503,"message":"overloaded"}`)

	var ie ingress.Error
	require.True(t, errors.As(err, &ie))
	require.False(t, ie.IsInvocationError())
	require.Equal(t, restate.Code(503), ie.Code())

	require.False(t, restate.IsTerminalError(err))
	require.Nil(t, restate.AsTerminalError(err))
}

func TestIngressError_NotFound(t *testing.T) {
	err := doErrRequest(t, http.StatusNotFound, nil,
		`{"code":404,"message":"not found"}`)

	var nf *ingress.InvocationNotFoundError
	require.True(t, errors.As(err, &nf))
	require.Equal(t, restate.Code(404), nf.Code())
	require.False(t, nf.IsInvocationError())

	// It is also an ingress.Error, and not a not-ready error.
	var ie ingress.Error
	require.True(t, errors.As(err, &ie))
	var nr *ingress.InvocationNotReadyError
	require.False(t, errors.As(err, &nr))
}

func TestIngressError_NotReady(t *testing.T) {
	err := doErrRequest(t, 470, nil, `{"code":470,"message":"not ready"}`)

	var nr *ingress.InvocationNotReadyError
	require.True(t, errors.As(err, &nr))

	var nf *ingress.InvocationNotFoundError
	require.False(t, errors.As(err, &nf))
}

func TestIngressError_EmptyBody(t *testing.T) {
	// An empty body still yields a usable Error with a status-derived code.
	err := doErrRequest(t, http.StatusUnauthorized, nil, "")

	var ie ingress.Error
	require.True(t, errors.As(err, &ie))
	require.Equal(t, restate.Code(http.StatusUnauthorized), ie.Code())
	require.False(t, ie.IsInvocationError())
}
