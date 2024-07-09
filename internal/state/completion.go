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
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.pendingCompletions[entryIndex]
}

func (m *Machine) ackable(entryIndex uint32) wire.AckableMessage {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.pendingAcks[entryIndex]
}

func (m *Machine) Write(message wire.Message) error {
	var flag wire.Flag
	if message, ok := message.(wire.CompleteableMessage); ok && !message.Completed() {
		m.mutex.Lock()
		m.pendingCompletions[m.entryIndex] = message
		m.mutex.Unlock()
	}
	if message, ok := message.(wire.AckableMessage); ok && !message.Acked() {
		m.mutex.Lock()
		m.pendingAcks[m.entryIndex] = message
		m.mutex.Unlock()
		flag |= wire.FlagRequiresAck
	}
	return m.protocol.Write(message, flag)
}

func (m *Machine) handleCompletionsAcks() {
	for {
		msg, err := m.protocol.Read()
		if err != nil {
			return
		}
		switch msg.Type() {
		case wire.CompletionMessageType:
			msg := msg.(*wire.CompletionMessage)
			completable := m.completable(msg.EntryIndex)
			if completable == nil {
				m.log.Error().Uint32("index", msg.CompletionMessage.EntryIndex).Msg("failed to find pending completion at index")
				continue
			}
			completable.Complete(&msg.CompletionMessage)
			m.log.Debug().Uint32("index", msg.CompletionMessage.EntryIndex).Msg("processed completion")
		case wire.EntryAckMessageType:
			msg := msg.(*wire.EntryAckMessage)
			ackable := m.ackable(msg.EntryAckMessage.EntryIndex)
			if ackable == nil {
				m.log.Error().Uint32("index", msg.EntryAckMessage.EntryIndex).Msg("failed to find pending ack at index")
				continue
			}
			ackable.Ack()
			m.log.Debug().Uint32("index", msg.EntryAckMessage.EntryIndex).Msg("processed ack")
		default:
			m.log.Error().Stringer("type", msg.Type()).Msg("unexpected non-completion non-ack message during invocation")
			continue
		}
	}
}
