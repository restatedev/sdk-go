package encoding

import (
	"encoding/json"

	"google.golang.org/protobuf/proto"
)

type InputPayload struct {
	Required    bool        `json:"required"`
	ContentType *string     `json:"contentType,omitempty"`
	JsonSchema  interface{} `json:"jsonSchema,omitempty"`
}

type OutputPayload struct {
	ContentType           *string     `json:"contentType,omitempty"`
	SetContentTypeIfEmpty bool        `json:"setContentTypeIfEmpty"`
	JsonSchema            interface{} `json:"jsonSchema,omitempty"`
}

type JSONDecoder[I any] struct{}

func (j JSONDecoder[I]) InputPayload() *InputPayload {
	return &InputPayload{Required: true, ContentType: proto.String("application/json")}
}

func (j JSONDecoder[I]) Decode(data []byte) (input I, err error) {
	err = json.Unmarshal(data, &input)
	return
}

type JSONEncoder[O any] struct{}

func (j JSONEncoder[O]) OutputPayload() *OutputPayload {
	return &OutputPayload{ContentType: proto.String("application/json")}
}

func (j JSONEncoder[O]) Encode(output O) ([]byte, error) {
	return json.Marshal(output)
}

type MessagePointer[I any] interface {
	proto.Message
	*I
}

type ProtoDecoder[I any, IP MessagePointer[I]] struct{}

func (p ProtoDecoder[I, IP]) InputPayload() *InputPayload {
	return &InputPayload{Required: true, ContentType: proto.String("application/proto")}
}

func (p ProtoDecoder[I, IP]) Decode(data []byte) (input IP, err error) {
	// Unmarshal expects a non-nil pointer to a proto.Message implementing struct
	// hence we must have a type parameter for the struct itself (I) and here we allocate
	// a non-nil pointer of type IP
	input = IP(new(I))
	err = proto.Unmarshal(data, input)
	return
}

type ProtoEncoder[O proto.Message] struct{}

func (p ProtoEncoder[O]) OutputPayload() *OutputPayload {
	return &OutputPayload{ContentType: proto.String("application/proto")}
}

func (p ProtoEncoder[O]) Encode(output O) ([]byte, error) {
	return proto.Marshal(output)
}
