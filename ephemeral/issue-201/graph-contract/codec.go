//go:build !jsonschema

package workflow

// Handwritten JSON codec for the recursive parts of the model.
//
// encoding/json already handles the recursive Supervisor struct. What it
// cannot do is decode the sealed Operation interface, so every owner of a
// []Operation field (Graph through its Sequence, Sequence, Branch, Loop,
// Scope, GroupChild) gets an owner codec here, mirroring the owner codecs
// polytype generates for LifecycleRecord in the root package. The wire shape
// is the one SealedUnion[Operation]("kind", polytype.Snake) would produce, so
// deleting this file and generating from declare.go changes nothing on the
// wire once polytype accepts recursive types.
//
// Each shadow struct below repeats its owner's fields. That duplication is
// the cost of the handwritten path and the reason it is temporary.

import (
	"bytes"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
)

// marshal matches polytype's generated encoder: v1 field semantics with nil
// slices written as [] so TypeScript consumers never see null for a list.
func marshal(value any) ([]byte, error) {
	return jsonv2.Marshal(value, json.DefaultOptionsV1(), jsonv2.FormatNilSliceAsNull(false))
}

// unmarshal rejects unknown properties, so a property belonging to another
// Operation variant fails at the decoding boundary.
func unmarshal(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.More() {
		return fmt.Errorf("workflow: trailing data after JSON value")
	}
	return nil
}

func kindOf(op Operation) (string, error) {
	switch op.(type) {
	case AgentCall:
		return "agent_call", nil
	case Command:
		return "command", nil
	case Set:
		return "set", nil
	case SetJSON:
		return "set_json", nil
	case Exit:
		return "exit", nil
	case Sequence:
		return "sequence", nil
	case Condition:
		return "condition", nil
	case Loop:
		return "loop", nil
	case Scope:
		return "scope", nil
	case Group:
		return "group", nil
	case nil:
		return "", fmt.Errorf("workflow: nil Operation")
	default:
		return "", fmt.Errorf("workflow: %T is not an Operation variant; pass variants by value", op)
	}
}

func newOperation(kind string) (any, error) {
	switch kind {
	case "agent_call":
		return &AgentCall{}, nil
	case "command":
		return &Command{}, nil
	case "set":
		return &Set{}, nil
	case "set_json":
		return &SetJSON{}, nil
	case "exit":
		return &Exit{}, nil
	case "sequence":
		return &Sequence{}, nil
	case "condition":
		return &Condition{}, nil
	case "loop":
		return &Loop{}, nil
	case "scope":
		return &Scope{}, nil
	case "group":
		return &Group{}, nil
	case "":
		return nil, fmt.Errorf(`workflow: operation object has no "kind"`)
	default:
		return nil, fmt.Errorf("workflow: unknown operation kind %q", kind)
	}
}

// marshalBody encodes each operation as its own object with "kind" first.
func marshalBody(body []Operation) ([]json.RawMessage, error) {
	out := make([]json.RawMessage, 0, len(body))
	for i, op := range body {
		kind, err := kindOf(op)
		if err != nil {
			return nil, fmt.Errorf("body[%d]: %w", i, err)
		}
		fields, err := marshal(op)
		if err != nil {
			return nil, fmt.Errorf("body[%d] (%s): %w", i, kind, err)
		}
		head := fmt.Appendf(nil, `{"kind":%q`, kind)
		if bytes.Equal(fields, []byte("{}")) {
			out = append(out, append(head, '}'))
			continue
		}
		out = append(out, append(append(head, ','), fields[1:]...))
	}
	return out, nil
}

// unmarshalBody dispatches on "kind", strips it, and decodes the remaining
// properties strictly into the variant.
func unmarshalBody(raw []json.RawMessage) ([]Operation, error) {
	if raw == nil {
		return nil, nil
	}
	body := make([]Operation, 0, len(raw))
	for i, object := range raw {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(object, &fields); err != nil {
			return nil, fmt.Errorf("body[%d]: %w", i, err)
		}
		var kind string
		if k, ok := fields["kind"]; ok {
			if err := json.Unmarshal(k, &kind); err != nil {
				return nil, fmt.Errorf("body[%d]: kind: %w", i, err)
			}
			delete(fields, "kind")
		}
		target, err := newOperation(kind)
		if err != nil {
			return nil, fmt.Errorf("body[%d]: %w", i, err)
		}
		rest, err := json.Marshal(fields)
		if err != nil {
			return nil, fmt.Errorf("body[%d]: %w", i, err)
		}
		if err := unmarshal(rest, target); err != nil {
			return nil, fmt.Errorf("body[%d] (%s): %w", i, kind, err)
		}
		body = append(body, deref(target))
	}
	return body, nil
}

