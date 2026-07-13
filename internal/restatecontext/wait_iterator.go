package restatecontext

import (
	"github.com/restatedev/sdk-go/internal/errors"
)

// WaitIterator lets you
type WaitIterator interface {
	// Next returns whether there are still operations that haven't been returned by Value().
	// If returns false, no more operations will be completed. After returning false, Err() should be checked.
	Next() bool

	// Err returns an error if the waiter was canceled using Restate's cancellation feature.
	Err() errors.TerminalError

	// Value returns the current value of this iterator, or nil if the iterator returned Next previously.
	// Panics if called before the first Next
	Value() Future
}

func (restateCtx *ctx) WaitIter(futs ...Future) WaitIterator {
	// Copy for safety.
	remaining := make([]Future, len(futs))
	copy(remaining, futs)

	return &waitIterator{
		restateCtx: restateCtx,
		remaining:  remaining,
	}
}

type waitIterator struct {
	restateCtx    *ctx
	remaining     []Future
	lastCompleted Future
	cancelled     bool
}

func (s *waitIterator) Next() bool {
	if s.cancelled || len(s.remaining) == 0 {
		s.lastCompleted = nil
		return false
	}

	remainingHandles := make([]uint32, len(s.remaining))
	for i, fut := range s.remaining {
		// If a future is already resolved, return without calling state machine
		if fut.isResolved() {
			s.lastCompleted = s.removeAt(i)
			return true
		}
		remainingHandles[i] = fut.handle()
	}

	// Do progress
	cancelled := s.restateCtx.pollProgress(remainingHandles)
	if cancelled {
		s.lastCompleted = nil
		s.cancelled = true
		return false
	}

	// One of the handles is now completed on the state machine side, gotta
	// figure out which one.
	for i, handle := range remainingHandles {
		completed, err := s.restateCtx.stateMachine.IsCompleted(s.restateCtx, handle)
		if err != nil {
			panic(err)
		}
		if completed {
			s.lastCompleted = s.removeAt(i)
			return true
		}
	}

	panic("Unexpectedly none of the remaining handles completed, this looks like a bug")
}

// removeAt removes and returns the future at index i.
func (s *waitIterator) removeAt(i int) Future {
	fut := s.remaining[i]
	s.remaining = append(s.remaining[:i], s.remaining[i+1:]...)
	return fut
}

func (s *waitIterator) Err() errors.TerminalError {
	if s.cancelled {
		return errors.NewTerminalError("cancelled", errors.WithCode(409))
	}
	return nil
}

func (s *waitIterator) Value() Future {
	if !s.cancelled && s.lastCompleted == nil {
		panic("Unexpected call to Value() before first call to Next()")
	}
	if s.cancelled {
		return nil
	}
	return s.lastCompleted
}
