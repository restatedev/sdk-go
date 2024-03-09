package router

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/muhamadazmy/restate-sdk-go/generated/proto/dynrpc"
	"google.golang.org/protobuf/types/known/structpb"
)

type UnKeyedHandler struct {
	fn     reflect.Value
	input  reflect.Type
	output reflect.Type
}

// NewUnKeyedHandler create a new handler for an `un-keyed` function
func NewUnKeyedHandler[I any, O any](fn UnKeyedHandlerFn[I, O]) *UnKeyedHandler {
	return &UnKeyedHandler{
		fn:     reflect.ValueOf(fn),
		input:  reflect.TypeFor[I](),
		output: reflect.TypeFor[O](),
	}
}

func (h *UnKeyedHandler) Call(ctx Context, request *dynrpc.RpcRequest) (*dynrpc.RpcResponse, error) {
	bytes, err := request.Request.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("request is not valid json: %w", err)
	}

	input := reflect.New(h.input)

	if err := json.Unmarshal(bytes, input.Interface()); err != nil {
		return nil, fmt.Errorf("request doesn't match handler signature: %w", err)
	}

	// we are sure about the fn signature so it's safe to do this
	output := h.fn.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		input.Elem(),
	})

	outI := output[0].Interface()
	errI := output[1].Interface()
	if errI != nil {
		return nil, errI.(error)
	}

	bytes, err = json.Marshal(outI)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize output: %w", err)
	}

	var response dynrpc.RpcResponse
	response.Response = &structpb.Value{}

	if err := response.Response.UnmarshalJSON(bytes); err != nil {
		return nil, err
	}

	return &response, nil
}

func (h *UnKeyedHandler) sealed() {}

type KeyedHandler struct {
	fn     reflect.Value
	input  reflect.Type
	output reflect.Type
}

func NewKeyedHandler[I any, O any](fn KeyedHandlerFn[I, O]) *KeyedHandler {
	return &KeyedHandler{
		fn:     reflect.ValueOf(fn),
		input:  reflect.TypeFor[I](),
		output: reflect.TypeFor[O](),
	}
}

func (h *KeyedHandler) Call(ctx Context, request *dynrpc.RpcRequest) (*dynrpc.RpcResponse, error) {
	bytes, err := request.Request.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("request is not valid json: %w", err)
	}

	input := reflect.New(h.input)

	if err := json.Unmarshal(bytes, input.Interface()); err != nil {
		return nil, fmt.Errorf("request doesn't match handler signature: %w", err)
	}

	// we are sure about the fn signature so it's safe to do this
	output := h.fn.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(request.Key),
		input.Elem(),
	})

	outI := output[0].Interface()
	errI := output[1].Interface()
	if errI != nil {
		return nil, errI.(error)
	}

	bytes, err = json.Marshal(outI)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize output: %w", err)
	}

	var response dynrpc.RpcResponse
	response.Response = &structpb.Value{}

	if err := response.Response.UnmarshalJSON(bytes); err != nil {
		return nil, err
	}

	return &response, nil
}

func (h *KeyedHandler) sealed() {}
