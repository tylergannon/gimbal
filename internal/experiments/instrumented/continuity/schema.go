//go:build jsonschema

package continuity

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

func (Report) Schema() json.RawMessage   { panic("generate") }
func (Report) ValidateJSON([]byte) error { panic("generate") }

var _ = polytype.Declare(Report.Schema)

func (Checks) Schema() json.RawMessage   { panic("generate") }
func (Checks) ValidateJSON([]byte) error { panic("generate") }

var _ = polytype.Declare(Checks.Schema)
