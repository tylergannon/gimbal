//go:build jsonschema

package servicecheck

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

func (Result) Schema() json.RawMessage   { panic("generated") }
func (Result) ValidateJSON([]byte) error { panic("generated") }

var _ = polytype.Declare(Result.Schema)
