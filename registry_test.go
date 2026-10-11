package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/workflow"
)

func testDefinition(name string) WorkflowDefinition {
	return WorkflowDefinition{Name: name, InputSchema: json.RawMessage(`{"type":"object","properties":{"goal":{"type":"string"},"flag":{"type":"boolean"},"count":{"type":"integer"}},"required":["goal"],"additionalProperties":false}`), Fields: []WorkflowField{{Name: "goal", Flag: "goal", Kind: "string"}, {Name: "flag", Flag: "flag", Kind: "bool", Optional: true}, {Name: "count", Flag: "count", Kind: "int", Optional: true}}, Graph: workflow.Graph{Name: name}, Prepare: func(json.RawMessage) (Invocation, error) { return func(context.Context, Env) error { return nil }, nil }}
}
func TestRegistryRejectsDuplicateBeforeInstallingGraph(t *testing.T) {
	r := &Registry{}
	d := testDefinition("test")
	if err := r.RegisterWorkflow(d); err != nil {
		t.Fatal(err)
	}
	r.source = "second.so"
	d.Summary = "replacement"
	err := r.RegisterWorkflow(d)
	if err == nil || !strings.Contains(err.Error(), "second.so") || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate: %v", err)
	}
	original, _ := r.Workflow("test")
	if original.Summary != "" {
		t.Fatal("duplicate replaced definition")
	}
}
func TestWorkflowInputPreservesOmissionAndExplicitZeros(t *testing.T) {
	d := testDefinition("test").WorkflowDescription
	for _, raw := range []string{`{"goal":"x"}`, `{"goal":"x","flag":false,"count":0}`} {
		if err := ValidateWorkflowInput(d, []byte(raw)); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
	}
	for _, tt := range []struct{ raw, field string }{{`{}`, "goal"}, {`{"goal":" "}`, "goal"}, {`{"goal":"x","flag":null}`, "flag"}, {`{"goal":"x","count":1.2}`, "count"}, {`{"goal":"x","extra":true}`, "extra"}} {
		err := ValidateWorkflowInput(d, []byte(tt.raw))
		var input *InputError
		if !errors.As(err, &input) || input.Field != tt.field {
			t.Errorf("%s: %v", tt.raw, err)
		}
	}
}
func TestRegistryRejectsMissingRoleDefault(t *testing.T) {
	r := &Registry{}
	d := testDefinition("role")
	d.Graph.Body = []workflow.Operation{workflow.Session{Name: "reviewer"}}
	if err := r.RegisterWorkflow(d); err == nil || !strings.Contains(err.Error(), "reviewer") {
		t.Fatalf("missing default: %v", err)
	}
	if len(r.Catalog()) != 0 {
		t.Fatal("malformed definition became visible")
	}
}
