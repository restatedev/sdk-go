package restate

import (
	"encoding/json"
	"fmt"

	"github.com/restatedev/sdk-go/encoding"
)

// Void is a placeholder used usually for functions that their signature require that
// you accept an input or return an output but the function implementation does not
// require them
type Void struct{}

type VoidDecoder struct{}

func (v VoidDecoder) InputPayload() *encoding.InputPayload {
	return &encoding.InputPayload{}
}

func (v VoidDecoder) Decode(data []byte) (input Void, err error) {
	if len(data) > 0 {
		err = fmt.Errorf("restate.Void decoder expects no request data")
	}
	return
}

type VoidEncoder struct{}

func (v VoidEncoder) OutputPayload() *encoding.OutputPayload {
	return &encoding.OutputPayload{}
}

func (v VoidEncoder) Encode(output Void) ([]byte, error) {
	return nil, nil
}

type serviceHandler[I any, O any] struct {
	fn      ServiceHandlerFn[I, O]
	decoder Decoder[I]
	encoder Encoder[O]
}

// NewJSONServiceHandler create a new handler for a service using JSON encoding
func NewJSONServiceHandler[I any, O any](fn ServiceHandlerFn[I, O]) *serviceHandler[I, O] {
	return &serviceHandler[I, O]{
		fn:      fn,
		decoder: encoding.JSONDecoder[I]{},
		encoder: encoding.JSONEncoder[O]{},
	}
}

// NewProtoServiceHandler create a new handler for a service using protobuf encoding
// Input and output type must both be pointers that satisfy proto.Message
func NewProtoServiceHandler[I any, O any, IP encoding.MessagePointer[I], OP encoding.MessagePointer[O]](fn ServiceHandlerFn[IP, OP]) *serviceHandler[IP, OP] {
	return &serviceHandler[IP, OP]{
		fn:      fn,
		decoder: encoding.ProtoDecoder[I, IP]{},
		encoder: encoding.ProtoEncoder[OP]{},
	}
}

// NewServiceHandlerWithEncoders create a new handler for a service using a custom encoder/decoder implementation
func NewServiceHandlerWithEncoders[I any, O any](fn ServiceHandlerFn[I, O], decoder Decoder[I], encoder Encoder[O]) *serviceHandler[I, O] {
	return &serviceHandler[I, O]{
		fn:      fn,
		decoder: decoder,
		encoder: encoder,
	}
}

func (h *serviceHandler[I, O]) Call(ctx Context, bytes []byte) ([]byte, error) {
	input, err := h.decoder.Decode(bytes)
	if err != nil {
		return nil, TerminalError(fmt.Errorf("request could not be decoded into handler input type: %w", err))
	}

	output, err := h.fn(
		ctx,
		input,
	)
	if err != nil {
		return nil, err
	}

	bytes, err = h.encoder.Encode(output)
	if err != nil {
		return nil, TerminalError(fmt.Errorf("failed to serialize output: %w", err))
	}

	return bytes, nil
}

func (h *serviceHandler[I, O]) InputPayload() *encoding.InputPayload {
	return h.decoder.InputPayload()
}

func (h *serviceHandler[I, O]) OutputPayload() *encoding.OutputPayload {
	return h.encoder.OutputPayload()
}

func (h *serviceHandler[I, O]) sealed() {}

type objectHandler[I any, O any] struct {
	fn ObjectHandlerFn[I, O]
}

func NewObjectHandler[I any, O any](fn ObjectHandlerFn[I, O]) *objectHandler[I, O] {
	return &objectHandler[I, O]{
		fn: fn,
	}
}

func (h *objectHandler[I, O]) Call(ctx ObjectContext, bytes []byte) ([]byte, error) {
	input := new(I)

	if len(bytes) > 0 {
		// use the zero value if there is no input data at all
		if err := json.Unmarshal(bytes, input); err != nil {
			return nil, TerminalError(fmt.Errorf("request doesn't match handler signature: %w", err))
		}
	}

	output, err := h.fn(
		ctx,
		*input,
	)
	if err != nil {
		return nil, err
	}

	bytes, err = json.Marshal(output)
	if err != nil {
		return nil, TerminalError(fmt.Errorf("failed to serialize output: %w", err))
	}

	return bytes, nil
}

func (h *objectHandler[I, O]) sealed() {}
