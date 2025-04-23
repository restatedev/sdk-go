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
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://github.com/restatedev/sdk-go/examples/codegen/proto/helloworld.HelloRequest","$defs":{"helloworld.HelloRequest":{"$ref":"#"}},"properties":{"name":{"type":"string"}},"additionalProperties":false,"type":"object"}`,
	},
	{
		desc:   "WatchRequest",
		msg:    &helloworld.WatchRequest{},
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://github.com/restatedev/sdk-go/examples/codegen/proto/helloworld.WatchRequest","$defs":{"helloworld.WatchRequest":{"$ref":"#"}},"properties":{"timeoutMillis":{"type":"integer"}},"additionalProperties":false,"type":"object"}`,
	},
	{
		desc:   "google.protobuf.Duration",
		msg:    &durationpb.Duration{},
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/durationpb/google.protobuf.Duration","type":"string","pattern":"^[-\\+]?([0-9]+\\.?[0-9]*|\\.[0-9]+)s$","format":"regex"}`,
	},
	{
		desc:   "google.protobuf.Timestamp",
		msg:    &timestamppb.Timestamp{},
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/timestamppb/google.protobuf.Timestamp","type":"string","format":"date-time"}`,
	},
	{
		desc:   "google.protobuf.Empty",
		msg:    &emptypb.Empty{},
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/emptypb/google.protobuf.Empty","additionalProperties":false,"type":"object"}`,
	},
	{
		desc:   "google.protobuf.Any",
		msg:    &anypb.Any{},
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/anypb/google.protobuf.Any","properties":{"@type":{"type":"string"},"value":{"type":"string","format":"binary"}},"additionalProperties":true,"type":"object"}`,
	},
	{
		desc:   "google.protobuf.FieldMask",
		msg:    &fieldmaskpb.FieldMask{},
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/fieldmaskpb/google.protobuf.FieldMask","type":"string"}`,
	},
	{
		desc:   "google.protobuf.Struct",
		msg:    &structpb.Struct{},
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/structpb/google.protobuf.Struct","additionalProperties":true,"type":"object"}`,
	},
	{
		desc:   "google.protobuf.Value",
		msg:    &structpb.Value{},
		schema: `true`,
	},
	{
		desc:   "google.protobuf.StringValue",
		msg:    wrapperspb.String(""),
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/wrapperspb/google.protobuf.StringValue","type":"string"}`,
	},
	{
		desc:   "google.protobuf.BytesValue",
		msg:    wrapperspb.Bytes(nil),
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/wrapperspb/google.protobuf.BytesValue","type":"string","format":"binary"}`,
	},
	{
		desc:   "google.protobuf.BoolValue",
		msg:    wrapperspb.Bool(false),
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/wrapperspb/google.protobuf.BoolValue","type":"boolean"}`,
	},
	{
		desc:   "google.protobuf.DoubleValue",
		msg:    wrapperspb.Double(0.0),
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/wrapperspb/google.protobuf.DoubleValue","oneOf":[{"type":"number"},{"type":"string"}]}`,
	},
	{
		desc:   "google.protobuf.Int64Value",
		msg:    wrapperspb.Int64(0),
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/wrapperspb/google.protobuf.Int64Value","oneOf":[{"type":"number"},{"type":"string"}]}`,
	},
	{
		desc:   "google.protobuf.UInt64Value",
		msg:    wrapperspb.UInt64(0),
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/wrapperspb/google.protobuf.UInt64Value","oneOf":[{"type":"number"},{"type":"string"}]}`,
	},
	{
		desc:   "google.protobuf.FloatValue",
		msg:    wrapperspb.Float(0.0),
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/wrapperspb/google.protobuf.FloatValue","oneOf":[{"type":"number"},{"type":"string"}]}`,
	},
	{
		desc:   "google.protobuf.Int32Value",
		msg:    wrapperspb.Int32(0),
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/wrapperspb/google.protobuf.Int32Value","type":"number"}`,
	},
	{
		desc:   "google.protobuf.Uint32Value",
		msg:    wrapperspb.UInt32(0),
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://google.golang.org/protobuf/types/known/wrapperspb/google.protobuf.UInt32Value","type":"number"}`,
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
