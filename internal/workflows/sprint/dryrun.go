package sprint

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/tylergannon/gimble"
)

// dryRun is the harness of a dry run. It calls no model: each turn's prompt
// and the schema it would send are written to w in order, and the turn is
// answered with an example value derived from the schema, marked as such, so
// the prompts that are built from earlier answers can be seen too. The first
// answer to a schema fills every array with one element and every nullable
// with a value; later answers leave them empty and null, so a loop planned
// through it runs one lap and ends. Turns are told apart by the first line
// of their prompt, which is the workflow's fixed text, and by their schema.
// It is a viewer, never proof.
type dryRun struct {
	w io.Writer

	mu       sync.Mutex
	sessions int
	models   map[string]string // model by session id
	turns    int
	answered map[string]int // answers given per prompt kind and schema
}

func (d *dryRun) CreateSession(_ context.Context, model, _ string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sessions++
	id := "session " + strconv.Itoa(d.sessions)
	d.models[id] = model
	return id, nil
}

func (d *dryRun) RunTurn(_ context.Context, sessionID, prompt string, schema json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.turns++
	var answer json.RawMessage
	shown := "none"
	if len(schema) == 0 {
		answer = json.RawMessage(`"<example answer>"`)
	} else {
		var parsed any
		if err := json.Unmarshal(schema, &parsed); err != nil {
			return gimble.TurnResult{}, fmt.Errorf("dry run: schema: %w", err)
		}
		key, _, _ := strings.Cut(prompt, "\n")
		key += string(schema)
		d.answered[key]++
		value := example(parsed, "answer", d.answered[key] == 1)
		var b bytes.Buffer
		enc := json.NewEncoder(&b)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(value); err != nil {
			return gimble.TurnResult{}, err
		}
		answer = bytes.TrimSpace(b.Bytes())
		shown = string(schema)
	}
	_, _ = fmt.Fprintf(d.w, "=== turn %d: %s (%s) ===\n\n--- prompt ---\n%s\n\n--- schema ---\n%s\n\n--- example answer (no model was called) ---\n%s\n\n",
		d.turns, sessionID, d.models[sessionID], prompt, shown, answer)
	return gimble.TurnResult{Output: answer}, nil
}

func (d *dryRun) Steer(context.Context, string, string) (bool, error) { return false, nil }

func (d *dryRun) Fork(_ context.Context, sessionID string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sessions++
	id := "session " + strconv.Itoa(d.sessions)
	d.models[id] = d.models[sessionID]
	return id, nil
}

func (*dryRun) Close(context.Context, string) error { return nil }

// example derives one value from a JSON Schema: strings name their field
// as "<example field>", numbers are 0, booleans false, objects carry every
// property, and arrays and nullables are filled when first is true and
// empty or null otherwise.
func example(schema any, name string, first bool) any {
	s, _ := schema.(map[string]any)
	types := []string{}
	switch t := s["type"].(type) {
	case string:
		types = []string{t}
	case []any:
		for _, v := range t {
			types = append(types, v.(string))
		}
	}
	if len(types) == 0 {
		return "<example " + name + ">"
	}
	kind := types[0]
	for _, t := range types {
		if t == "null" && !first {
			return nil
		}
		if t != "null" {
			kind = t
		}
	}
	switch kind {
	case "object":
		object := map[string]any{}
		properties, _ := s["properties"].(map[string]any)
		for field, property := range properties {
			object[field] = example(property, field, first)
		}
		return object
	case "array":
		if !first {
			return []any{}
		}
		return []any{example(s["items"], name, first)}
	case "integer", "number":
		return 0
	case "boolean":
		return false
	case "null":
		return nil
	default:
		return "<example " + name + ">"
	}
}
