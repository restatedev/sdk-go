package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// inputDrainTimeout is the maximum time to wait for the request stream to reach EOF
// after the handler finishes, before forcefully closing the connection.
const inputDrainTimeout = 5 * time.Second

type stream struct {
	r       io.ReadCloser
	flusher http.Flusher
	w       http.ResponseWriter

	wLock sync.Mutex
	rLock sync.Mutex
}

func newStream(w http.ResponseWriter, r *http.Request) *stream {
	flusher, _ := w.(http.Flusher)
	c := &stream{r: r.Body, flusher: flusher, w: w}
	return c
}

func (c *stream) Write(data []byte) (int, error) {
	c.wLock.Lock()
	defer c.wLock.Unlock()

	if len(data) == 0 {
		return 0, nil
	}
	n, err := c.w.Write(data)
	if c.flusher != nil {
		c.flusher.Flush()
	}
	return n, err
}

func (c *stream) Read(data []byte) (int, error) {
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

// Drains and close connection
func (c *stream) Drain() error {
	defer c.r.Close()

	ch := make(chan error)
	go func(errCh chan<- error) {
		_, err := io.Copy(io.Discard, c.r)
		errCh <- err
	}(ch)

	select {
	case err := <-ch:
		if err != nil && err != io.EOF {
			return err
		}
	case <-time.After(inputDrainTimeout):
		return fmt.Errorf("Timeout waiting on request draining")
	}

	return nil
}
