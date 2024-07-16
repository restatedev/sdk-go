package futures

import (
	"context"
	"reflect"
	"slices"

	"github.com/restatedev/sdk-go/internal/wire"
)

type Selectable interface {
	getEntry() (wire.CompleteableMessage, uint32)
}

type Selector struct {
	suspensionCtx context.Context
	indexedFuts   map[uint32]Selectable
	indexedChans  map[uint32]<-chan struct{}
	chosen        Selectable
	err           error
}

func (s *Selector) Select() (uint32, bool) {
	if s.err != nil {
		return 0, false
	}
	if len(s.indexedFuts) == 0 {
		return 0, false
	}

	indexes := s.Indexes()
	cases := make([]reflect.SelectCase, len(indexes)+1)
	for i, entryIndex := range indexes {
		cases[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(s.indexedChans[entryIndex]),
		}

	}
	cases[len(indexes)] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(s.suspensionCtx.Done()),
	}
	chosen, _, _ := reflect.Select(cases)
	switch chosen {
	case len(indexes):
		// suspensionCtx won
		panic(&wire.SuspensionPanic{EntryIndexes: indexes, Err: context.Cause(s.suspensionCtx)})
	default:
		return indexes[chosen], true
	}
}

func (s *Selector) Take(winningEntryIndex uint32) Selectable {
	selectable := s.indexedFuts[winningEntryIndex]
	if selectable == nil {
		return nil
	}
	entry, _ := selectable.getEntry()
	if !entry.Completed() {
		return nil
	}
	delete(s.indexedFuts, winningEntryIndex)
	delete(s.indexedChans, winningEntryIndex)
	return selectable
}

func (s *Selector) Remaining() bool {
	return len(s.indexedFuts) > 0
}

func (s *Selector) Indexes() []uint32 {
	indexes := make([]uint32, 0, len(s.indexedFuts))
	for i := range s.indexedFuts {
		indexes = append(indexes, i)
	}
	slices.Sort(indexes)
	return indexes
}

func Select(suspensionCtx context.Context, futs ...Selectable) *Selector {
	s := &Selector{
		suspensionCtx: suspensionCtx,
		indexedFuts:   make(map[uint32]Selectable, len(futs)),
		indexedChans:  make(map[uint32]<-chan struct{}, len(futs)),
	}
	for i := range futs {
		entry, entryIndex := futs[i].getEntry()
		s.indexedFuts[entryIndex] = futs[i]
		s.indexedChans[entryIndex] = entry.Done()
	}
	return s
}
