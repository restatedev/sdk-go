package state

import (
	"encoding/json"
	"fmt"

	"github.com/muhamadazmy/restate-sdk-go/generated/proto/dynrpc"
	"github.com/muhamadazmy/restate-sdk-go/generated/proto/protocol"
	"github.com/muhamadazmy/restate-sdk-go/internal/wire"
	"github.com/muhamadazmy/restate-sdk-go/router"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	_ router.Service = (*serviceProxy)(nil)
	_ router.Call    = (*serviceCall)(nil)
)

// service proxy only works as an extension to context
// to implement other services function calls
type serviceProxy struct {
	*Context
	service string
}

func (c *serviceProxy) Method(fn string) router.Call {
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

func (c *serviceCall) Do(key string, body any) ([]byte, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

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

	input, err = proto.Marshal(params)
	if err != nil {
		return nil, err
	}

	err = c.protocol.Write(&protocol.InvokeEntryMessage{
		ServiceName: c.service,
		MethodName:  c.method,
		Parameter:   input,
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
