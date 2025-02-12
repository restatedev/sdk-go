package state

import (
	"slices"

	_go "github.com/restatedev/sdk-go/generated/dev/restate/sdk/go"
	"github.com/restatedev/sdk-go/internal/futures"
	"github.com/restatedev/sdk-go/internal/wire"
)

// Selector is an iterator over a list of blocking Restate operations that are running
// in the background.
type Selector interface {
	// Remaining returns whether there are still operations that haven't been returned by Select().
	// There will always be exactly the same number of results as there were operations
	// given to Context.Select
	Remaining() bool
	// Select blocks on the next completed operation or returns nil if there are none left
	Select() futures.Selectable
}

type selector struct {
	machine *Machine
	inner   *futures.Selector
}

func (m *Machine) selector(futs ...futures.Selectable) *selector {
	inner := futures.Select(m.suspensionCtx, futs...)
	return &selector{m, inner}
}

func (s *selector) Select() futures.Selectable {
	entry, entryIndex := replayOrNew(
		s.machine,
		func(entry *wire.SelectorEntryMessage) *wire.SelectorEntryMessage {
			indexes := s.inner.Indexes()
			if !slices.Equal(entry.JournalEntries, indexes) {
				panic(s.machine.newEntryMismatch(&wire.SelectorEntryMessage{
					SelectorEntryMessage: _go.SelectorEntryMessage{
						JournalEntries: indexes,
					},
				}, entry))
			}
			return entry
		},
		func() *wire.SelectorEntryMessage {
			return s._select()
		},
	)

	if entry == nil {
		// no futures left to select
		return nil
	}

	// selector entry must be acknowledged before proceeding
	entry.Await(s.machine.suspensionCtx, entryIndex)
	return s.inner.Take(entry.WinningEntryIndex)
}

func (s *selector) Remaining() bool {
	return s.inner.Remaining()
}

func (s *selector) _select() *wire.SelectorEntryMessage {
	indexes := s.inner.Indexes()
	winningEntryIndex, ok := s.inner.Select()
	if !ok {
		// no more promises left, we don't need to write this to the journal
		return nil
	}

	entry := &wire.SelectorEntryMessage{
		SelectorEntryMessage: _go.SelectorEntryMessage{
			JournalEntries:    indexes,
			WinningEntryIndex: winningEntryIndex,
		},
	}
	s.machine.Write(entry)

	return entry
}
