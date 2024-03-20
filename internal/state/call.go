package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/muhamadazmy/restate-sdk-go"
	"github.com/muhamadazmy/restate-sdk-go/generated/proto/dynrpc"
	"github.com/muhamadazmy/restate-sdk-go/generated/proto/protocol"
	"github.com/muhamadazmy/restate-sdk-go/internal/wire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	_ restate.Service = (*serviceProxy)(nil)
	_ restate.Call    = (*serviceCall)(nil)
)

// service proxy only works as an extension to context
// to implement other services function calls
type serviceProxy struct {
	*Context
	service string
}

func (c *serviceProxy) Method(fn string) restate.Call {
	return &serviceCall{
		Context: c.Context,
		service: c.service,
		method:  fn,
	}
}

type serviceCall struct {
	*Context
	service string
	method  string
}

// Do makes a call and wait for the response
func (c *serviceCall) Do(key string, input any, output any) error {
	return c.machine.doDynCall(c.service, c.method, key, input, output)
}

// Send runs a call in the background after delay duration
func (c *serviceCall) Send(key string, body any, delay time.Duration) error {
	return c.machine.sendCall(c.service, c.method, key, body, delay)
}

func (m *Machine) makeRequest(key string, body any) ([]byte, error) {

	input, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	params := &dynrpc.RpcRequest{
		Key:     key,
		Request: &structpb.Value{},
	}

	if err := params.Request.UnmarshalJSON(input); err != nil {
		return nil, err
	}

	return proto.Marshal(params)
}

func (m *Machine) doDynCall(service, method, key string, input, output any) error {
	m.log.Debug().Str("service", service).Str("method", method).Msg("in do call")

	params, err := m.makeRequest(key, input)
	if err != nil {
		return err
	}

	bytes, err := m.doCall(service, method, params)
	if err != nil {
		return err
	}

	var rpcResponse dynrpc.RpcResponse
	if err := proto.Unmarshal(bytes, &rpcResponse); err != nil {
		return fmt.Errorf("failed to decode rpc response: %w", err)
	}

	js, err := rpcResponse.Response.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to process response payload")
	}

	if output == nil {
		return nil
	}

	if err := json.Unmarshal(js, output); err != nil {
		// TODO: is this should be a terminal error or not?
		return restate.TerminalError(fmt.Errorf("failed to decode response (%s): %w", string(bytes), err))
	}

	return nil
}

func (m *Machine) doCall(service, method string, params []byte) ([]byte, error) {
	return replayOrNew(
		m,
		wire.InvokeEntryMessageType,
		func(entry *wire.InvokeEntryMessage) ([]byte, error) {
			if entry.Payload.ServiceName != service ||
				entry.Payload.MethodName != method ||
				!bytes.Equal(entry.Payload.Parameter, params) {
				return nil, errEntryMismatch
			}

			switch result := entry.Payload.Result.(type) {
			case *protocol.InvokeEntryMessage_Failure:
				return nil, restate.WithErrorCode(fmt.Errorf(result.Failure.Message), restate.Code(result.Failure.Code))
			case *protocol.InvokeEntryMessage_Value:
				return result.Value, nil
			}

			return nil, errUnreachable
		}, func() ([]byte, error) {
			return m._doCall(service, method, params)
		})
}

func (m *Machine) _doCall(service, method string, params []byte) ([]byte, error) {
	err := m.protocol.Write(&protocol.InvokeEntryMessage{
		ServiceName: service,
		MethodName:  method,
		Parameter:   params,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to send request message: %w", err)
	}

	response, err := m.protocol.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read response message: %w", err)
	}

	if response.Type() != wire.CompletionMessageType {
		return nil, wire.ErrUnexpectedMessage
	}

	completion := response.(*wire.CompletionMessage)

	switch result := completion.Payload.Result.(type) {
	case *protocol.CompletionMessage_Empty:
		return nil, nil
	case *protocol.CompletionMessage_Failure:
		return nil, restate.WithErrorCode(fmt.Errorf(result.Failure.Message), restate.Code(result.Failure.Code))
	case *protocol.CompletionMessage_Value:
		return result.Value, nil
	}

	return nil, errUnreachable
}

func (c *Machine) sendCall(service, method, key string, body any, delay time.Duration) error {
	params, err := c.makeRequest(key, body)
	if err != nil {
		return err
	}

	_, err = replayOrNew(
		c,
		wire.BackgroundInvokeEntryMessageType,
		func(entry *wire.BackgroundInvokeEntryMessage) (restate.Void, error) {
			if entry.Payload.ServiceName != service ||
				entry.Payload.MethodName != method ||
				!bytes.Equal(entry.Payload.Parameter, params) {
				return restate.Void{}, errEntryMismatch
			}

			return restate.Void{}, nil
		},
		func() (restate.Void, error) {
			return restate.Void{}, c._sendCall(service, method, params, delay)
		},
	)

	return err
}

func (c *Machine) _sendCall(service, method string, params []byte, delay time.Duration) error {
	var invokeTime uint64
	if delay != 0 {
		invokeTime = uint64(time.Now().Add(delay).UnixMilli())
	}

	err := c.protocol.Write(&protocol.BackgroundInvokeEntryMessage{
		ServiceName: service,
		MethodName:  method,
		Parameter:   params,
		InvokeTime:  invokeTime,
	})

	if err != nil {
		return fmt.Errorf("failed to send request message: %w", err)
	}

	return nil
}
