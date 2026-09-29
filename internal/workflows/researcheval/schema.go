//go:build jsonschema

package researcheval

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

func (Quality) Schema() json.RawMessage       { panic("not implemented") }
func (Quality) ValidateJSON([]byte) error     { panic("not implemented") }
func (ReaderStep) Schema() json.RawMessage    { panic("not implemented") }
func (ReaderStep) ValidateJSON([]byte) error  { panic("not implemented") }
func (AnswerGrade) Schema() json.RawMessage   { panic("not implemented") }
func (AnswerGrade) ValidateJSON([]byte) error { panic("not implemented") }

var _ = polytype.Declare(Quality.Schema)
var _ = polytype.Declare(ReaderStep.Schema)
var _ = polytype.Declare(AnswerGrade.Schema)
