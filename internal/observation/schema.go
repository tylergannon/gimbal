//go:build jsonschema

package observation

import "github.com/tylergannon/polytype"

// These are the observation shapes the hand-owned browser projection shares
// with generated server bindings.
var (
	_ = polytype.Declare[InterviewRow]()
	_ = polytype.Declare[TurnRow]()
)
