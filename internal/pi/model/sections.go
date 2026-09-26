package model

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// SystemSection is one named prompt section. A nil Value is JSON null: on a
// later system message it removes the section.
type SystemSection struct {
	Name  string
	Value *string
}

// SystemSections is pi's Record<string, string | null> for
// SystemMessage.sections, preserving insertion order.
type SystemSections []SystemSection

// Get returns the value for name and whether the name is present.
func (s SystemSections) Get(name string) (*string, bool) {
	var value *string
	found := false
	for _, section := range s {
		if section.Name == name {
			value, found = section.Value, true
		}
	}
	return value, found
}

// Set assigns value to name, keeping the slot of an existing name.
func (s *SystemSections) Set(name string, value *string) {
	for i := range *s {
		if (*s)[i].Name == name {
			(*s)[i].Value = value
			s.dropRepeatsAfter(i)
			return
		}
	}
	*s = append(*s, SystemSection{Name: name, Value: value})
}

func (s *SystemSections) dropRepeatsAfter(i int) {
	name := (*s)[i].Name
	for j := i + 1; j < len(*s); j++ {
		if (*s)[j].Name == name {
			*s = append((*s)[:j], (*s)[j+1:]...)
			return
		}
	}
}

// Delete removes name.
func (s *SystemSections) Delete(name string) {
	for i := range *s {
		if (*s)[i].Name == name {
			*s = append((*s)[:i], (*s)[i+1:]...)
			return
		}
	}
}

// Len reports the number of distinct names.
func (s SystemSections) Len() int {
	if s == nil {
		return 0
	}
	seen := map[string]bool{}
	for _, section := range s {
		seen[section.Name] = true
	}
	return len(seen)
}

// Entries returns the distinct sections in insertion order, a repeated name
// keeping its first slot and its last value.
func (s SystemSections) Entries() []SystemSection {
	index := map[string]int{}
	out := make([]SystemSection, 0, len(s))
	for _, section := range s {
		if i, ok := index[section.Name]; ok {
			out[i].Value = section.Value
			continue
		}
		index[section.Name] = len(out)
		out = append(out, section)
	}
	return out
}

// MarshalJSON writes the sections as a JSON object in insertion order.
func (s SystemSections) MarshalJSON() ([]byte, error) {
	if s == nil {
		return []byte("null"), nil
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, section := range s.Entries() {
		if i > 0 {
			buf.WriteByte(',')
		}
		name, _ := json.Marshal(section.Name)
		buf.Write(name)
		buf.WriteByte(':')
		value, err := json.Marshal(section.Value)
		if err != nil {
			return nil, err
		}
		buf.Write(value)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// UnmarshalJSON reads a JSON object of string-or-null values.
func (s *SystemSections) UnmarshalJSON(data []byte) error {
	if string(trimSpace(data)) == "null" {
		*s = nil
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return fmt.Errorf("model: system message sections must be a JSON object of strings or null, got %s", trimSpace(data))
	}
	out := SystemSections{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		name, _ := tok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return err
		}
		var value *string
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("model: system message section %q must be a string or null, got %s", name, raw)
		}
		out.Set(name, value)
	}
	if _, err := dec.Token(); err != nil {
		return err
	}
	*s = out
	return nil
}

// Clone returns a deep copy of the sections.
func (s SystemSections) Clone() SystemSections {
	if s == nil {
		return nil
	}
	out := make(SystemSections, len(s))
	for i, section := range s {
		out[i] = section
		if section.Value != nil {
			v := *section.Value
			out[i].Value = &v
		}
	}
	return out
}
