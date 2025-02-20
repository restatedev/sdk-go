package restatecontext

// Selector is an iterator over a list of blocking Restate operations that are running
// in the background.
type Selector interface {
	// Remaining returns whether there are still operations that haven't been returned by Select().
	// There will always be exactly the same number of results as there were operations
	// given to Context.Select
	Remaining() bool
	// Select blocks on the next completed operation or returns nil if there are none left
	Select() Selectable
}

type selector struct {
	restateCtx  *ctx
	indexedFuts map[uint32]Selectable
}

func (restateCtx *ctx) Select(futs ...Selectable) Selector {
	indexedFuts := make(map[uint32]Selectable, len(futs))
	for i := range futs {
		handle := futs[i].handle()
		indexedFuts[handle] = futs[i]
	}

	return &selector{
		restateCtx:  restateCtx,
		indexedFuts: indexedFuts,
	}
}

func (s *selector) Select() Selectable {
	if !s.Remaining() {
		return nil
	}

	remainingHandles := make([]uint32, len(s.indexedFuts))
	for k := range s.indexedFuts {
		remainingHandles = append(remainingHandles, k)
	}

	// Do progress
	s.restateCtx.pollProgress(remainingHandles)

	// If we exit, one of them is completed, gotta figure out which one
	for _, handle := range remainingHandles {
		completed, err := s.restateCtx.stateMachine.IsCompleted(s.restateCtx, handle)
		if err != nil {
			panic(err)
		}
		if completed {
			fut := s.indexedFuts[handle]
			delete(s.indexedFuts, handle)
			return fut
		}
	}

	panic("Unexpectedly none of the remaining handles completed, this looks like a bug")
}

func (s *selector) Remaining() bool {
	return len(s.indexedFuts) != 0
}
