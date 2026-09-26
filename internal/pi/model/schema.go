package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// Schema is a literal JSON Schema node used for tool parameters. It is the Go
// analogue of pi's TypeBox schemas: it serializes to standard JSON Schema for
// providers and carries the keywords the port needs. Property order is
// preserved so serialization is deterministic.
type Schema struct {
	Type          string
	Description   string
	Properties    map[string]*Schema
	PropertyOrder []string
	Required      []string
	Items         *Schema
	Enum          []any
	Default       any
	// AdditionalAllowed maps to additionalProperties: bool when AdditionalSchema
	// is nil.
	AdditionalAllowed *bool
	AdditionalSchema  *Schema
	Minimum           *float64
	Maximum           *float64
	ExclusiveMinimum  *float64
	ExclusiveMaximum  *float64
	MultipleOf        *float64
	MinLength         *int
	MaxLength         *int
	Pattern           string
	MinItems          *int
	MaxItems          *int
	// Const is the JSON Schema "const" keyword; HasConst distinguishes
	// const:null from no const.
	Const    any
	HasConst bool
	Format   string
	// Nullable adds "null" to the JSON type.
	Nullable bool
	AnyOf    []*Schema
	OneOf    []*Schema
	AllOf    []*Schema
	// Extra holds passthrough keywords not modeled above.
	Extra map[string]any
}

// Field is a named object property.
type Field struct {
	Name     string
	Schema   *Schema
	Optional bool
}

// Prop declares a required object property.
func Prop(name string, s *Schema) Field { return Field{Name: name, Schema: s} }

// Opt declares an optional object property.
func Opt(name string, s *Schema) Field { return Field{Name: name, Schema: s, Optional: true} }

// Object builds an object schema from ordered fields.
func Object(fields ...Field) *Schema {
	s := &Schema{Type: "object", Properties: map[string]*Schema{}}
	for _, f := range fields {
		s.Properties[f.Name] = f.Schema
		s.PropertyOrder = append(s.PropertyOrder, f.Name)
		if !f.Optional {
			s.Required = append(s.Required, f.Name)
		}
	}
	return s
}

// String builds a string schema.
func String() *Schema { return &Schema{Type: "string"} }

// Number builds a number schema.
func Number() *Schema { return &Schema{Type: "number"} }

// Integer builds an integer schema.
func Integer() *Schema { return &Schema{Type: "integer"} }

// Boolean builds a boolean schema.
func Boolean() *Schema { return &Schema{Type: "boolean"} }

// Array builds an array schema.
func Array(items *Schema) *Schema { return &Schema{Type: "array", Items: items} }

