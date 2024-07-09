package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/generated/proto/protocol"
	"github.com/restatedev/sdk-go/internal/wire"
)

var (
	_ restate.Service = (*serviceProxy)(nil)
	_ restate.Object  = (*serviceProxy)(nil)
	_ restate.Call    = (*serviceCall)(nil)
)

// service proxy only works as an extension to context
// to implement other services function calls
type serviceProxy struct {
	*Context
	service string
	key     string
}

func (c *serviceProxy) Method(fn string) restate.Call {
	return &serviceCall{
		Context: c.Context,
		service: c.service,
		key:     c.key,
		method:  fn,
	}
}

type serviceCall struct {
	*Context
	service string
	key     string
	method  string
}

// Do makes a call and wait for the response
func (c *serviceCall) Do(input any, output any) error {
	return c.machine.doDynCall(c.service, c.key, c.method, input, output)
}

// Send runs a call in the background after delay duration
func (c *serviceCall) Send(body any, delay time.Duration) error {
	return c.machine.sendCall(c.service, c.key, c.method, body, delay)
}

func (m *Machine) doDynCall(service, key, method string, input, output any) error {
	params, err := json.Marshal(input)
	if err != nil {
		return err
	}

	bytes, err := m.doCall(service, key, method, params)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(bytes, output); err != nil {
		// TODO: is this should be a terminal error or not?
		return restate.TerminalError(fmt.Errorf("failed to decode response (%s): %w", string(bytes), err))
	}

	return nil
}

func (m *Machine) doCall(service, key, method string, params []byte) ([]byte, error) {
	m.log.Debug().Str("service", service).Str("method", method).Str("key", key).Msg("executing sync call")

	return replayOrNew(
		m,
		wire.CallEntryMessageType,
		func(entry *wire.CallEntryMessage) ([]byte, error) {
			if entry.CallEntryMessage.ServiceName != service ||
				entry.CallEntryMessage.Key != key ||
				entry.CallEntryMessage.HandlerName != method ||
				!bytes.Equal(entry.CallEntryMessage.Parameter, params) {
				return nil, errEntryMismatch
			}

			switch result := entry.CallEntryMessage.Result.(type) {
			case *protocol.CallEntryMessage_Failure:
				return nil, ErrorFromFailure(result.Failure)
			case *protocol.CallEntryMessage_Value:
				return result.Value, nil
			}

			return nil, restate.TerminalError(fmt.Errorf("sync call entry  had invalid result: %v", entry.CallEntryMessage.Result), restate.ErrProtocolViolation)
		}, func() ([]byte, error) {
			return m._doCall(service, key, method, params)
		})
}

func (m *Machine) _doCall(service, key, method string, params []byte) ([]byte, error) {
	msg := &wire.CallEntryMessage{
		CallEntryMessage: protocol.CallEntryMessage{
			ServiceName: service,
			HandlerName: method,
			Parameter:   params,
			Key:         key,
		},
	}
	completionFut, err := m.WriteWithCompletion(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to send request message: %w", err)
	}

	completion, err := completionFut.Await(m.ctx)
	if err != nil {
		return nil, err
	}

	switch result := completion.Result.(type) {
	case *protocol.CompletionMessage_Empty:
		return nil, nil
	case *protocol.CompletionMessage_Failure:
		return nil, ErrorFromFailure(result.Failure)
	case *protocol.CompletionMessage_Value:
		return result.Value, nil
	}

	return nil, restate.TerminalError(fmt.Errorf("sync call completion had invalid result: %v", completion.Result), restate.ErrProtocolViolation)
}

func (c *Machine) sendCall(service, key, method string, body any, delay time.Duration) error {
	c.log.Debug().Str("service", service).Str("method", method).Str("key", key).Msg("executing async call")

	params, err := json.Marshal(body)
	if err != nil {
		return err
	}

	_, err = replayOrNew(
		c,
		wire.OneWayCallEntryMessageType,
		func(entry *wire.OneWayCallEntryMessage) (restate.Void, error) {
			if entry.OneWayCallEntryMessage.ServiceName != service ||
				entry.OneWayCallEntryMessage.Key != key ||
				entry.OneWayCallEntryMessage.HandlerName != method ||
				!bytes.Equal(entry.OneWayCallEntryMessage.Parameter, params) {
				return restate.Void{}, errEntryMismatch
			}

			return restate.Void{}, nil
		},
		func() (restate.Void, error) {
			return restate.Void{}, c._sendCall(service, key, method, params, delay)
		},
	)

	return err
}

func (c *Machine) _sendCall(service, key, method string, params []byte, delay time.Duration) error {
	var invokeTime uint64
	if delay != 0 {
		invokeTime = uint64(time.Now().Add(delay).UnixMilli())
	}

	err := c.OneWayWrite(&wire.OneWayCallEntryMessage{
		OneWayCallEntryMessage: protocol.OneWayCallEntryMessage{
			ServiceName: service,
			HandlerName: method,
			Parameter:   params,
			Key:         key,
			InvokeTime:  invokeTime,
		},
	})

	if err != nil {
		return fmt.Errorf("failed to send request message: %w", err)
	}

	return nil
}

func ErrorFromFailure(failure *protocol.Failure) error {
	return restate.TerminalError(fmt.Errorf(failure.Message), restate.Code(failure.Code))
}
