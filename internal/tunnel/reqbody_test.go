package tunnel

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

// errReadCloser returns its data once, then err on the next Read.
type errReadCloser struct {
	data []byte
	err  error
}

func (r *errReadCloser) Read(p []byte) (int, error) {
	if len(r.data) > 0 {
		n := copy(p, r.data)
		r.data = r.data[n:]
		return n, nil
	}
	return 0, r.err
}

func (r *errReadCloser) Close() error { return nil }

func TestIsStreamCancel(t *testing.T) {
	require.True(t, isStreamCancel(http2.StreamError{StreamID: 9, Code: http2.ErrCodeCancel}))
	require.True(t, isStreamCancel(fmt.Errorf("wrapped: %w", http2.StreamError{Code: http2.ErrCodeCancel})))

	require.False(t, isStreamCancel(http2.StreamError{Code: http2.ErrCodeInternal}))
	require.False(t, isStreamCancel(io.ErrUnexpectedEOF))
	require.False(t, isStreamCancel(nil))
}

func TestCancelAsEOFBody(t *testing.T) {
	// A CANCEL reset reads as EOF, preserving any bytes drained alongside it.
	body := cancelAsEOFBody{&errReadCloser{data: []byte("hi"), err: http2.StreamError{Code: http2.ErrCodeCancel}}}
	got, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, "hi", string(got))

	// Non-CANCEL resets stay errors.
	body = cancelAsEOFBody{&errReadCloser{err: http2.StreamError{Code: http2.ErrCodeInternal}}}
	_, err = io.ReadAll(body)
	require.True(t, errors.As(err, new(http2.StreamError)))
}