// OrderedProperties returns the declared property names in order, followed by
// any map keys that have no declared order.
func (s *Schema) OrderedProperties() []string {
	seen := make(map[string]bool, len(s.Properties))
	out := make([]string, 0, len(s.Properties))
	for _, name := range s.PropertyOrder {
		if _, ok := s.Properties[name]; ok && !seen[name] {
			out = append(out, name)
			seen[name] = true
		}
	}
	if len(seen) == len(s.Properties) {
		return out
	}
	var rest []string
	for name := range s.Properties {
		if !seen[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// Clone returns a deep copy of the schema.
func (s *Schema) Clone() *Schema {
	if s == nil {
		return nil
	}
	out := *s
	out.PropertyOrder = append([]string(nil), s.PropertyOrder...)
	out.Required = append([]string(nil), s.Required...)
	if s.Enum != nil {
		out.Enum = cloneValue(s.Enum).([]any)
	}
	out.Default = cloneValue(s.Default)
	out.Const = cloneValue(s.Const)
	if s.Properties != nil {
		out.Properties = make(map[string]*Schema, len(s.Properties))
		for k, v := range s.Properties {
			out.Properties[k] = v.Clone()
		}
	}
	out.Items = s.Items.Clone()
	out.AdditionalSchema = s.AdditionalSchema.Clone()
	out.AnyOf = cloneSchemas(s.AnyOf)
	out.OneOf = cloneSchemas(s.OneOf)
	out.AllOf = cloneSchemas(s.AllOf)
	if s.AdditionalAllowed != nil {
		v := *s.AdditionalAllowed
		out.AdditionalAllowed = &v
	}
	out.Extra = cloneMap(s.Extra)
	out.Minimum = cloneFloat(s.Minimum)
	out.Maximum = cloneFloat(s.Maximum)
	out.ExclusiveMinimum = cloneFloat(s.ExclusiveMinimum)
	out.ExclusiveMaximum = cloneFloat(s.ExclusiveMaximum)
	out.MultipleOf = cloneFloat(s.MultipleOf)
	out.MinLength = cloneInt(s.MinLength)
	out.MaxLength = cloneInt(s.MaxLength)
	out.MinItems = cloneInt(s.MinItems)
	out.MaxItems = cloneInt(s.MaxItems)
	return &out
}

func cloneSchemas(in []*Schema) []*Schema {
	if in == nil {
		return nil
	}
	out := make([]*Schema, len(in))
	for i, s := range in {
		out[i] = s.Clone()
	}
	return out
}

func cloneFloat(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func cloneInt(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// MarshalJSON writes the schema keywords with properties in declaration order.
func (s *Schema) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	first := true
	write := func(key string, raw []byte) {
		if !first {
			buf.WriteByte(',')
		}
		first = false
		k, _ := json.Marshal(key)
		buf.Write(k)
		buf.WriteByte(':')
		buf.Write(raw)
	}
	if s.Description != "" {
		raw, _ := json.Marshal(s.Description)
		write("description", raw)
	}
	if s.Type != "" {
		if s.Nullable {
			raw, _ := json.Marshal([]string{s.Type, "null"})
			write("type", raw)
		} else {
			raw, _ := json.Marshal(s.Type)
			write("type", raw)
		}
	}
	if len(s.Required) > 0 {
		raw, _ := json.Marshal(s.Required)
		write("required", raw)
	}
	if s.Properties != nil {
		var props bytes.Buffer
		props.WriteByte('{')
		for i, name := range s.OrderedProperties() {
			if i > 0 {
				props.WriteByte(',')
			}
			k, _ := json.Marshal(name)
			props.Write(k)
			props.WriteByte(':')
			raw, err := json.Marshal(s.Properties[name])
			if err != nil {
				return nil, err
			}
			props.Write(raw)
		}
		props.WriteByte('}')
		write("properties", props.Bytes())
	}
	if s.Items != nil {
		raw, err := json.Marshal(s.Items)
		if err != nil {
			return nil, err
		}
		write("items", raw)
	}
	if len(s.Enum) > 0 {
		raw, _ := json.Marshal(s.Enum)
		write("enum", raw)
	}
	if s.Default != nil {
		raw, _ := json.Marshal(s.Default)
		write("default", raw)
	}
	if s.HasConst {
		raw, _ := json.Marshal(s.Const)
		write("const", raw)
	}
	if s.AdditionalSchema != nil {
		raw, err := json.Marshal(s.AdditionalSchema)
		if err != nil {
			return nil, err
		}
		write("additionalProperties", raw)
	} else if s.AdditionalAllowed != nil {
		raw, _ := json.Marshal(*s.AdditionalAllowed)
		write("additionalProperties", raw)
	}
	writeFloat := func(key string, p *float64, skipZero bool) {
		if p == nil || (skipZero && *p == 0) {
			return
		}
		raw, _ := json.Marshal(*p)
		write(key, raw)
	}
	writeFloat("minimum", s.Minimum, false)
	writeFloat("maximum", s.Maximum, false)
	writeFloat("exclusiveMinimum", s.ExclusiveMinimum, false)
	writeFloat("exclusiveMaximum", s.ExclusiveMaximum, false)
	writeFloat("multipleOf", s.MultipleOf, false)
	writeInt := func(key string, p *int) {
		if p == nil {
			return
		}
		raw, _ := json.Marshal(*p)
		write(key, raw)
	}
	writeInt("minLength", s.MinLength)
	writeInt("maxLength", s.MaxLength)
	if s.Pattern != "" {
		raw, _ := json.Marshal(s.Pattern)
		write("pattern", raw)
	}
	writeInt("minItems", s.MinItems)
	writeInt("maxItems", s.MaxItems)
	if s.Format != "" {
		raw, _ := json.Marshal(s.Format)
		write("format", raw)
	}
	for _, group := range []struct {
		key  string
		list []*Schema
	}{{"anyOf", s.AnyOf}, {"oneOf", s.OneOf}, {"allOf", s.AllOf}} {
		if len(group.list) == 0 {
			continue
		}
		raw, err := json.Marshal(group.list)
		if err != nil {
			return nil, err
		}
		write(group.key, raw)
	}
	for _, key := range sortedKeys(s.Extra) {
		raw, err := json.Marshal(s.Extra[key])
		if err != nil {
			return nil, err
		}
		write(key, raw)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// UnmarshalJSON reads a literal JSON Schema, keeping unknown keywords in Extra.
func (s *Schema) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	out := Schema{}
	for key, value := range raw {
		switch key {
		case "type":
			var single string
			if err := json.Unmarshal(value, &single); err == nil {
				out.Type = single
				break
			}
			var many []string
			if err := json.Unmarshal(value, &many); err != nil {
				return fmt.Errorf("model: schema type: %w", err)
			}
			for _, t := range many {
				if t == "null" {
					out.Nullable = true
					continue
				}
				out.Type = t
			}
		case "description":
			_ = json.Unmarshal(value, &out.Description)
		case "properties":
			if err := json.Unmarshal(value, &out.Properties); err != nil {
				return err
			}
			out.PropertyOrder = orderedObjectKeys(value)
		case "required":
			_ = json.Unmarshal(value, &out.Required)
		case "items":
			var items Schema
			if err := json.Unmarshal(value, &items); err == nil {
				out.Items = &items
			} else {
				// Tuple form: keep the first schema as items.
				var many []Schema
				if err := json.Unmarshal(value, &many); err == nil && len(many) > 0 {
					out.Items = &many[0]
				}
			}
		case "enum":
			_ = json.Unmarshal(value, &out.Enum)
		case "default":
			_ = json.Unmarshal(value, &out.Default)
		case "const":
			out.HasConst = true
			_ = json.Unmarshal(value, &out.Const)
		case "additionalProperties":
			var allowed bool
			if err := json.Unmarshal(value, &allowed); err == nil {
				out.AdditionalAllowed = &allowed
				break
			}
			var schema Schema
			if err := json.Unmarshal(value, &schema); err == nil {
				out.AdditionalSchema = &schema
			}
		case "minimum":
			out.Minimum = decodeFloat(value)
		case "maximum":
			out.Maximum = decodeFloat(value)
		case "exclusiveMinimum":
			out.ExclusiveMinimum = decodeFloat(value)
		case "exclusiveMaximum":
			out.ExclusiveMaximum = decodeFloat(value)
		case "multipleOf":
			out.MultipleOf = decodeFloat(value)
		case "minLength":
			out.MinLength = decodeInt(value)
		case "maxLength":
			out.MaxLength = decodeInt(value)
		case "pattern":
			_ = json.Unmarshal(value, &out.Pattern)
		case "minItems":
			out.MinItems = decodeInt(value)
		case "maxItems":
			out.MaxItems = decodeInt(value)
		case "format":
			_ = json.Unmarshal(value, &out.Format)
		case "anyOf":
			_ = json.Unmarshal(value, &out.AnyOf)
		case "oneOf":
			_ = json.Unmarshal(value, &out.OneOf)
		case "allOf":
			_ = json.Unmarshal(value, &out.AllOf)
		default:
			if out.Extra == nil {
				out.Extra = map[string]any{}
			}
			var v any
			if err := json.Unmarshal(value, &v); err != nil {
				return err
			}
			out.Extra[key] = v
		}
	}
	*s = out
	return nil
}

func decodeFloat(raw json.RawMessage) *float64 {
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil
	}
	return &f
}

func decodeInt(raw json.RawMessage) *int {
	var n int
	if err := json.Unmarshal(raw, &n); err != nil {
		return nil
	}
	return &n
}

// orderedObjectKeys returns the keys of a JSON object in document order.
func orderedObjectKeys(data []byte) []string {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil
	}
	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return keys
		}
		key, _ := tok.(string)
		keys = append(keys, key)
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return keys
		}
	}
	return keys
}

