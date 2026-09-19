//go:build jsonschema

package validateproduct

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

func (Observation) Schema() json.RawMessage   { panic("not implemented") }
func (Observation) ValidateJSON([]byte) error { panic("not implemented") }
func (Verdict) Schema() json.RawMessage       { panic("not implemented") }
func (Verdict) ValidateJSON([]byte) error     { panic("not implemented") }

var _ = polytype.Declare(Observation.Schema)
var _ = polytype.Declare(Verdict.Schema)

func (Feature) Schema() json.RawMessage   { panic("not implemented") }
func (Feature) ValidateJSON([]byte) error { panic("not implemented") }

var _ = polytype.Declare(Feature.Schema)
