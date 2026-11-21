package tunnel

import (
	"math/rand"
	"time"
)

// backoff is a jittered exponential backoff. next() returns the delay for the
// upcoming attempt (the current interval jittered by ±50%, capped at max) and
// advances the interval by the exponentiation factor. reset() returns to the
// initial interval — the caller should only reset after a connection has been
// healthy long enough to prove the backoff is no longer warranted, so a flapping
// server can't drive a zero-delay reconnect storm.
type backoff struct {
	initial time.Duration
	max     time.Duration
	factor  float64
	current time.Duration
}

func newBackoff(initial, max time.Duration) *backoff {
	return &backoff{initial: initial, max: max, factor: 2, current: initial}
}

func (b *backoff) next() time.Duration {
	jittered := time.Duration(float64(b.current) * (0.5 + rand.Float64()))
	if jittered > b.max {
		jittered = b.max
	}

	next := time.Duration(float64(b.current) * b.factor)
	if next > b.max {
		next = b.max
	}
	b.current = next

	return jittered
}

func (b *backoff) reset() {
	b.current = b.initial
}
