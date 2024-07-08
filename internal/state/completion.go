package state

import (
	"context"
	"fmt"
	"sync"

	"github.com/restatedev/sdk-go/generated/proto/protocol"
	"github.com/restatedev/sdk-go/internal/wire"
	"google.golang.org/protobuf/proto"
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

func (m *Machine) Write(message proto.Message, flag wire.Flag) (*AckFuture, *CompletionFuture, error) {
	index := m.entryIndex
	ack := m.pendingAcks[index]
	completion := m.pendingCompletions[index]

	if flag.Ack() && ack == nil {
		ack = newAck()
		m.pendingAcks[index] = ack
	}

	if completion == nil {
		switch message := message.(type) {
		case *protocol.SetStateEntryMessage, *protocol.ClearStateEntryMessage,
			*protocol.ClearAllStateEntryMessage, *protocol.CompleteAwakeableEntryMessage,
			*protocol.OneWayCallEntryMessage:
			// don't need completion, nil is ok
		case *protocol.GetStateEntryMessage:
			completion = newCompletion()
			if message.Result == nil {
				m.pendingCompletions[index] = completion
				break
			} else {
				// we are already completed; ensure flag is set right
				flag |= wire.FlagCompleted
				break
			}
		case *protocol.GetStateKeysEntryMessage:
			completion = newCompletion()
			if message.Result == nil {
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

func (m *Machine) OneWayWrite(message proto.Message) error {
	_, _, err := m.Write(message, 0)
	return err
}

func (m *Machine) WriteWithCompletion(message proto.Message) (*CompletionFuture, error) {
	_, ack, err := m.Write(message, 0)
	if ack == nil {
		return nil, fmt.Errorf("completion requested for message type that does not get completions")
	}
	return ack, err
}

func (m *Machine) WriteWithAck(message proto.Message) (*AckFuture, error) {
	completion, _, err := m.Write(message, wire.FlagRequiresAck)
	return completion, err
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
			completion, ok := m.pendingCompletions[msg.Payload.EntryIndex]
			if !ok {
				m.log.Error().Uint32("index", msg.Payload.EntryIndex).Msg("failed to find pending completion at index")
				continue
			}
			if err := completion.complete(&msg.Payload); err != nil {
				m.log.Error().Err(err).Msg("failed to process completion")
				continue
			}
			m.log.Debug().Uint32("index", msg.Payload.EntryIndex).Msg("processed completion")
		case wire.EntryAckMessageType:
			msg := msg.(*wire.EntryAckMessage)
			ack, ok := m.pendingAcks[msg.Payload.EntryIndex]
			if !ok {
				m.log.Error().Uint32("index", msg.Payload.EntryIndex).Msg("failed to find pending ack at index")
				continue
			}
			if err := ack.ack(); err != nil {
				m.log.Error().Err(err).Msg("Failed to process ack")
				continue
			}
			m.log.Debug().Uint32("index", msg.Payload.EntryIndex).Msg("processed ack")
		default:
			m.log.Error().Stringer("type", msg.Type()).Msg("unexpected non-completion non-ack message during invocation")
			continue
		}
	}
}
