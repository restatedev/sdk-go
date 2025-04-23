package protojsonschema

import (
	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"google.golang.org/protobuf/reflect/protoreflect"
)

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
		return &jsonschema.Schema{
			Type: "object",
			Properties: orderedmap.New[string, *jsonschema.Schema](orderedmap.WithInitialData[string, *jsonschema.Schema](
				orderedmap.Pair[string, *jsonschema.Schema]{
					Key:   "@type",
					Value: &jsonschema.Schema{Type: "string"},
				},
				orderedmap.Pair[string, *jsonschema.Schema]{
					Key:   "value",
					Value: &jsonschema.Schema{Type: "string", Format: "binary"},
				},
			)),
			AdditionalProperties: jsonschema.TrueSchema,
		}
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
