package encoding

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVoid(t *testing.T) {
	codecs := map[string]Codec{
		"json":      JSONCodec,
		"proto":     ProtoCodec,
		"protojson": ProtoJSONCodec,
		"binary":    BinaryCodec,
	}
	for name, codec := range codecs {
		t.Run(name, func(t *testing.T) {
			bytes, err := Marshal(codec, Void{})
			if err != nil {
				t.Fatal(err)
			}

			if bytes != nil {
				t.Fatalf("expected bytes to be nil, found %v", bytes)
			}

			if err := Unmarshal(codec, []byte{1, 2, 3}, &Void{}); err != nil {
				t.Fatal(err)
			}

			if err := Unmarshal(codec, []byte{1, 2, 3}, Void{}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

var jsonSchemaCases = []struct {
	object any
	schema string
}{
	{
		object: "abc",
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"string"}`,
	},
	{
		object: 123,
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"integer"}`,
	},
	{
		object: 1.1,
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"number"}`,
	},
	{
		object: struct {
			Foo string `json:"foo"`
		}{},
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","properties":{"foo":{"type":"string"}},"additionalProperties":false,"type":"object","required":["foo"]}`,
	},
	{
		object: recursive{},
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://github.com/restatedev/sdk-go/encoding/recursive","$defs":{"recursive":{"$ref":"#"}},"properties":{"inner":{"$ref":"#/$defs/recursive"}},"additionalProperties":false,"type":"object","required":["inner"]}`,
	},
	{
		object: nestedRecursiveA{},
		schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://github.com/restatedev/sdk-go/encoding/nested-recursive-a","$defs":{"nestedRecursiveA":{"$ref":"#"},"nestedRecursiveB":{"properties":{"inner":{"$ref":"#/$defs/nestedRecursiveC"}},"additionalProperties":false,"type":"object","required":["inner"]},"nestedRecursiveC":{"properties":{"inner":{"$ref":"#/$defs/nestedRecursiveA"}},"additionalProperties":false,"type":"object","required":["inner"]}},"properties":{"inner":{"$ref":"#/$defs/nestedRecursiveB"}},"additionalProperties":false,"type":"object","required":["inner"]}`,
	},
}

type recursive struct {
	Inner *recursive `json:"inner"`
}

type nestedRecursiveA struct {
	Inner *nestedRecursiveB `json:"inner"`
}

type nestedRecursiveB struct {
	Inner *nestedRecursiveC `json:"inner"`
}

type nestedRecursiveC struct {
	Inner *nestedRecursiveA `json:"inner"`
}

func TestGenerateJsonSchema(t *testing.T) {
	for _, test := range jsonSchemaCases {
		t.Run(reflect.TypeOf(test.object).String(), func(t *testing.T) {
			schema := generateJsonSchema(test.object)
			data, err := json.Marshal(schema)
			require.NoError(t, err)
			require.Equal(t, test.schema, string(data))
		})
	}
}

func TestIsUnstableSerialization(t *testing.T) {
	tests := []struct {
		name     string
		codec    Codec
		expected bool
	}{
		{"JSONCodec", JSONCodec, false},
		{"ProtoCodec", ProtoCodec, false},
		{"ProtoJSONCodec", ProtoJSONCodec, true},
		{"BinaryCodec", BinaryCodec, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsUnstableSerialization(tt.codec)
			require.Equal(t, tt.expected, result, "IsUnstableSerialization(%s) = %v, want %v", tt.name, result, tt.expected)
		})
	}
}
