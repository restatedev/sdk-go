package state

import (
	"github.com/restatedev/sdk-go/internal/wire"
)

func (m *Machine) completable(entryIndex uint32) wire.CompleteableMessage {
	if entryIndex <= uint32(len(m.entries)) {
		// completion for replayed entry
		cm, ok := m.entries[entryIndex-1].(wire.CompleteableMessage)
		if !ok {
			return nil
		}
		return cm
	}

	// completion for an outgoing entry
	m.pendingMutex.RLock()
	defer m.pendingMutex.RUnlock()
	return m.pendingCompletions[entryIndex]
}

func (m *Machine) ackable(entryIndex uint32) wire.AckableMessage {
	m.pendingMutex.RLock()
	defer m.pendingMutex.RUnlock()
	return m.pendingAcks[entryIndex]
}

func (m *Machine) Write(message wire.Message) {
	if message, ok := message.(wire.CompleteableMessage); ok && !message.Completed() {
		m.pendingMutex.Lock()
		m.pendingCompletions[m.entryIndex] = message
		m.pendingMutex.Unlock()
	}
	if message, ok := message.(wire.AckableMessage); ok && !message.Acked() {
		m.pendingMutex.Lock()
		m.pendingAcks[m.entryIndex] = message
		m.pendingMutex.Unlock()
	}
	if err := m.protocol.Write(message); err != nil {
		panic(m.newWriteError(message, err))
	}
}

type writeError struct {
	entry wire.Message
	err   error
}

func (m *Machine) newWriteError(entry wire.Message, err error) *writeError {
	w := &writeError{entry, err}
	m.failure = w
	return w
}

func (m *Machine) handleCompletionsAcks() {
	for {
		msg, err := m.protocol.Read()
		if err != nil {
			return
		}
		switch msg := msg.(type) {
		case *wire.CompletionMessage:
			completable := m.completable(msg.EntryIndex)
			if completable == nil {
				m.log.Error().Uint32("index", msg.EntryIndex).Msg("failed to find pending completion at index")
				continue
			}
			completable.Complete(&msg.CompletionMessage)
			m.log.Debug().Uint32("index", msg.EntryIndex).Msg("processed completion")
		case *wire.EntryAckMessage:
			ackable := m.ackable(msg.EntryIndex)
			if ackable == nil {
				m.log.Error().Uint32("index", msg.EntryIndex).Msg("failed to find pending ack at index")
				continue
			}
			ackable.Ack()
			m.log.Debug().Uint32("index", msg.EntryIndex).Msg("processed ack")
		default:
			m.log.Error().Type("type", msg).Msg("unexpected non-completion non-ack message during invocation")
			continue
		}
	}
}
