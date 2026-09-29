//go:build jsonschema

package resulttypes

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

func (Result) Schema() json.RawMessage    { panic("generate") }
func (Result) ValidateJSON([]byte) error  { panic("generate") }
func (Numeric) Schema() json.RawMessage   { panic("generate") }
func (Numeric) ValidateJSON([]byte) error { panic("generate") }

var _ = polytype.Declare(Result.Schema)
var _ = polytype.Declare(Numeric.Schema)

func (CollisionResult) Schema() json.RawMessage   { panic("generate") }
func (CollisionResult) ValidateJSON([]byte) error { panic("generate") }

var _ = polytype.Declare(CollisionResult.Schema)
