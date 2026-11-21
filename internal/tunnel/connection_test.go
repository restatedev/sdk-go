package tunnel

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

const forwardPrefix = "/http/in-process/9080"

// TestDrainingRefusesNewStreams checks the connection refuses forwarded streams
// while draining, with the deselection sentinel.
func TestDrainingRefusesNewStreams(t *testing.T) {
	c := newConnection(&memConn{}, handshakeCredentials{}, nil, config{logger: testLogger, drainGrace: time.Second, handshakeTimeout: time.Second})
	c.mu.Lock()
	c.draining = true
	c.mu.Unlock()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, forwardPrefix+"/invoke/Greeter/Greet", nil)
	req.RequestURI = forwardPrefix + "/invoke/Greeter/Greet"
	c.serveForwarded(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Equal(t, "true", rec.Header().Get(drainingHeader))
}
