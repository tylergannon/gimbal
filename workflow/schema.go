//go:build jsonschema

package workflow

import (
	"github.com/tylergannon/polytype"
)

// Graph is recursive, and recursive JSON Schema is unsupported, so it is
// declared with no schema entrypoint: it gets the generated Go JSON codecs
// and the TypeScript projection, and no schema file.
var (
	_ = polytype.Declare[Graph]()
	_ = polytype.SealedUnion[Operation]("kind", polytype.Snake)
)
