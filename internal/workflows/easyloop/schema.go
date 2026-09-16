//go:build jsonschema

package easyloop

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

// Stubs so the package compiles before generation; jsonschema_gen.go
// provides the real methods.
func (review) Schema() json.RawMessage     { panic("not implemented") }
func (review) ValidateJSON(_ []byte) error { panic("not implemented") }

var _ = polytype.Declare(review.Schema)
