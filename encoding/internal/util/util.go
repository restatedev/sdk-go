package util

import (
	"strings"

	"github.com/invopop/jsonschema"
)

// Schemas that have a top-level ref can be problematic for some parsers, like the playground in the UI.
// To be more forgiving, we can yank the definition up to the top level
func ExpandSchema(rootSchema *jsonschema.Schema) *jsonschema.Schema {
	if !strings.HasPrefix(rootSchema.Ref, `#/$defs/`) {
		return rootSchema
	}
	defName := rootSchema.Ref[len(`#/$defs/`):]
	def, ok := rootSchema.Definitions[defName]
	if !ok {
		return rootSchema
	}
	// allow references to #/$defs/name to still work by redirecting to the root
	rootSchema.Definitions[defName] = &jsonschema.Schema{Ref: "#"}

	expandedSchema := &*def
	expandedSchema.ID = rootSchema.ID
	expandedSchema.Version = rootSchema.Version
	expandedSchema.Definitions = rootSchema.Definitions

	return expandedSchema
}
