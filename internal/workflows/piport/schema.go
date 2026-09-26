//go:build jsonschema

package piport

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

func (Assessment) Schema() json.RawMessage   { panic("not implemented") }
func (Assessment) ValidateJSON([]byte) error { panic("not implemented") }

var _ = polytype.Declare(Assessment.Schema)
