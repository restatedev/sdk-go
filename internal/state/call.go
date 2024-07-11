package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/generated/proto/protocol"
	"github.com/restatedev/sdk-go/internal/futures"
	"github.com/restatedev/sdk-go/internal/wire"
)

var (
	_ restate.ServiceClient     = (*serviceProxy)(nil)
	_ restate.ServiceSendClient = (*serviceSendProxy)(nil)
	_ restate.CallClient        = (*serviceCall)(nil)
	_ restate.SendClient        = (*serviceSend)(nil)
)

type serviceProxy struct {
	*Context
	service string
	key     string
}

func (c *serviceProxy) Method(fn string) restate.CallClient {
	return &serviceCall{
		Context: c.Context,
		service: c.service,
		key:     c.key,
		method:  fn,
	}
}

type serviceSendProxy struct {
	*Context
	service string
	key     string
	delay   time.Duration
}

func (c *serviceSendProxy) Method(fn string) restate.SendClient {
	return &serviceSend{
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
func (c *serviceCall) Request(input any) restate.ResponseFuture {
	if msg, err := c.machine.doDynCall(c.service, c.key, c.method, input); err != nil {
		return futures.NewFailedResponseFuture(c.ctx, err)
	} else {
		return futures.NewResponseFuture(c.ctx, msg)
	}
}

type serviceSend struct {
	*Context
	service string
	key     string
	method  string

	delay time.Duration
}

// Send runs a call in the background after delay duration
func (c *serviceSend) Request(input any) error {
	return c.machine.sendCall(c.service, c.key, c.method, input, c.delay)
}

func (m *Machine) doDynCall(service, key, method string, input any) (*wire.CallEntryMessage, error) {
	params, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	return m.doCall(service, key, method, params)
}

func (m *Machine) doCall(service, key, method string, params []byte) (*wire.CallEntryMessage, error) {
	m.log.Debug().Str("service", service).Str("method", method).Str("key", key).Msg("executing sync call")

	return replayOrNew(
		m,
		func(entry *wire.CallEntryMessage) (*wire.CallEntryMessage, error) {
			if entry.ServiceName != service ||
				entry.Key != key ||
				entry.HandlerName != method ||
				!bytes.Equal(entry.Parameter, params) {
				return nil, errEntryMismatch
			}

			return entry, nil
		}, func() (*wire.CallEntryMessage, error) {
			return m._doCall(service, key, method, params)
		})
}

func (m *Machine) _doCall(service, key, method string, params []byte) (*wire.CallEntryMessage, error) {
	msg := &wire.CallEntryMessage{
		CallEntryMessage: protocol.CallEntryMessage{
			ServiceName: service,
			HandlerName: method,
			Parameter:   params,
			Key:         key,
		},
	}
	if err := m.Write(msg); err != nil {
		return nil, fmt.Errorf("failed to send request message: %w", err)
	}

	return msg, nil
}

func (c *Machine) sendCall(service, key, method string, body any, delay time.Duration) error {
	c.log.Debug().Str("service", service).Str("method", method).Str("key", key).Msg("executing async call")

	params, err := json.Marshal(body)
	if err != nil {
		return err
	}

	_, err = replayOrNew(
		c,
		func(entry *wire.OneWayCallEntryMessage) (restate.Void, error) {
			if entry.ServiceName != service ||
				entry.Key != key ||
				entry.HandlerName != method ||
				!bytes.Equal(entry.Parameter, params) {
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

	err := c.Write(&wire.OneWayCallEntryMessage{
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
