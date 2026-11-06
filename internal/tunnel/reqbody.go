package tunnel

import (
	"errors"
	"io"

	"golang.org/x/net/http2"
)

// cancelAsEOFBody wraps a forwarded request body so that an HTTP/2 CANCEL stream
// error reads as a clean io.EOF.
//
// Once Restate Cloud has the full response it tears the request stream down with
// RST_STREAM(CANCEL) rather than a graceful end-of-stream. That surfaces to the
// SDK's input read loop and drain (server package) as a StreamError, which they
// log as "Unexpected when reading input" / "Failed to drain request stream" on
// every completed invocation. The cancel is normal teardown, not an error, and
// it's transport-specific — so we normalise it here at the tunnel boundary
// instead of teaching the generic server to swallow cancels.
type cancelAsEOFBody struct {
	io.ReadCloser
}

func (b cancelAsEOFBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil && isStreamCancel(err) {
		return n, io.EOF
	}
	return n, err
}

// isStreamCancel reports whether err is (or wraps) an HTTP/2 stream reset with
// the CANCEL code. Other reset codes (e.g. INTERNAL_ERROR) are genuine failures
// and are left untouched.
func isStreamCancel(err error) bool {
	var se http2.StreamError
	return errors.As(err, &se) && se.Code == http2.ErrCodeCancel
}
