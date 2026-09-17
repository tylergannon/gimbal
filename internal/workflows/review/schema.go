//go:build jsonschema

package review

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

// Stubs so the package compiles before generation; jsonschema_gen.go
// provides the real methods.
func (Result) Schema() json.RawMessage     { panic("not implemented") }
func (Result) ValidateJSON(_ []byte) error { panic("not implemented") }

var _ = polytype.Declare(Result.Schema)
