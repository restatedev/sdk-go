package protojsonschema

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// mustParseSchema builds a *jsonschema.Schema from a JSON literal, panicking on
// error. Used for static well-known schemas so we never have to reference the
// concrete ordered-map type backing jsonschema.Schema.Properties — that type is
// an implementation detail of invopop/jsonschema and has changed between
// versions (wk8/go-ordered-map -> pb33f/ordered-map), which would otherwise
// pin us to a specific fork.
func mustParseSchema(s string) *jsonschema.Schema {
	schema := &jsonschema.Schema{}
	if err := json.Unmarshal([]byte(s), schema); err != nil {
		panic(err)
	}
	return schema
}

var wellKnownToSchemaFns = map[string]func(protoreflect.Descriptor) *jsonschema.Schema{
	"google.protobuf.Duration": func(d protoreflect.Descriptor) *jsonschema.Schema {
		return &jsonschema.Schema{
			Type:    "string",
			Format:  "regex",
			Pattern: `^[-\+]?([0-9]+\.?[0-9]*|\.[0-9]+)s$`,
		}
	},
	"google.protobuf.Timestamp": func(d protoreflect.Descriptor) *jsonschema.Schema {
		return &jsonschema.Schema{
			Type:   "string",
			Format: "date-time",
		}
	},
	"google.protobuf.Empty": func(d protoreflect.Descriptor) *jsonschema.Schema {
		return &jsonschema.Schema{
			Type:                 "object",
			AdditionalProperties: jsonschema.FalseSchema,
		}
	},
	"google.protobuf.Any": func(d protoreflect.Descriptor) *jsonschema.Schema {
		return mustParseSchema(`{
			"type": "object",
			"properties": {
				"@type": {"type": "string"},
				"value": {"type": "string", "format": "binary"}
			},
			"additionalProperties": true
		}`)
	},
	"google.protobuf.FieldMask": func(d protoreflect.Descriptor) *jsonschema.Schema {
		return &jsonschema.Schema{
			Type: "string",
		}
	},

	"google.protobuf.Struct": func(d protoreflect.Descriptor) *jsonschema.Schema {
		return &jsonschema.Schema{
			Type:                 "object",
			AdditionalProperties: jsonschema.TrueSchema,
		}
	},
	"google.protobuf.Value": func(d protoreflect.Descriptor) *jsonschema.Schema {
		return jsonschema.TrueSchema
	},
	"google.protobuf.NullValue": func(d protoreflect.Descriptor) *jsonschema.Schema {
		return &jsonschema.Schema{
			Type: "null",
		}
	},
	"google.protobuf.StringValue": func(d protoreflect.Descriptor) *jsonschema.Schema {
		return &jsonschema.Schema{
			Type: "string",
		}
	},
	"google.protobuf.BytesValue": func(d protoreflect.Descriptor) *jsonschema.Schema {
		return &jsonschema.Schema{
			Type:   "string",
			Format: "binary",
		}
	},
	"google.protobuf.BoolValue": func(d protoreflect.Descriptor) *jsonschema.Schema {
		return &jsonschema.Schema{
			Type: "boolean",
		}
	},
	"google.protobuf.DoubleValue": google64BitNumberValue,
	"google.protobuf.Int64Value":  google64BitNumberValue,
	"google.protobuf.UInt64Value": google64BitNumberValue,
	"google.protobuf.FloatValue":  google64BitNumberValue,
	"google.protobuf.Int32Value":  google32BitNumberValue,
	"google.protobuf.UInt32Value": google32BitNumberValue,
}

var google64BitNumberValue = func(d protoreflect.Descriptor) *jsonschema.Schema {
	return &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			&jsonschema.Schema{Type: "number"},
			&jsonschema.Schema{Type: "string"},
		},
	}
}
var google32BitNumberValue = func(d protoreflect.Descriptor) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "number",
	}
}