func sortedKeys(m map[string]any) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Check reports whether value satisfies the schema's type and simple
// constraints. It is a shallow structural check; the agent's validation path
// derives its messages from the same keywords.
func (s *Schema) Check(value any) bool {
	if s == nil {
		return true
	}
	if s.HasConst {
		return jsonEqual(s.Const, value)
	}
	if len(s.Enum) > 0 {
		for _, e := range s.Enum {
			if jsonEqual(e, value) {
				return true
			}
		}
		return false
	}
	if len(s.AnyOf) > 0 || len(s.OneOf) > 0 || len(s.AllOf) > 0 {
		return s.checkCombinators(value)
	}
	switch s.Type {
	case "":
		return true
	case "null":
		return value == nil
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "number", "integer":
		f, ok := numberValue(value)
		if !ok {
			return false
		}
		if s.Type == "integer" && math.Trunc(f) != f {
			return false
		}
		return true
	case "array":
		arr, ok := value.([]any)
		if !ok {
			return false
		}
		if s.Items != nil {
			for _, item := range arr {
				if !s.Items.Check(item) {
					return false
				}
			}
		}
		return true
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			return false
		}
		for _, name := range s.Required {
			if _, present := obj[name]; !present {
				return false
			}
		}
		for name, prop := range s.Properties {
			if v, present := obj[name]; present {
				if !prop.Check(v) {
					return false
				}
			}
		}
		return true
	default:
		return true
	}
}

func (s *Schema) checkCombinators(value any) bool {
	for _, sub := range s.AllOf {
		if !sub.Check(value) {
			return false
		}
	}
	if len(s.AnyOf) > 0 {
		matched := false
		for _, sub := range s.AnyOf {
			if sub.Check(value) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(s.OneOf) > 0 {
		count := 0
		for _, sub := range s.OneOf {
			if sub.Check(value) {
				count++
			}
		}
		if count != 1 {
			return false
		}
	}
	return true
}

func numberValue(value any) (float64, bool) {
	switch t := value.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func jsonEqual(a, b any) bool {
	ra, errA := json.Marshal(a)
	rb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return bytes.Equal(ra, rb)
}
