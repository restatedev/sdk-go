package encoding

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"runtime/debug"

	"github.com/invopop/jsonschema"
	"github.com/restatedev/sdk-go/encoding/internal/protojsonschema"
	"github.com/restatedev/sdk-go/encoding/internal/util"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var (
	// BinaryCodec marshals []byte and unmarshals into *[]byte
	// In handlers, it uses a content type of application/octet-stream
	BinaryCodec Codec = binaryCodec{}
	// ProtoCodec marshals proto.Message and unmarshals into proto.Message or pointers to types that implement proto.Message
	// In handlers, it uses a content-type of application/proto
	ProtoCodec Codec = protoCodec{}
	// ProtoJSONCodec marshals proto.Message and unmarshals into proto.Message or pointers to types that implement proto.Message
	// It uses the protojson package to marshal and unmarshal
	// In handlers, it uses a content-type of application/json
	ProtoJSONCodec Codec = protoJSONCodec{}
	// JSONCodec marshals any json.Marshallable type and unmarshals into any json.Unmarshallable type
	// In handlers, it uses a content-type of application/json
	JSONCodec Codec = jsonCodec{
		genJsonSchema: generateJsonSchema,
	}

	_ RestateMarshaler   = Void{}
	_ RestateUnmarshaler = Void{}
	_ RestateUnmarshaler = &Void{}
)

// Void is a placeholder to signify 'no value' where a type is otherwise needed
// It implements [RestateMarshaler] and [RestateUnmarshaler] to ensure that no marshaling or unmarshaling ever happens
// on this type.
type Void struct{}

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

// RestateUnmarshaler can be implemented by types that want to control their own unmarshaling
type RestateUnmarshaler interface {
	RestateUnmarshal(codec Codec, data []byte) error
	InputPayload(codec Codec) *InputPayload
}

// InputPayloadFor determines the InputPayload for the type stored in i, respecting [RestateUnmarshaler] implementors
func InputPayloadFor(codec Codec, i any) *InputPayload {
	if ru, ok := i.(RestateUnmarshaler); ok {
		return ru.InputPayload(codec)
	}
	return &InputPayload{
		Required:    inputRequired(codec),
		ContentType: contentTypePtr(codec),
		JsonSchema:  jsonSchemaFor(codec, i),
	}
}

// OutputPayloadFor determines the OutputPayload for the type stored in o, respecting [RestateMarshaler] implementors
func OutputPayloadFor(codec Codec, o any) *OutputPayload {
	if rm, ok := o.(RestateMarshaler); ok {
		return rm.OutputPayload(codec)
	}
	return &OutputPayload{
		ContentType:           contentTypePtr(codec),
		SetContentTypeIfEmpty: setContentTypeIfEmpty(codec),
		JsonSchema:            jsonSchemaFor(codec, o),
	}
}

// RestateMarshaler can be implemented by types that want to control their own marshaling
// Marshaling *must* be deterministic
type RestateMarshaler interface {
	RestateMarshal(codec Codec) ([]byte, error)
	OutputPayload(codec Codec) *OutputPayload
}

// Codec is a mechanism for serialising and deserialising a wide range of types.
// Care should be taken to ensure that only valid types are passed to a codec, eg proto.Message for [ProtoCodec].
// Codecs *must* marshal deterministically, such that a round trip of []byte -> any -> []byte leaves the bytes unchanged.
//
// A codec may additionally describe the payloads it produces for service discovery by
// implementing the optional [CodecMetadata] (content-type + JSON schema),
// [CodecInputMetadata] and [CodecOutputMetadata] interfaces.
type Codec interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

// CodecMetadata is an optional interface a [Codec] may implement to describe its payloads
// for service discovery. When not implemented, no content-type or schema is advertised.
type CodecMetadata interface {
	// ContentType returns the MIME type of the serialised bytes, or "" for none.
	ContentType() string
	// JsonSchema returns a JSON schema for the type of v, or nil for none.
	JsonSchema(v any) any
}

// CodecInputMetadata is an optional interface a [Codec] may implement to declare, for service
// discovery, whether a request body is required. When not implemented, input is required.
type CodecInputMetadata interface {
	InputRequired() bool
}

