package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
)

type connection struct {
	r       io.ReadCloser
	flusher http.Flusher
	w       http.ResponseWriter
	cancel  func()

	wLock sync.Mutex
	rLock sync.Mutex
}

func newConnection(w http.ResponseWriter, r *http.Request) *connection {
	ctx, cancel := context.WithCancel(r.Context())
	flusher, _ := w.(http.Flusher)

	c := &connection{r: r.Body, flusher: flusher, w: w, cancel: cancel}

	// Update the request context with the connection context.
	// If the connection is closed by the server,
	// it will also notify everything that waits on the request context.
	*r = *r.WithContext(ctx)

	return c
}

func (c *connection) Write(data []byte) (int, error) {
	c.wLock.Lock()
	defer c.wLock.Unlock()
	n, err := c.w.Write(data)
	if c.flusher != nil {
		c.flusher.Flush()
	}
	return n, err
}

func (c *connection) Read(data []byte) (int, error) {
	c.rLock.Lock()
	defer c.rLock.Unlock()
	n, err := c.r.Read(data)
	if errors.Is(err, http.ErrBodyReadAfterClose) ||
		// This error is returned when Close() comes while a Read is blocked.
		// Unfortunately the Golang stdlib won't give us a way to match with this error,
		// so we need this string matching
		(err != nil && err.Error() == "body closed by handler") {
		// make our state machine a bit more generic by avoiding this http error which to us means the same as EOF
		return n, io.EOF
	}
	return n, err
}

func (c *connection) Close() error {
	c.cancel()
	// Unblock Read()
	c.r.Close()
	return nil
}
