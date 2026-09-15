//go:build jsonschema

package planning

//go:generate go tool polytype --validate

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

func (synthesis) Schema() json.RawMessage   { panic("generated") }
func (synthesis) ValidateJSON([]byte) error { panic("generated") }

var _ = polytype.Declare(synthesis.Schema)
