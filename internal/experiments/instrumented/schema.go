//go:build jsonschema

package main

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

func (Checks) Schema() json.RawMessage   { panic("generate") }
func (Checks) ValidateJSON([]byte) error { panic("generate") }
func (Report) Schema() json.RawMessage   { panic("generate") }
func (Report) ValidateJSON([]byte) error { panic("generate") }

var _ = polytype.Declare(Checks.Schema)
var _ = polytype.Declare(Report.Schema)

func (Assignment) Schema() json.RawMessage   { panic("generate") }
func (Assignment) ValidateJSON([]byte) error { panic("generate") }

var _ = polytype.Declare(Assignment.Schema)
