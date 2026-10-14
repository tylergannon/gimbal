package workflow

import "encoding/json"

// Role identifies the model binding used by a workflow session.
type Role string

// Field describes one supported input and its visible presentation.
type Field struct {
	Name        string `json:"name"`
	Flag        string `json:"flag"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Optional    bool   `json:"optional"`
	Nullable    bool   `json:"nullable"`
}

// Description is the serializable catalog used by clients.
type Description struct {
	Name        string          `json:"name"`
	Summary     string          `json:"summary"`
	Help        string          `json:"help"`
	InputSchema json.RawMessage `json:"input_schema"`
	Fields      []Field         `json:"fields"`
	Roles       map[Role]string `json:"roles"`
	SourcePage  json.RawMessage `json:"source_page"`
}
