package gimbal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestValidateTurnResult(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"n":{"type":"integer","minimum":1}},"required":["n"],"additionalProperties":false}`)
	for _, tc := range []struct {
		name, raw string
		schema    json.RawMessage
		valid     bool
	}{
		{"unchanged object", " \n {\"n\": 2} \t", schema, true},
		{"schema mismatch", `{"n":0}`, schema, false},
		{"wrong shape", `{"n":"two"}`, schema, false},
		{"duplicate name", `{"n":"bad","n":2}`, schema, false},
		{"trailing value", `{"n":2} {}`, schema, false},
		{"bad syntax", `{`, schema, false},
		{"invalid UTF-8", "\"\xff\"", nil, false},
		{"text", `"hello"`, nil, true},
		{"text null", `null`, nil, false},
		{"text object", `{}`, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := TurnResult{Output: json.RawMessage(tc.raw)}
			before := bytes.Clone(result.Output)
			err := ValidateTurnResult(tc.schema, result)
			if tc.valid && err != nil {
				t.Fatal(err)
			}
			if !tc.valid && !errors.Is(err, ErrInvalidOutput) {
				t.Fatalf("error = %v", err)
			}
			if !bytes.Equal(before, result.Output) {
				t.Fatal("validation changed output")
			}
		})
	}
}

// No ValidateJSON method: the schema is the complete Output constraint.
type schemaOnlyAnswer struct {
	N int `json:"n"`
}

func (schemaOnlyAnswer) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"n":{"type":"integer"}},"required":["n"]}`)
}

type countedAnswer struct {
	schemaOnlyAnswer
	Decodes int
}

func (a *countedAnswer) UnmarshalJSON(raw []byte) error {
	a.Decodes++
	return json.Unmarshal(raw, &a.schemaOnlyAnswer)
}
func (countedAnswer) ValidateJSON([]byte) error { panic("Generate must not call custom validation") }

// A valid schema result can still fail the plugin's typed decode.
type rejectingAnswer struct{ schemaOnlyAnswer }

func (*rejectingAnswer) UnmarshalJSON([]byte) error {
	return errors.New("plugin decode rejected answer")
}

func TestGenerateDecodesOnceAndReasksInvalidNativeOutput(t *testing.T) {
	calls := 0
	f := &fake{answer: func(_ context.Context, _, prompt string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		calls++
		if calls == 1 {
			return `{"n":"bad"}`, nil
		}
		if !strings.Contains(prompt, "previous answer was invalid") {
			t.Error("missing re-ask reason")
		}
		return " \n {\"n\":2} \t", nil
	}}
	err := runTest(t, bind(f, "m", "coder"), func(ctx context.Context) error {
		result, err := NewSession(ctx, "coder", "/w").Generate[countedAnswer](ctx, "answer")
		if err == nil && (result.Decodes != 1 || result.N != 2) {
			t.Errorf("decoded answer = %+v", result)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestGenerateBoundsTypedDecodeReasks(t *testing.T) {
	calls := 0
	f := &fake{answer: func(_ context.Context, _, prompt string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		calls++
		if calls > 1 && !strings.Contains(prompt, "plugin decode rejected answer") {
			t.Error("missing decode reason")
		}
		return `{"n":2}`, nil
	}}
	err := runTest(t, bind(f, "m", "coder"), func(ctx context.Context) error {
		_, err := NewSession(ctx, "coder", "/w").Generate[rejectingAnswer](ctx, "answer")
		return err
	})
	if !errors.Is(err, ErrInvalidOutput) || calls != generateAttempts {
		t.Fatalf("calls=%d error=%v", calls, err)
	}
}