// CodecOutputMetadata is an optional interface a [Codec] may implement to declare, for
// service discovery, that the output content-type should be set even when the output body
// is empty. When not implemented, it is not set for empty bodies.
type CodecOutputMetadata interface {
	SetContentTypeIfEmpty() bool
}

func inputRequired(codec Codec) bool {
	if x, ok := codec.(CodecInputMetadata); ok {
		return x.InputRequired()
	}
	return true
}

func setContentTypeIfEmpty(codec Codec) bool {
	if x, ok := codec.(CodecOutputMetadata); ok {
		return x.SetContentTypeIfEmpty()
	}
	return false
}

func contentTypePtr(codec Codec) *string {
	if m, ok := codec.(CodecMetadata); ok {
		if ct := m.ContentType(); ct != "" {
			return proto.String(ct)
		}
	}
	return nil
}

func jsonSchemaFor(codec Codec, v any) any {
	if m, ok := codec.(CodecMetadata); ok {
		return m.JsonSchema(v)
	}
	return nil
}

// Marshal converts its input v into []byte using the codec, respecting [RestateMarshaler] implementors
func Marshal(codec Codec, v any) ([]byte, error) {
	if marshaler, ok := v.(RestateMarshaler); ok {
		return marshaler.RestateMarshal(codec)
	}
	return codec.Marshal(v)
}

// Unmarshal converts its input data and stores the result into v using the codec, respecting [RestateUnmarshaler] implementors
// v is expected to be a mutable concrete type, generally a pointer.
func Unmarshal(codec Codec, data []byte, v any) error {
	if marshaler, ok := v.(RestateUnmarshaler); ok {
		return marshaler.RestateUnmarshal(codec, data)
	}
	return codec.Unmarshal(data, v)
}

// NonDeterministicSerializer is an interface that codecs can implement to indicate
// they may produce non-deterministic output for the same input.
type NonDeterministicSerializer interface {
	IsNonDeterministic() bool
}

// IsNonDeterministicSerialization returns true if the codec may produce non-deterministic output.
// This is true for ProtoJSONCodec (protojson does not guarantee deterministic output)
// and any codec implementing NonDeterministicSerializer that returns true from IsNonDeterministic().
func IsNonDeterministicSerialization(codec Codec) bool {
	if nonDeterministic, ok := codec.(NonDeterministicSerializer); ok {
		return nonDeterministic.IsNonDeterministic()
	}

	return false
}

// InputPayload is provided to Restate upon handler discovery, to teach the ingress how to validate incoming
// request bodies.
type InputPayload struct {
	Required    bool        `json:"required"`
	ContentType *string     `json:"contentType,omitempty"`
	JsonSchema  interface{} `json:"jsonSchema,omitempty"`
}

// OutputPayload is provided to Restate upon handler discovery, to teach the ingress how to annotate outgoing
// response bodies.
type OutputPayload struct {
	ContentType           *string     `json:"contentType,omitempty"`
	SetContentTypeIfEmpty bool        `json:"setContentTypeIfEmpty"`
	JsonSchema            interface{} `json:"jsonSchema,omitempty"`
}

type binaryCodec struct{}

func (j binaryCodec) ContentType() string { return "application/octet-stream" }
func (j binaryCodec) JsonSchema(any) any  { return nil }

// InputRequired is false because 0 bytes is a valid input.
func (j binaryCodec) InputRequired() bool { return false }

// SetContentTypeIfEmpty is true because 0 bytes is a valid output.
func (j binaryCodec) SetContentTypeIfEmpty() bool { return true }

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

func JSONCodecWithCustomSchemaGenerator(genJsonSchema func(v any) interface{}) Codec {
	return jsonCodec{genJsonSchema}
}

type jsonCodec struct {
	genJsonSchema func(v any) interface{}
}

func (j jsonCodec) ContentType() string  { return "application/json" }
func (j jsonCodec) JsonSchema(v any) any { return j.genJsonSchema(v) }

func (j jsonCodec) Unmarshal(data []byte, input any) (err error) {
	return json.Unmarshal(data, input)
}

