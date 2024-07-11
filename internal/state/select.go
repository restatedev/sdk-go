package state

import (
	"slices"

	_go "github.com/restatedev/sdk-go/generated/proto/go"
	"github.com/restatedev/sdk-go/internal/futures"
	"github.com/restatedev/sdk-go/internal/wire"
)

type selector struct {
	machine *Machine
	inner   *futures.Selector
}

func (m *Machine) selector(futs ...futures.Selectable) (*selector, error) {
	inner, err := futures.Select(m.suspensionCtx, futs...)
	if err != nil {
		return nil, err
	}
	return &selector{m, inner}, nil
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
