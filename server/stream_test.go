package server

import (
	"bytes"
	"io"
	"testing"
)

// TestDrainReadRace exercises Drain concurrently with Read against a shared
// underlying reader. Both paths ultimately call the reader's Read, so unless
// Drain takes rLock the two run without synchronization. Run with -race:
// before the fix this reports a data race on the shared reader's internal
// state, after the fix rLock serializes the two paths and it passes.
func TestDrainReadRace(t *testing.T) {
	body := bytes.Repeat([]byte("x"), 1<<20)
	c := &stream{r: io.NopCloser(bytes.NewReader(body))}

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 512)
		for {
			if _, err := c.Read(buf); err != nil {
				return
			}
		}
	}()

	if err := c.Drain(); err != nil {
		t.Fatalf("Drain returned error: %v", err)
	}
	<-done
}
