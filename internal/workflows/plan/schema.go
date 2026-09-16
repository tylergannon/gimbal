//go:build jsonschema

package plan

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

// Stubs so the package compiles before generation; jsonschema_gen.go
// provides the real methods.
func (Input) Schema() json.RawMessage     { panic("not implemented") }
func (Input) ValidateJSON(_ []byte) error { panic("not implemented") }

var _ = polytype.Declare(Input.Schema)
