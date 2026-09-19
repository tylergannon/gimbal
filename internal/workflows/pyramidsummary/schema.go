//go:build jsonschema

package pyramidsummary

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

// Stubs let the package compile while polytype writes the real methods.
func (PyramidVerdict) Schema() json.RawMessage     { panic("not implemented") }
func (PyramidVerdict) ValidateJSON(_ []byte) error { panic("not implemented") }

var _ = polytype.Declare(PyramidVerdict.Schema)
