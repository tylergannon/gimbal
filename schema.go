//go:build jsonschema

package gimbal

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

// Stubs so the package compiles before generation; jsonschema_gen.go
// provides the real methods.
func (review) Schema() json.RawMessage                { panic("not implemented") }
func (review) ValidateJSON(_ []byte) error            { panic("not implemented") }
func (Plan) Schema() json.RawMessage                  { panic("not implemented") }
func (Plan) ValidateJSON(_ []byte) error              { panic("not implemented") }
func (InteractiveTranscript) Schema() json.RawMessage { panic("not implemented") }
func (InteractiveTranscript) ValidateJSON(_ []byte) error {
	panic("not implemented")
}
func (interactiveOutput) Schema() json.RawMessage { panic("not implemented") }
func (interactiveOutput) ValidateJSON(_ []byte) error {
	panic("not implemented")
}
func (LifecycleRecord) Schema() json.RawMessage     { panic("not implemented") }
func (LifecycleRecord) ValidateJSON(_ []byte) error { panic("not implemented") }

var (
	_ = polytype.Declare(review.Schema)
	_ = polytype.Declare(Plan.Schema)
	_ = polytype.Declare(InteractiveTranscript.Schema)
	_ = polytype.Declare(interactiveOutput.Schema)
	_ = polytype.Declare(LifecycleRecord.Schema)
	_ = polytype.SealedUnion[interactiveDecision]("type", polytype.Snake)
	_ = polytype.SealedUnion[LifecycleEvent]("kind", polytype.Snake)
)
