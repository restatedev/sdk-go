package state

import (
	"bytes"
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/restatedev/sdk-go/encoding"
	protocol "github.com/restatedev/sdk-go/generated/dev/restate/service"
	"github.com/restatedev/sdk-go/internal/futures"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/wire"
)

type Client struct {
	options options.ClientOptions
	machine *Machine
	service string
	key     string
	method  string
}

// RequestFuture makes a call and returns a handle on the response
func (c *Client) RequestFuture(input any, opts ...options.RequestOption) DecodingResponseFuture {
	o := options.RequestOptions{}
	for _, opt := range opts {
		opt.BeforeRequest(&o)
	}

	bytes, err := encoding.Marshal(c.options.Codec, input)
	if err != nil {
		panic(c.machine.newCodecFailure(fmt.Errorf("failed to marshal RequestFuture input: %w", err)))
	}

	entry, entryIndex := c.machine.doCall(c.service, c.key, c.method, o.Headers, bytes)

	return DecodingResponseFuture{
		futures.NewResponseFuture(c.machine.suspensionCtx, entry, entryIndex, func(err error) any { return c.machine.newProtocolViolation(entry, err) }),
		c.machine,
		c.options,
	}
}

type DecodingResponseFuture struct {
	*futures.ResponseFuture
	machine *Machine
	options options.ClientOptions
}

func (d DecodingResponseFuture) Response(output any) (err error) {
	bytes, err := d.ResponseFuture.Response()
	if err != nil {
		return err
	}

	if err := encoding.Unmarshal(d.options.Codec, bytes, output); err != nil {
		panic(d.machine.newCodecFailure(fmt.Errorf("failed to unmarshal Call response into output: %w", err)))
	}

	return nil
}

// Request makes a call and blocks on the response
func (c *Client) Request(input any, output any, opts ...options.RequestOption) error {
	return c.RequestFuture(input, opts...).Response(output)
}

// Send runs a call in the background after delay duration
func (c *Client) Send(input any, opts ...options.SendOption) {
	o := options.SendOptions{}
	for _, opt := range opts {
		opt.BeforeSend(&o)
	}

	bytes, err := encoding.Marshal(c.options.Codec, input)
	if err != nil {
		panic(c.machine.newCodecFailure(fmt.Errorf("failed to marshal Send input: %w", err)))
	}
	c.machine.sendCall(c.service, c.key, c.method, o.Headers, bytes, o.Delay)
	return
}

func (m *Machine) doCall(service, key, method string, headersMap map[string]string, params []byte) (*wire.CallEntryMessage, uint32) {
	headers := headersToProto(headersMap)

	entry, entryIndex := replayOrNew(
		m,
		func(entry *wire.CallEntryMessage) *wire.CallEntryMessage {
			if entry.ServiceName != service ||
				entry.Key != key ||
				entry.HandlerName != method ||
				!headersEqual(entry.Headers, headers) ||
				!bytes.Equal(entry.Parameter, params) {
				panic(m.newEntryMismatch(&wire.CallEntryMessage{
					CallEntryMessage: protocol.CallEntryMessage{
						ServiceName: service,
						HandlerName: method,
						Headers:     headers,
						Parameter:   params,
						Key:         key,
					},
				}, entry))
			}

			return entry
		}, func() *wire.CallEntryMessage {
			return m._doCall(service, key, method, headers, params)
		})
	return entry, entryIndex
}

func (m *Machine) _doCall(service, key, method string, headers []*protocol.Header, params []byte) *wire.CallEntryMessage {
	msg := &wire.CallEntryMessage{
		CallEntryMessage: protocol.CallEntryMessage{
			ServiceName: service,
			HandlerName: method,
			Parameter:   params,
			Headers:     headers,
			Key:         key,
		},
	}
	m.Write(msg)

	return msg
}

func headersEqual(left, right []*protocol.Header) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].Key != right[i].Key || left[i].Value != right[i].Value {
			return false
		}
	}
	return true
}

func headersToProto(headers map[string]string) []*protocol.Header {
	if len(headers) == 0 {
		return nil
	}

	h := make([]*protocol.Header, 0, len(headers))
	for k, v := range headers {
		h = append(h, &protocol.Header{Key: k, Value: v})
	}

	slices.SortFunc(h, func(a, b *protocol.Header) int {
		return cmp.Compare(a.Key, b.Key)
	})

	return h
}

func (m *Machine) sendCall(service, key, method string, headersMap map[string]string, body []byte, delay time.Duration) {
	headers := headersToProto(headersMap)

	_, _ = replayOrNew(
		m,
		func(entry *wire.OneWayCallEntryMessage) encoding.Void {
			if entry.ServiceName != service ||
				entry.Key != key ||
				entry.HandlerName != method ||
				!headersEqual(entry.Headers, headers) ||
				!bytes.Equal(entry.Parameter, body) {
				panic(m.newEntryMismatch(&wire.OneWayCallEntryMessage{
					OneWayCallEntryMessage: protocol.OneWayCallEntryMessage{
						ServiceName: service,
						HandlerName: method,
						Headers:     headers,
						Parameter:   body,
						Key:         key,
					},
				}, entry))
			}

			return encoding.Void{}
		},
		func() encoding.Void {
			m._sendCall(service, key, method, headers, body, delay)
			return encoding.Void{}
		},
	)
}

func (c *Machine) _sendCall(service, key, method string, headers []*protocol.Header, params []byte, delay time.Duration) {
	var invokeTime uint64
	if delay != 0 {
		invokeTime = uint64(time.Now().Add(delay).UnixMilli())
	}

	c.Write(&wire.OneWayCallEntryMessage{
		OneWayCallEntryMessage: protocol.OneWayCallEntryMessage{
			ServiceName: service,
			HandlerName: method,
			Headers:     headers,
			Parameter:   params,
			Key:         key,
			InvokeTime:  invokeTime,
		},
	})
}
