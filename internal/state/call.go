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
	return c.machine.doCall(c.service, c.method, key, input, output)
}

// Send runs a call in the background after delay duration
func (c *serviceCall) Send(key string, body any, delay time.Duration) error {
	return c.machine.sendCall(c.service, c.method, key, body, delay)
}

func (c *Machine) makeRequest(key string, body any) ([]byte, error) {

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

func (c *Machine) doCall(service, method, key string, input, output any) error {
	params, err := c.makeRequest(key, input)
	if err != nil {
		return err
	}

	bytes, err := replayOrNew(
		c,
		wire.InvokeEntryMessageType,
		func(entry *wire.InvokeEntryMessage) ([]byte, error) {
			if entry.Payload.ServiceName != service ||
				entry.Payload.MethodName != method ||
				!bytes.Equal(entry.Payload.Parameter, params) {
				return nil, errEntryMismatch
			}

			switch result := entry.Payload.Result.(type) {
			case *protocol.InvokeEntryMessage_Failure:
				return nil, fmt.Errorf("[%d] %s", result.Failure.Code, result.Failure.Message)
			case *protocol.InvokeEntryMessage_Value:
				return result.Value, nil
			}

			return nil, errUnreachable
		}, func() ([]byte, error) {
			return c._doCall(service, method, params)
		})

	if err != nil {
		return err
	}

	if output == nil {
		return nil
	}

	if err := json.Unmarshal(bytes, output); err != nil {
		return restate.TerminalError(fmt.Errorf("failed to decode response: %w", err))
	}

	return nil
}

func (c *Machine) _doCall(service, method string, params []byte) ([]byte, error) {
	err := c.protocol.Write(&protocol.InvokeEntryMessage{
		ServiceName: service,
		MethodName:  method,
		Parameter:   params,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to send request message: %w", err)
	}

	response, err := c.protocol.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read response message: %w", err)
	}

	if response.Type() != wire.CompletionMessageType {
		return nil, ErrUnexpectedMessage
	}

	//response := msg.(*wire.CompletionMessage)

	completion := response.(*wire.CompletionMessage)

	var output []byte
	switch value := completion.Payload.Result.(type) {
	case *protocol.CompletionMessage_Empty:
		return nil, nil
	case *protocol.CompletionMessage_Failure:
		// the get state entry message is not failable so this should
		// never happen
		return nil, fmt.Errorf("[%d] %s", value.Failure.Code, value.Failure.Message)
	case *protocol.CompletionMessage_Value:
		output = value.Value
	}

	var rpcResponse dynrpc.RpcResponse
	if err := proto.Unmarshal(output, &rpcResponse); err != nil {
		return nil, fmt.Errorf("failed to decode rpc response: %w", err)
	}

	return rpcResponse.Response.MarshalJSON()
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
