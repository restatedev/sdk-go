package restatecontext

import (
	"fmt"

	"github.com/restatedev/sdk-go/internal/errors"
)

// WaitIterator lets you
type WaitIterator interface {
	// Next returns whether there are still operations that haven't been returned by Value().
	// If returns false, no more operations will be completed. After returning false, Err() should be checked.
	Next() bool

	// Err returns an error if the waiter was canceled using Restate's cancellation feature.
	Err() error

	// Value returns the current value of this iterator, or nil if the iterator returned Next previously.
	// Panics if called before the first Next
	Value() Selectable
}

func (restateCtx *ctx) WaitIter(futs ...Selectable) WaitIterator {
	indexedFuts := make(map[uint32]Selectable, len(futs))
	for i := range futs {
		handle := futs[i].handle()
		indexedFuts[handle] = futs[i]
	}

	return &waitIterator{
		restateCtx:  restateCtx,
		indexedFuts: indexedFuts,
	}
}

type waitIterator struct {
	restateCtx    *ctx
	indexedFuts   map[uint32]Selectable
	lastCompleted Selectable
	cancelled     bool
}

func (s *waitIterator) Next() bool {
	if s.cancelled || len(s.indexedFuts) == 0 {
		s.lastCompleted = nil
		return false
	}

	remainingHandles := make([]uint32, 0, len(s.indexedFuts))
	for k := range s.indexedFuts {
		remainingHandles = append(remainingHandles, k)
	}

	// Do progress
	cancelled := s.restateCtx.pollProgress(remainingHandles)
	if cancelled {
		s.lastCompleted = nil
		s.cancelled = true
		return false
	}

	// If we exit, one of them is completed, gotta figure out which one
	for _, handle := range remainingHandles {
		completed, err := s.restateCtx.stateMachine.IsCompleted(s.restateCtx, handle)
		if err != nil {
			panic(err)
		}
		if completed {
			fut := s.indexedFuts[handle]
			delete(s.indexedFuts, handle)
			s.lastCompleted = fut
			return true
		}
	}

	panic("Unexpectedly none of the remaining handles completed, this looks like a bug")
}

func (s *waitIterator) Err() error {
	if s.cancelled {
		return &errors.CodeError{Inner: &errors.TerminalError{Inner: fmt.Errorf("cancelled")}, Code: errors.Code(409)}
	}
	return nil
}

func (s *waitIterator) Value() Selectable {
	if !s.cancelled && s.lastCompleted == nil {
		panic("Unexpected call to Value() before first call to Next()")
	}
	if s.cancelled {
		return nil
	}
	return s.lastCompleted
}
