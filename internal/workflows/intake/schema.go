//go:build jsonschema

package intake

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

func (decision) Schema() json.RawMessage   { panic("not implemented") }
func (decision) ValidateJSON([]byte) error { panic("not implemented") }

var _ = polytype.Declare(decision.Schema)
