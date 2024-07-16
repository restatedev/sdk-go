package encoding

import (
	"encoding/json"
	"fmt"
	"reflect"

	"google.golang.org/protobuf/proto"
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

type VoidCodec struct{}

var _ PayloadCodec = VoidCodec{}

func (j VoidCodec) InputPayload() *InputPayload {
	return &InputPayload{}
}

func (j VoidCodec) OutputPayload() *OutputPayload {
	return &OutputPayload{}
}

func (j VoidCodec) Unmarshal(data []byte, input any) (err error) {
	return nil
}

func (j VoidCodec) Marshal(output any) ([]byte, error) {
	return nil, nil
}

type PairCodec struct {
	Input  PayloadCodec
	Output PayloadCodec
}

var _ PayloadCodec = PairCodec{}

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
		return VoidCodec{}
	case inputVoid:
		return PairCodec{Input: VoidCodec{}, Output: nil}
	case outputVoid:
		return PairCodec{Input: nil, Output: VoidCodec{}}
	default:
		return nil
	}
}

type BinaryCodec struct{}

var _ PayloadCodec = BinaryCodec{}

func (j BinaryCodec) InputPayload() *InputPayload {
	return &InputPayload{Required: true, ContentType: proto.String("application/octet-stream")}
}

func (j BinaryCodec) OutputPayload() *OutputPayload {
	return &OutputPayload{ContentType: proto.String("application/octet-stream")}
}

func (j BinaryCodec) Unmarshal(data []byte, input any) (err error) {
	switch input := input.(type) {
	case *[]byte:
		*input = data
		return nil
	default:
		return fmt.Errorf("BinaryCodec.Unmarshal called with a type that is not *[]byte")
	}
}

func (j BinaryCodec) Marshal(output any) ([]byte, error) {
	switch output := output.(type) {
	case []byte:
		return output, nil
	default:
		return nil, fmt.Errorf("BinaryCodec.Marshal called with a type that is not []byte")
	}
}

type JSONCodec struct{}

var _ PayloadCodec = JSONCodec{}

func (j JSONCodec) InputPayload() *InputPayload {
	return &InputPayload{Required: true, ContentType: proto.String("application/json")}
}

func (j JSONCodec) OutputPayload() *OutputPayload {
	return &OutputPayload{ContentType: proto.String("application/json")}
}

func (j JSONCodec) Unmarshal(data []byte, input any) (err error) {
	return json.Unmarshal(data, &input)
}

func (j JSONCodec) Marshal(output any) ([]byte, error) {
	return json.Marshal(output)
}

type ProtoCodec struct{}

var _ PayloadCodec = ProtoCodec{}

func (p ProtoCodec) InputPayload() *InputPayload {
	return &InputPayload{Required: true, ContentType: proto.String("application/proto")}
}

func (p ProtoCodec) OutputPayload() *OutputPayload {
	return &OutputPayload{ContentType: proto.String("application/proto")}
}

func (p ProtoCodec) Unmarshal(data []byte, input any) (err error) {
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

func (p ProtoCodec) Marshal(output any) (data []byte, err error) {
	switch output := output.(type) {
	case proto.Message:
		return proto.Marshal(output)
	default:
		return nil, fmt.Errorf("ProtoCodec.Marshal called with a type that is not a proto.Message")
	}
}
