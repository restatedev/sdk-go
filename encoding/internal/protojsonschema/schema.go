package protojsonschema

import (
	"reflect"

	"github.com/invopop/jsonschema"
	"github.com/restatedev/sdk-go/encoding/internal/util"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var protoMessageType = reflect.TypeOf((*protoreflect.ProtoMessage)(nil)).Elem()
var protoEnumType = reflect.TypeOf((*protoreflect.Enum)(nil)).Elem()

func descriptor(typ reflect.Type) protoreflect.Descriptor {
	if typ.Implements(protoEnumType) {
		zero := reflect.Zero(typ).Interface().(protoreflect.Enum)
		return zero.Descriptor()
	}

	pointerTyp := reflect.PointerTo(typ)
	if pointerTyp.Implements(protoMessageType) {
		zero := reflect.Zero(pointerTyp).Interface().(protoreflect.ProtoMessage)
		return zero.ProtoReflect().Descriptor()
	}

	return nil
}

func GenerateSchema(v any) *jsonschema.Schema {
	reflector := jsonschema.Reflector{
		// Unfortunately we can't enable this due to a panic bug https://github.com/invopop/jsonschema/issues/163
		// So we use ExpandSchema instead, which has the same effect but without the panic
		// ExpandedStruct: true,
		KeyNamer: func(fieldName string) string {
			return jsonCamelCase(fieldName)
		},
		Lookup: func(typ reflect.Type) jsonschema.ID {
			desc := descriptor(typ)
			if desc == nil {
				return jsonschema.EmptyID
			}

			id := jsonschema.ID("https://" + typ.PkgPath())
			if err := id.Validate(); err != nil {
				return jsonschema.EmptyID
			}

			return id.Add(string(desc.FullName()))
		},
		Mapper: func(typ reflect.Type) *jsonschema.Schema {
			desc := descriptor(typ)
			if desc == nil {
				return nil
			}

			schemaFn, ok := wellKnownToSchemaFns[string(desc.FullName())]
			if !ok {
				return nil
			}

			return schemaFn(desc)
		},
		Namer: func(typ reflect.Type) string {
			desc := descriptor(typ)
			if desc == nil {
				return ""
			}

			return string(desc.FullName())
		},
	}
	return util.ExpandSchema(reflector.Reflect(v))
}

// jsonCamelCase converts a snake_case identifier to a camelCase identifier,
// according to the protobuf JSON specification.
func jsonCamelCase(s string) string {
	var b []byte
	var wasUnderscore bool
	for i := 0; i < len(s); i++ { // proto identifiers are always ASCII
		c := s[i]
		if c != '_' {
			if wasUnderscore && 'a' <= c && c <= 'z' {
				c -= 'a' - 'A' // convert to uppercase
			}
			b = append(b, c)
		}
		wasUnderscore = c == '_'
	}
	return string(b)
}
