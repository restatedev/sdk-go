package state

import (
	"context"
	"fmt"
	"sync"

	"github.com/restatedev/sdk-go/generated/proto/protocol"
	"github.com/restatedev/sdk-go/internal/wire"
)

type AckFuture struct {
	sync.Mutex
	ch   chan struct{}
	done bool
}

func newAck() *AckFuture {
	return &AckFuture{ch: make(chan struct{})}
}

func (c *AckFuture) ack() error {
	c.Lock()
	defer c.Unlock()
	if c.done {
		return fmt.Errorf("received ack on already acked message")
	}
	c.done = true
	close(c.ch)
	return nil
}

func (c *AckFuture) Done(ctx context.Context) error {
	if c.done {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.ch:
		return nil
	}
}

type CompletionFuture struct {
	sync.Mutex
	ch   chan struct{}
	done bool
	*protocol.CompletionMessage
}

func newCompletion() *CompletionFuture {
	return &CompletionFuture{ch: make(chan struct{})}
}

func (c *CompletionFuture) complete(msg *protocol.CompletionMessage) error {
	c.Lock()
	defer c.Unlock()
	if c.done {
		return fmt.Errorf("received completion on already completed message")
	}
	c.done = true
	c.CompletionMessage = msg
	close(c.ch)
	return nil
}

func (c *CompletionFuture) Await(ctx context.Context) (*protocol.CompletionMessage, error) {
	c.Lock()
	defer c.Unlock()
	if c.done {
		return c.CompletionMessage, nil
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ch:
		return c.CompletionMessage, nil
	}
}

func (c *CompletionFuture) Done() (*protocol.CompletionMessage, bool) {
	c.Lock()
	defer c.Unlock()
	if c.done {
		return c.CompletionMessage, true
	} else {
		return nil, false
	}
}

func (m *Machine) checkReplayCompletion(index uint32, msg wire.Message) {
	switch msg.Type() {
	case wire.SetStateEntryMessageType, wire.ClearStateEntryMessageType,
		wire.ClearAllStateEntryMessageType, wire.CompleteAwakeableEntryMessageType,
		wire.OneWayCallEntryMessageType:
		// don't need completion
	default:
		if !msg.Flags().Completed() {
			m.pendingCompletions[index] = newCompletion()
		}
	}
}

func (m *Machine) Write(message wire.Message, flag wire.Flag) (*AckFuture, *CompletionFuture, error) {
	index := m.entryIndex
	ack := m.pendingAcks[index]
	completion := m.pendingCompletions[index]

	if flag.Ack() && ack == nil {
		ack = newAck()
		m.pendingAcks[index] = ack
	}

	if completion == nil {
		switch message := message.(type) {
		case *wire.SetStateEntryMessage, *wire.ClearStateEntryMessage,
			*wire.ClearAllStateEntryMessage, *wire.CompleteAwakeableEntryMessage,
			*wire.OneWayCallEntryMessage:
			// don't need completion, nil is ok
		case *wire.GetStateEntryMessage:
			completion = newCompletion()
			if message.GetStateEntryMessage.Result == nil {
				m.pendingCompletions[index] = completion
				break
			} else {
				// we are already completed; ensure flag is set right
				flag |= wire.FlagCompleted
				break
			}
		case *wire.GetStateKeysEntryMessage:
			completion = newCompletion()
			if message.GetStateKeysEntryMessage.Result == nil {
				m.pendingCompletions[index] = completion
				break
			} else {
				// we are already completed; ensure flag is set right
				flag |= wire.FlagCompleted
				break
			}
		default:
			completion = newCompletion()
			m.pendingCompletions[index] = completion
		}
	}

	if err := m.protocol.Write(message, flag); err != nil {
		return nil, nil, err
	}
	return ack, completion, nil
}

func (m *Machine) OneWayWrite(message wire.Message) error {
	_, _, err := m.Write(message, 0)
	return err
}

func (m *Machine) WriteWithCompletion(message wire.Message) (*CompletionFuture, error) {
	_, completionFut, err := m.Write(message, 0)
	if completionFut == nil {
		return nil, fmt.Errorf("completion requested for message type that does not get completions")
	}
	return completionFut, err
}

func (m *Machine) WriteWithAck(message wire.Message) (*AckFuture, error) {
	ackFut, _, err := m.Write(message, wire.FlagRequiresAck)
	return ackFut, err
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
			completion, ok := m.pendingCompletions[msg.CompletionMessage.EntryIndex]
			if !ok {
				m.log.Error().Uint32("index", msg.CompletionMessage.EntryIndex).Msg("failed to find pending completion at index")
				continue
			}
			if err := completion.complete(&msg.CompletionMessage); err != nil {
				m.log.Error().Err(err).Msg("failed to process completion")
				continue
			}
			m.log.Debug().Uint32("index", msg.CompletionMessage.EntryIndex).Msg("processed completion")
		case wire.EntryAckMessageType:
			msg := msg.(*wire.EntryAckMessage)
			ack, ok := m.pendingAcks[msg.EntryAckMessage.EntryIndex]
			if !ok {
				m.log.Error().Uint32("index", msg.EntryAckMessage.EntryIndex).Msg("failed to find pending ack at index")
				continue
			}
			if err := ack.ack(); err != nil {
				m.log.Error().Err(err).Msg("Failed to process ack")
				continue
			}
			m.log.Debug().Uint32("index", msg.EntryAckMessage.EntryIndex).Msg("processed ack")
		default:
			m.log.Error().Stringer("type", msg.Type()).Msg("unexpected non-completion non-ack message during invocation")
			continue
		}
	}
}
