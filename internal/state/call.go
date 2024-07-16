package state

import (
	"bytes"
	"time"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/generated/proto/protocol"
	"github.com/restatedev/sdk-go/internal/futures"
	"github.com/restatedev/sdk-go/internal/wire"
)

type serviceCall struct {
	machine *Machine
	service string
	key     string
	method  string
}

// RequestFuture makes a call and returns a handle on the response
func (c *serviceCall) RequestFuture(input []byte) (restate.ResponseFuture[[]byte], error) {
	entry, entryIndex := c.machine.doCall(c.service, c.key, c.method, input)

	return futures.NewResponseFuture(c.machine.suspensionCtx, entry, entryIndex, func(err error) any { return c.machine.newProtocolViolation(entry, err) }), nil
}

// Request makes a call and blocks on the response
func (c *serviceCall) Request(input []byte) ([]byte, error) {
	fut, err := c.RequestFuture(input)
	if err != nil {
		return nil, err
	}
	return fut.Response()
}

// Send runs a call in the background after delay duration
func (c *serviceCall) Send(input []byte, delay time.Duration) error {
	c.machine.sendCall(c.service, c.key, c.method, input, delay)
	return nil
}

func (m *Machine) doCall(service, key, method string, params []byte) (*wire.CallEntryMessage, uint32) {
	entry, entryIndex := replayOrNew(
		m,
		func(entry *wire.CallEntryMessage) *wire.CallEntryMessage {
			if entry.ServiceName != service ||
				entry.Key != key ||
				entry.HandlerName != method ||
				!bytes.Equal(entry.Parameter, params) {
				panic(m.newEntryMismatch(&wire.CallEntryMessage{
					CallEntryMessage: protocol.CallEntryMessage{
						ServiceName: service,
						HandlerName: method,
						Parameter:   params,
						Key:         key,
					},
				}, entry))
			}

			return entry
		}, func() *wire.CallEntryMessage {
			return m._doCall(service, key, method, params)
		})
	return entry, entryIndex
}

func (m *Machine) _doCall(service, key, method string, params []byte) *wire.CallEntryMessage {
	msg := &wire.CallEntryMessage{
		CallEntryMessage: protocol.CallEntryMessage{
			ServiceName: service,
			HandlerName: method,
			Parameter:   params,
			Key:         key,
		},
	}
	m.Write(msg)

	return msg
}

func (m *Machine) sendCall(service, key, method string, body []byte, delay time.Duration) {
	_, _ = replayOrNew(
		m,
		func(entry *wire.OneWayCallEntryMessage) restate.Void {
			if entry.ServiceName != service ||
				entry.Key != key ||
				entry.HandlerName != method ||
				!bytes.Equal(entry.Parameter, body) {
				panic(m.newEntryMismatch(&wire.OneWayCallEntryMessage{
					OneWayCallEntryMessage: protocol.OneWayCallEntryMessage{
						ServiceName: service,
						HandlerName: method,
						Parameter:   body,
						Key:         key,
					},
				}, entry))
			}

			return restate.Void{}
		},
		func() restate.Void {
			m._sendCall(service, key, method, body, delay)
			return restate.Void{}
		},
	)
}

func (c *Machine) _sendCall(service, key, method string, params []byte, delay time.Duration) {
	var invokeTime uint64
	if delay != 0 {
		invokeTime = uint64(time.Now().Add(delay).UnixMilli())
	}

	c.Write(&wire.OneWayCallEntryMessage{
		OneWayCallEntryMessage: protocol.OneWayCallEntryMessage{
			ServiceName: service,
			HandlerName: method,
			Parameter:   params,
			Key:         key,
			InvokeTime:  invokeTime,
		},
	})
}
