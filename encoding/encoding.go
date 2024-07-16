package encoding

import (
	"encoding/json"
	"fmt"
	"reflect"

	"google.golang.org/protobuf/proto"
)

// Void is a placeholder to signify 'no value' where a type is otherwise needed
type Void struct{}

var (
	// BinaryCodec marshals []byte and unmarshals into *[]byte
	// In handlers, it uses a content type of application/octet-stream
	BinaryCodec PayloadCodec = binaryCodec{}
	// VoidCodec marshals anything into []byte(nil) and skips unmarshaling
	// In handlers, it requires that there is no input content-type and does not set an output content-type
	VoidCodec PayloadCodec = voidCodec{}
	// ProtoCodec marshals proto.Message and unmarshals into proto.Message or pointers to types that implement proto.Message
	// In handlers, it uses a content-type of application/proto
	ProtoCodec PayloadCodec = protoCodec{}
	// JSONCodec marshals any json.Marshallable type and unmarshals into any json.Unmarshallable type
	// In handlers, it uses a content-type of application/json
	JSONCodec PayloadCodec       = jsonCodec{}
	_         RestateMarshaler   = Void{}
	_         RestateUnmarshaler = Void{}
	_         RestateUnmarshaler = &Void{}
)

func (v Void) RestateUnmarshal(codec Codec, data []byte) error {
	return nil
}

func (v Void) InputPayload(codec Codec) *InputPayload {
	return &InputPayload{}
}

func (v Void) RestateMarshal(codec Codec) ([]byte, error) {
	return nil, nil
}

func (v Void) OutputPayload(codec Codec) *OutputPayload {
	return &OutputPayload{}
}

type RestateUnmarshaler interface {
	RestateUnmarshal(codec Codec, data []byte) error
	InputPayload(codec Codec) *InputPayload
}

func InputPayloadFor(codec PayloadCodec, i any) *InputPayload {
	ru, ok := i.(RestateUnmarshaler)
	if ok {
		return ru.InputPayload(codec)
	}
	return codec.InputPayload()
}

func OutputPayloadFor(codec PayloadCodec, o any) *OutputPayload {
	ru, ok := o.(RestateMarshaler)
	if ok {
		return ru.OutputPayload(codec)
	}
	return codec.OutputPayload()
}

func RestateMarshalerFor[O any]() (RestateMarshaler, bool) {
	var o O
	ru, ok := any(o).(RestateMarshaler)
	return ru, ok
}

// RestateMarshaler can be implemented by types that want to control their own marshaling
type RestateMarshaler interface {
	RestateMarshal(codec Codec) ([]byte, error)
	OutputPayload(codec Codec) *OutputPayload
}

type Codec interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

func Marshal(codec Codec, v any) ([]byte, error) {
	if marshaler, ok := v.(RestateMarshaler); ok {
		return marshaler.RestateMarshal(codec)
	}
	return codec.Marshal(v)
}

func Unmarshal(codec Codec, data []byte, v any) error {
	if marshaler, ok := v.(RestateUnmarshaler); ok {
		return marshaler.RestateUnmarshal(codec, data)
	}
	return codec.Unmarshal(data, v)
}

type PayloadCodec interface {
	InputPayload() *InputPayload
	OutputPayload() *OutputPayload
	Codec
}

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

type voidCodec struct{}

func (j voidCodec) InputPayload() *InputPayload {
	return &InputPayload{}
}

func (j voidCodec) OutputPayload() *OutputPayload {
	return &OutputPayload{}
}

func (j voidCodec) Unmarshal(data []byte, input any) (err error) {
	return nil
}

func (j voidCodec) Marshal(output any) ([]byte, error) {
	return nil, nil
}

type binaryCodec struct{}

func (j binaryCodec) InputPayload() *InputPayload {
	// Required false because 0 bytes is a valid input
	return &InputPayload{Required: false, ContentType: proto.String("application/octet-stream")}
}

func (j binaryCodec) OutputPayload() *OutputPayload {
	// SetContentTypeIfEmpty true because 0 bytes is a valid output
	return &OutputPayload{ContentType: proto.String("application/octet-stream"), SetContentTypeIfEmpty: true}
}

func (j binaryCodec) Unmarshal(data []byte, input any) (err error) {
	switch input := input.(type) {
	case *[]byte:
		*input = data
		return nil
	default:
		return fmt.Errorf("BinaryCodec.Unmarshal called with a type that is not *[]byte")
	}
}

func (j binaryCodec) Marshal(output any) ([]byte, error) {
	switch output := output.(type) {
	case []byte:
		return output, nil
	default:
		return nil, fmt.Errorf("BinaryCodec.Marshal called with a type that is not []byte")
	}
}

type jsonCodec struct{}

func (j jsonCodec) InputPayload() *InputPayload {
	return &InputPayload{Required: true, ContentType: proto.String("application/json")}
}

func (j jsonCodec) OutputPayload() *OutputPayload {
	return &OutputPayload{ContentType: proto.String("application/json")}
}

func (j jsonCodec) Unmarshal(data []byte, input any) (err error) {
	return json.Unmarshal(data, input)
}

func (j jsonCodec) Marshal(output any) ([]byte, error) {
	return json.Marshal(output)
}

type protoCodec struct{}

func (p protoCodec) InputPayload() *InputPayload {
	return &InputPayload{Required: true, ContentType: proto.String("application/proto")}
}

func (p protoCodec) OutputPayload() *OutputPayload {
	return &OutputPayload{ContentType: proto.String("application/proto")}
}

func (p protoCodec) Unmarshal(data []byte, input any) (err error) {
	switch input := input.(type) {
	case proto.Message:
		// called with a *Message
		return proto.Unmarshal(data, input)
	default:
		// we must support being called with a **Message where *Message is nil because this is the result of new(I) where I is a proto.Message
		// and calling with new(I) is really the only generic approach.
		value := reflect.ValueOf(input)
		if value.Kind() != reflect.Pointer || value.IsNil() || value.Elem().Kind() != reflect.Pointer {
			return fmt.Errorf("ProtoCodec.Unmarshal called with neither a proto.Message nor a non-nil pointer to a type that implements proto.Message.")
		}
		elem := value.Elem() // hopefully a *Message
		if elem.IsNil() {
			// allocate a &Message and swap this in
			elem.Set(reflect.New(elem.Type().Elem()))
		}
		switch elemI := elem.Interface().(type) {
		case proto.Message:
			return proto.Unmarshal(data, elemI)
		default:
			return fmt.Errorf("ProtoCodec.Unmarshal called with neither a proto.Message nor a non-nil pointer to a type that implements proto.Message.")
		}
	}
}

func (p protoCodec) Marshal(output any) (data []byte, err error) {
	switch output := output.(type) {
	case proto.Message:
		return proto.Marshal(output)
	default:
		return nil, fmt.Errorf("ProtoCodec.Marshal called with a type that is not a proto.Message")
	}
}
