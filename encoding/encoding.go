package encoding

import (
	"encoding/json"
	"fmt"
	"reflect"

	"google.golang.org/protobuf/proto"
)

var (
	BinaryCodec PayloadCodec = binaryCodec{}
	VoidCodec   PayloadCodec = voidCodec{}
	ProtoCodec  PayloadCodec = protoCodec{}
	JSONCodec   PayloadCodec = jsonCodec{}
	_           PayloadCodec = PairCodec{}
)

type Void struct{}

type Codec interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

type PayloadCodec interface {
	Codec
	InputPayload() *InputPayload
	OutputPayload() *OutputPayload
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

type PairCodec struct {
	Input  PayloadCodec
	Output PayloadCodec
}

func (w PairCodec) InputPayload() *InputPayload {
	return w.Input.InputPayload()
}

func (w PairCodec) OutputPayload() *OutputPayload {
	return w.Output.OutputPayload()
}

func (w PairCodec) Unmarshal(data []byte, v any) error {
	return w.Input.Unmarshal(data, v)
}

func (w PairCodec) Marshal(v any) ([]byte, error) {
	return w.Output.Marshal(v)
}

func MergeCodec(base, overlay PayloadCodec) PayloadCodec {
	switch {
	case base == nil && overlay == nil:
		return nil
	case base == nil:
		return overlay
	case overlay == nil:
		return base
	}

	basePair, baseOk := base.(PairCodec)
	overlayPair, overlayOk := overlay.(PairCodec)

	switch {
	case baseOk && overlayOk:
		return PairCodec{
			Input:  MergeCodec(basePair.Input, overlayPair.Input),
			Output: MergeCodec(basePair.Output, overlayPair.Output),
		}
	case baseOk:
		return PairCodec{
			Input:  MergeCodec(basePair.Input, overlay),
			Output: MergeCodec(basePair.Output, overlay),
		}
	case overlayOk:
		return PairCodec{
			Input:  MergeCodec(base, overlayPair.Input),
			Output: MergeCodec(base, overlayPair.Output),
		}
	default:
		// just two non-pairs; keep base
		return base
	}
}

func PartialVoidCodec[I any, O any]() PayloadCodec {
	var input I
	var output O
	_, inputVoid := any(input).(Void)
	_, outputVoid := any(output).(Void)
	switch {
	case inputVoid && outputVoid:
		return VoidCodec
	case inputVoid:
		return PairCodec{Input: VoidCodec, Output: nil}
	case outputVoid:
		return PairCodec{Input: nil, Output: VoidCodec}
	default:
		return nil
	}
}

type binaryCodec struct{}

func (j binaryCodec) InputPayload() *InputPayload {
	return &InputPayload{Required: true, ContentType: proto.String("application/octet-stream")}
}

func (j binaryCodec) OutputPayload() *OutputPayload {
	return &OutputPayload{ContentType: proto.String("application/octet-stream")}
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
	return json.Unmarshal(data, &input)
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