func (j jsonCodec) Marshal(output any) ([]byte, error) {
	return json.Marshal(output)
}

type protoCodec struct{}

func (p protoCodec) ContentType() string { return "application/proto" }
func (p protoCodec) JsonSchema(any) any  { return nil }

// SetContentTypeIfEmpty is true because 0 bytes is a valid output.
func (p protoCodec) SetContentTypeIfEmpty() bool { return true }

func (p protoCodec) Unmarshal(data []byte, input any) (err error) {
	switch input := input.(type) {
	case proto.Message:
		// called with a *Message
		return proto.Unmarshal(data, input)
	default:
		msg, err := allocateProtoMessage("ProtoCodec", input)
		if err != nil {
			return err
		}
		return proto.Unmarshal(data, msg)
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

type protoJSONCodec struct{}

func (j protoJSONCodec) IsNonDeterministic() bool {
	// protojson does not guarantee deterministic output:
	// https://github.com/golang/protobuf/issues/1373
	return true
}

func (j protoJSONCodec) generateProtoJsonSchema(v any) interface{} {
	_, msgOk := v.(protoreflect.ProtoMessage)
	if !msgOk {
		_, enumOk := v.(protoreflect.Enum)
		if !enumOk {
			return nil
		}
	}
	return protojsonschema.GenerateSchema(v)
}

func (j protoJSONCodec) ContentType() string  { return "application/json" }
func (j protoJSONCodec) JsonSchema(v any) any { return j.generateProtoJsonSchema(v) }

func (j protoJSONCodec) Unmarshal(data []byte, input any) (err error) {
	switch input := input.(type) {
	case proto.Message:
		// called with a *Message
		return protojson.Unmarshal(data, input)
	default:
		msg, err := allocateProtoMessage("ProtoJSONCodec", input)
		if err != nil {
			return err
		}
		return protojson.Unmarshal(data, msg)
	}
}

func (j protoJSONCodec) Marshal(output any) ([]byte, error) {
	switch output := output.(type) {
	case proto.Message:
		return protojson.Marshal(output)
	default:
		return nil, fmt.Errorf("ProtoJSONCodec.Marshal called with a type that is not a proto.Message")
	}
}

// we must support being called with a **Message where *Message is nil because this is the result of new(I) where I is a proto.Message
// new(I) is really the only generic approach for allocating. Hitting this code path is meaningfully slower
// for protobuf decoding, but the effect is minimal for protojson
func allocateProtoMessage(codecName string, input any) (proto.Message, error) {
	value := reflect.ValueOf(input)
	if value.Kind() != reflect.Pointer || value.IsNil() || value.Elem().Kind() != reflect.Pointer {
		return nil, fmt.Errorf("%s.Unmarshal called with neither a proto.Message nor a non-nil pointer to a type that implements proto.Message.", codecName)
	}
	elem := value.Elem() // hopefully a *Message
	if elem.IsNil() {
		// allocate a &Message and swap this in
		elem.Set(reflect.New(elem.Type().Elem()))
	}
	switch elemI := elem.Interface().(type) {
	case proto.Message:
		return elemI, nil
	default:
		return nil, fmt.Errorf("%s.Unmarshal called with neither a proto.Message nor a non-nil pointer to a type that implements proto.Message.", codecName)
	}
}

func generateJsonSchema(v any) (schema interface{}) {
	defer func() {
		if err := recover(); err != nil {
			slog.Warn(`Error when trying to generate schema for the given type, you will see a generic schema in the UI Playground for this type.
If you can't fix the issue, disable json schema generation using encoding.JSONCodecWithCustomSchemaGenerator(func(v any) interface{} { return make(map[string]interface{}) })`,
				"typeName", reflect.TypeOf(v),
				"cause", err,
			)

			debug.PrintStack()

			schema = map[string]string{}
		}
	}()

	reflector := jsonschema.Reflector{
		// Unfortunately we can't enable this due to a panic bug https://github.com/invopop/jsonschema/issues/163
		// So we use expandSchema instead, which has the same effect but without the panic
		// ExpandedStruct: true,
	}
	return util.ExpandSchema(reflector.Reflect(v))
}