// deref returns the variant by value, which is how variants implement
// Operation and how kindOf recognizes them.
func deref(target any) Operation {
	switch v := target.(type) {
	case *AgentCall:
		return *v
	case *Command:
		return *v
	case *Set:
		return *v
	case *SetJSON:
		return *v
	case *Exit:
		return *v
	case *Sequence:
		return *v
	case *Condition:
		return *v
	case *Loop:
		return *v
	case *Scope:
		return *v
	case *Group:
		return *v
	}
	panic(fmt.Sprintf("workflow: newOperation returned %T", target))
}

// Owner codecs. Every struct with a []Operation field encodes through a
// shadow struct whose body is pre-encoded raw JSON.

type sequenceJSON struct {
	Site
	Body []json.RawMessage `json:"body"`
}

func (s Sequence) MarshalJSON() ([]byte, error) {
	body, err := marshalBody(s.Body)
	if err != nil {
		return nil, err
	}
	return marshal(sequenceJSON{s.Site, body})
}

func (s *Sequence) UnmarshalJSON(data []byte) error {
	var shadow sequenceJSON
	if err := unmarshal(data, &shadow); err != nil {
		return err
	}
	body, err := unmarshalBody(shadow.Body)
	if err != nil {
		return err
	}
	*s = Sequence{Site: shadow.Site, Body: body}
	return nil
}

type branchJSON struct {
	Site
	Case Expression        `json:"case"`
	Body []json.RawMessage `json:"body"`
}

func (b Branch) MarshalJSON() ([]byte, error) {
	body, err := marshalBody(b.Body)
	if err != nil {
		return nil, err
	}
	return marshal(branchJSON{b.Site, b.Case, body})
}

func (b *Branch) UnmarshalJSON(data []byte) error {
	var shadow branchJSON
	if err := unmarshal(data, &shadow); err != nil {
		return err
	}
	body, err := unmarshalBody(shadow.Body)
	if err != nil {
		return err
	}
	*b = Branch{Site: shadow.Site, Case: shadow.Case, Body: body}
	return nil
}

type loopJSON struct {
	Site
	Name        string            `json:"name"`
	Planner     string            `json:"planner"`
	Goal        Expression        `json:"goal"`
	Condition   Expression        `json:"condition"`
	Body        []json.RawMessage `json:"body"`
	Supervision []Supervision     `json:"supervision"`
}

func (l Loop) MarshalJSON() ([]byte, error) {
	body, err := marshalBody(l.Body)
	if err != nil {
		return nil, err
	}
	return marshal(loopJSON{l.Site, l.Name, l.Planner, l.Goal, l.Condition, body, l.Supervision})
}

func (l *Loop) UnmarshalJSON(data []byte) error {
	var shadow loopJSON
	if err := unmarshal(data, &shadow); err != nil {
		return err
	}
	body, err := unmarshalBody(shadow.Body)
	if err != nil {
		return err
	}
	*l = Loop{Site: shadow.Site, Name: shadow.Name, Planner: shadow.Planner, Goal: shadow.Goal,
		Condition: shadow.Condition, Body: body, Supervision: shadow.Supervision}
	return nil
}

type scopeJSON struct {
	Site
	Name        string            `json:"name"`
	Body        []json.RawMessage `json:"body"`
	Supervision []Supervision     `json:"supervision"`
}

func (s Scope) MarshalJSON() ([]byte, error) {
	body, err := marshalBody(s.Body)
	if err != nil {
		return nil, err
	}
	return marshal(scopeJSON{s.Site, s.Name, body, s.Supervision})
}

func (s *Scope) UnmarshalJSON(data []byte) error {
	var shadow scopeJSON
	if err := unmarshal(data, &shadow); err != nil {
		return err
	}
	body, err := unmarshalBody(shadow.Body)
	if err != nil {
		return err
	}
	*s = Scope{Site: shadow.Site, Name: shadow.Name, Body: body, Supervision: shadow.Supervision}
	return nil
}

func (c GroupChild) MarshalJSON() ([]byte, error) {
	body, err := marshalBody(c.Body)
	if err != nil {
		return nil, err
	}
	return marshal(scopeJSON{c.Site, c.Name, body, c.Supervision})
}

func (c *GroupChild) UnmarshalJSON(data []byte) error {
	var shadow scopeJSON
	if err := unmarshal(data, &shadow); err != nil {
		return err
	}
	body, err := unmarshalBody(shadow.Body)
	if err != nil {
		return err
	}
	*c = GroupChild{Site: shadow.Site, Name: shadow.Name, Body: body, Supervision: shadow.Supervision}
	return nil
}

// Graph, Condition, and Group have no []Operation field of their own; their
// nested owners carry the codec. They still marshal through marshal so nil
// slices become [] at the root.

func (g Graph) MarshalJSON() ([]byte, error) {
	type plain Graph
	return marshal(plain(g))
}

func (g *Graph) UnmarshalJSON(data []byte) error {
	type plain Graph
	var shadow plain
	if err := unmarshal(data, &shadow); err != nil {
		return err
	}
	*g = Graph(shadow)
	return nil
}
