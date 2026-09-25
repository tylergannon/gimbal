//go:build jsonschema

package gimbal

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

// Stubs so the package compiles before generation; jsonschema_gen.go
// provides the real methods.
func (review) Schema() json.RawMessage              { panic("not implemented") }
func (review) ValidateJSON(_ []byte) error          { panic("not implemented") }
func (plan) Schema() json.RawMessage                { panic("not implemented") }
func (plan) ValidateJSON(_ []byte) error            { panic("not implemented") }
func (InterviewTranscript) Schema() json.RawMessage { panic("not implemented") }
func (InterviewTranscript) ValidateJSON(_ []byte) error {
	panic("not implemented")
}
func (interviewDecision) Schema() json.RawMessage { panic("not implemented") }
func (interviewDecision) ValidateJSON(_ []byte) error {
	panic("not implemented")
}
func (LifecycleRecord) Schema() json.RawMessage     { panic("not implemented") }
func (LifecycleRecord) ValidateJSON(_ []byte) error { panic("not implemented") }

var (
	_ = polytype.Declare(review.Schema)
	_ = polytype.Declare(plan.Schema)
	_ = polytype.Declare(InterviewTranscript.Schema)
	_ = polytype.Declare(interviewDecision.Schema)
	_ = polytype.Declare(LifecycleRecord.Schema)
	_ = polytype.SealedUnion[LifecycleEvent]("kind", polytype.Snake)
)
