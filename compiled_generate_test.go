package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tylergannon/gimbal/internal/compiledscope"
)

type decodeWitness struct {
	Value string `json:"value"`
}

var witnessDecodes atomic.Int32

func (decodeWitness) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (decodeWitness) ValidateJSON(raw []byte) error {
	if !json.Valid(raw) {
		return errors.New("invalid JSON")
	}
	return nil
}
func (w *decodeWitness) UnmarshalJSON(raw []byte) error {
	witnessDecodes.Add(1)
	type plain decodeWitness
	if err := json.Unmarshal(raw, (*plain)(w)); err != nil {
		return err
	}
	w.Value += ":decoded"
	return nil
}

// This witness intentionally bypasses compiler admission to detect accidental
// typed materialization inside the private runtime seam. The compiler separately
// rejects its application-defined decoder.
func TestCompiledGenerateTransportsBeforeConsumption(t *testing.T) {
	witnessDecodes.Store(0)
	attempts := 0
	f := &fake{answer: func(_ context.Context, _, prompt string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		attempts++
		if attempts == 1 {
			return `{`, nil
		}
		if !strings.Contains(prompt, "previous answer was invalid") || !strings.Contains(prompt, "## context") {
			t.Error("re-ask lost scoped prompt")
		}
		return `{"value":"original"}`, nil
	}}
	err := runTest(t, bind(f, "fake", "coder"), func(ctx context.Context) error {
		Set(ctx, "context", "retained")
		s := NewSession(ctx, "coder", ".")
		raw, err := compiledscope.Generate[decodeWitness](ctx, s, "Return a value.")
		if err != nil {
			return err
		}
		if witnessDecodes.Load() != 0 || string(raw) != `{"value":"original"}` {
			t.Fatal("activity materialized or changed the result")
		}
		got, err := compiledscope.Consume[decodeWitness](raw, nil)
		if err != nil || got.Value != "original:decoded" || witnessDecodes.Load() != 1 {
			t.Fatalf("consume=%+v %v, calls=%d", got, err, witnessDecodes.Load())
		}
		return nil
	})
	if err != nil || attempts != 2 {
		t.Fatalf("operation %v attempts=%d", err, attempts)
	}
}

type looseResult struct {
	Name  string `json:"name"`
	Count int8   `json:"count"`
}

func (looseResult) Schema() json.RawMessage   { return json.RawMessage(`{"type":"object"}`) }
func (looseResult) ValidateJSON([]byte) error { return nil }

func TestLocalGenerateRetainsDecodeReasksAndPartialFailure(t *testing.T) {
	for _, recover := range []bool{false, true} {
		t.Run(map[bool]string{false: "exhaustion", true: "recovery"}[recover], func(t *testing.T) {
			attempts := 0
			f := &fake{answer: func(_ context.Context, _, prompt string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
				attempts++
				if attempts > 1 && !strings.Contains(prompt, "previous answer was invalid") {
					t.Error("missing re-ask")
				}
				if recover && attempts == 2 {
					return `{"name":"valid","count":7}`, nil
				}
				return `{"name":"partial","count":300}`, nil
			}}
			err := runTest(t, bind(f, "fake", "coder"), func(ctx context.Context) error {
				s := NewSession(ctx, "coder", ".")
				got, err := s.Generate[looseResult](ctx, "Return a small count.")
				if recover {
					if err != nil || got.Name != "valid" || got.Count != 7 || attempts != 2 {
						t.Fatalf("result=%+v %v attempts=%d", got, err, attempts)
					}
				} else if !errors.Is(err, errInvalidResult) || got.Name != "partial" || attempts != generateAttempts {
					t.Fatalf("partial result=%+v %v attempts=%d", got, err, attempts)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
