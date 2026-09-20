//go:build jsonschema

package indexfeedback

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

func (Questions) Schema() json.RawMessage   { panic("not implemented") }
func (Questions) ValidateJSON([]byte) error { panic("not implemented") }

var _ = polytype.Declare(Questions.Schema)
