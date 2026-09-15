//go:build jsonschema

package semanticindex

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

func (readerOutput) Schema() json.RawMessage   { panic("not implemented") }
func (readerOutput) ValidateJSON([]byte) error { panic("not implemented") }

var _ = polytype.Declare(readerOutput.Schema)
