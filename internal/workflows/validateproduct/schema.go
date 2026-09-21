//go:build jsonschema

package validateproduct

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

// Stubs so the package compiles before generation; jsonschema_gen.go
// provides the real methods.
func (VisualVerdict) Schema() json.RawMessage     { panic("not implemented") }
func (VisualVerdict) ValidateJSON(_ []byte) error { panic("not implemented") }

var _ = polytype.Declare(VisualVerdict.Schema)
