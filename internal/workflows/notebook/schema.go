//go:build jsonschema

package notebook

import (
	"encoding/json"
	"github.com/tylergannon/polytype"
)

func (Index) Schema() json.RawMessage    { panic("generation stub") }
func (Index) ValidateJSON([]byte) error  { panic("generation stub") }
func (Sprint) Schema() json.RawMessage   { panic("generation stub") }
func (Sprint) ValidateJSON([]byte) error { panic("generation stub") }
func (Review) Schema() json.RawMessage   { panic("generation stub") }
func (Review) ValidateJSON([]byte) error { panic("generation stub") }

var _ = polytype.Declare(Index.Schema)
var _ = polytype.Declare(Sprint.Schema)
var _ = polytype.Declare(Review.Schema)
