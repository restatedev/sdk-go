package protojsonschema_test

import (
	"testing"

	"github.com/restatedev/sdk-go/encoding/internal/protojsonschema"
	helloworld "github.com/restatedev/sdk-go/examples/codegen/proto"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type testCase struct {
	desc   string
	msg    proto.Message
	schema string
}

var testCases = []testCase{
	{
		desc:   "HelloRequest",
		msg:    &helloworld.HelloRequest{},
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://github.com/restatedev/sdk-go/examples/codegen/proto/helloworld.-hello-request","$defs":{"helloworld.HelloRequest":{"$ref":"#"}},"properties":{"name":{"type":"string"}},"additionalProperties":false,"type":"object"}`,
	},
	{
		desc:   "WatchRequest",
		msg:    &helloworld.WatchRequest{},
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://github.com/restatedev/sdk-go/examples/codegen/proto/helloworld.-watch-request","$defs":{"helloworld.WatchRequest":{"$ref":"#"}},"properties":{"timeoutMillis":{"type":"integer"}},"additionalProperties":false,"type":"object"}`,
	},
	{
		desc:   "Test",
		msg:    &helloworld.Test{},
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://github.com/restatedev/sdk-go/examples/codegen/proto/helloworld.-test","$defs":{"helloworld.Test":{"$ref":"#"},"helloworld.WatchRequest":{"properties":{"timeoutMillis":{"type":"integer"}},"additionalProperties":false,"type":"object"}},"properties":{"inner":{"$ref":"#/$defs/helloworld.Test"},"primitive":{"type":"string"},"anotherInner":{"$ref":"#/$defs/helloworld.WatchRequest"}},"additionalProperties":false,"type":"object"}`,
	},
	{
		desc:   "google.protobuf.Duration",
		msg:    &durationpb.Duration{},
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/durationpb/google.protobuf.-duration","type":"string","pattern":"^[-\\+]?([0-9]+\\.?[0-9]*|\\.[0-9]+)s$","format":"regex"}`,
	},
	{
		desc:   "google.protobuf.Timestamp",
		msg:    &timestamppb.Timestamp{},
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/timestamppb/google.protobuf.-timestamp","type":"string","format":"date-time"}`,
	},
	{
		desc:   "google.protobuf.Empty",
		msg:    &emptypb.Empty{},
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/emptypb/google.protobuf.-empty","additionalProperties":false,"type":"object"}`,
	},
	{
		desc:   "google.protobuf.Any",
		msg:    &anypb.Any{},
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/anypb/google.protobuf.-any","properties":{"@type":{"type":"string"},"value":{"type":"string","format":"binary"}},"additionalProperties":true,"type":"object"}`,
	},
	{
		desc:   "google.protobuf.FieldMask",
		msg:    &fieldmaskpb.FieldMask{},
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/fieldmaskpb/google.protobuf.-field-mask","type":"string"}`,
	},
	{
		desc:   "google.protobuf.Struct",
		msg:    &structpb.Struct{},
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/structpb/google.protobuf.-struct","additionalProperties":true,"type":"object"}`,
	},
	{
		desc:   "google.protobuf.Value",
		msg:    &structpb.Value{},
		schema: `true`,
	},
	{
		desc:   "google.protobuf.StringValue",
		msg:    wrapperspb.String(""),
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/wrapperspb/google.protobuf.-string-value","type":"string"}`,
	},
	{
		desc:   "google.protobuf.BytesValue",
		msg:    wrapperspb.Bytes(nil),
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/wrapperspb/google.protobuf.-bytes-value","type":"string","format":"binary"}`,
	},
	{
		desc:   "google.protobuf.BoolValue",
		msg:    wrapperspb.Bool(false),
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/wrapperspb/google.protobuf.-bool-value","type":"boolean"}`,
	},
	{
		desc:   "google.protobuf.DoubleValue",
		msg:    wrapperspb.Double(0.0),
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/wrapperspb/google.protobuf.-double-value","oneOf":[{"type":"number"},{"type":"string"}]}`,
	},
	{
		desc:   "google.protobuf.Int64Value",
		msg:    wrapperspb.Int64(0),
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/wrapperspb/google.protobuf.-int64-value","oneOf":[{"type":"number"},{"type":"string"}]}`,
	},
	{
		desc:   "google.protobuf.UInt64Value",
		msg:    wrapperspb.UInt64(0),
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/wrapperspb/google.protobuf.u-int64-value","oneOf":[{"type":"number"},{"type":"string"}]}`,
	},
	{
		desc:   "google.protobuf.FloatValue",
		msg:    wrapperspb.Float(0.0),
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/wrapperspb/google.protobuf.-float-value","oneOf":[{"type":"number"},{"type":"string"}]}`,
	},
	{
		desc:   "google.protobuf.Int32Value",
		msg:    wrapperspb.Int32(0),
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/wrapperspb/google.protobuf.-int32-value","type":"number"}`,
	},
	{
		desc:   "google.protobuf.Uint32Value",
		msg:    wrapperspb.UInt32(0),
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/wrapperspb/google.protobuf.u-int32-value","type":"number"}`,
	},
}

func TestSchema(t *testing.T) {
	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			schema := protojsonschema.GenerateSchema(testCase.msg)
			bytes, err := schema.MarshalJSON()
			require.NoError(t, err)
			require.Equal(t, testCase.schema, string(bytes))
		})

	}
}
