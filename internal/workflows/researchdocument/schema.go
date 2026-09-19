//go:build jsonschema

package researchdocument

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

// Stubs let the package compile while polytype writes the real methods.
func (ResearchPlan) Schema() json.RawMessage         { panic("not implemented") }
func (ResearchPlan) ValidateJSON(_ []byte) error     { panic("not implemented") }
func (TopicGroup) Schema() json.RawMessage           { panic("not implemented") }
func (TopicGroup) ValidateJSON(_ []byte) error       { panic("not implemented") }
func (ResearchResult) Schema() json.RawMessage       { panic("not implemented") }
func (ResearchResult) ValidateJSON(_ []byte) error   { panic("not implemented") }
func (EditorialVerdict) Schema() json.RawMessage     { panic("not implemented") }
func (EditorialVerdict) ValidateJSON(_ []byte) error { panic("not implemented") }

var _ = polytype.Declare(ResearchPlan.Schema)
var _ = polytype.Declare(TopicGroup.Schema)
var _ = polytype.Declare(ResearchResult.Schema)
var _ = polytype.Declare(EditorialVerdict.Schema)
