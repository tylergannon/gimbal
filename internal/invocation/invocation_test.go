package invocation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/workflow"
	"testing"

	"github.com/tylergannon/polytype"
)

func TestJSONValidation(t *testing.T) {
	for _, raw := range []string{`{}`, `{"goal":" "}`, `null`, `{"goal":17}`, `{"goal":"x","typo":true}`, `{"goal":"x"} {}`} {
		if _, _, err := Prepare(Request{Workflow: "review", Input: json.RawMessage(raw)}); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	if _, _, err := Prepare(Request{Workflow: "review", Input: json.RawMessage(`{"goal":"review"}`)}); err != nil {
		t.Fatal(err)
	}
	var input struct {
		Flag  polytype.Optional[bool] `json:"flag,omitzero"`
		Count polytype.Optional[int]  `json:"count,omitzero"`
	}
	if err := json.Unmarshal([]byte(`{}`), &input); err != nil {
		t.Fatal(err)
	}
	if input.Flag.Present || input.Count.Present {
		t.Fatal("omitted fields became present")
	}
	if err := json.Unmarshal([]byte(`{"flag":false,"count":0}`), &input); err != nil {
		t.Fatal(err)
	}
	if !input.Flag.Present || !input.Count.Present || input.Flag.Value || input.Count.Value != 0 {
		t.Fatalf("explicit zero lost: %+v", input)
	}
}

func TestConversationEnvironmentPreflightIsPure(t *testing.T) {
	var connects int
	err := gimbal.DefaultRegistry.RegisterMessaging(gimbal.MessagingDefinition{
		Name: "preflight-test", ExecutionIdentity: true,
		ValidateConfiguration: func(json.RawMessage) error { t.Fatal("preflight read bootstrap configuration"); return nil },
		ValidateEnvironment: func(raw json.RawMessage, name string) error {
			if string(raw) != `{"namespace":"test"}` || name != "bob" {
				return fmt.Errorf("unconfigured environment %q", name)
			}
			return nil
		},
		Connect: func(context.Context, json.RawMessage, backend.Dependencies) (backend.Messaging, error) {
			connects++
			return nil, nil
		},
		Worker: func(context.Context, json.RawMessage, backend.Dependencies, backend.MessageHandler) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	route := backend.Route{Name: "preflight-test", Configuration: json.RawMessage(`{"namespace":"test"}`)}
	for _, tc := range []struct {
		input string
		route backend.Route
		ok    bool
	}{
		{`{"purpose":"talk","environment":"bob"}`, route, true},
		{`{"purpose":"talk","environment":"missing"}`, route, false},
		{`{"purpose":"talk","environment":""}`, route, false},
		{`{"purpose":"talk"}`, route, false},
		{`{"purpose":"talk","environment":"bob"}`, backend.Route{}, false},
		{`{"purpose":"talk","environment":"local"}`, backend.Route{}, true},
		{`{"purpose":"talk","environment":"bob"}`, backend.Route{Name: "preflight-test", Environment: "physical", Configuration: route.Configuration}, false},
		{`{"purpose":"talk","environment":"bob"}`, backend.Route{Name: "preflight-test", Configuration: route.Configuration, Backends: &backend.BackendSelection{Orchestrator: backend.Selection{Name: "legacy"}}}, false},
	} {
		_, _, err := Prepare(Request{Workflow: "conversation", Input: json.RawMessage(tc.input), TaskRoute: tc.route})
		if (err == nil) != tc.ok {
			t.Errorf("input %s route %+v: %v", tc.input, tc.route, err)
		}
	}
	if connects != 0 {
		t.Fatalf("preflight connected %d times", connects)
	}
}

func TestIndeterminateEnvironmentRejectsBeforeWorkflow(t *testing.T) {
	ran := false
	definition := gimbal.WorkflowDefinition{
		WorkflowDescription: gimbal.WorkflowDescription{Name: "indeterminate-environment-test", InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`)},
		Graph:               workflow.Graph{Name: "indeterminate-environment-test", Environments: []workflow.EnvironmentDeclaration{{Source: workflow.Source{File: "test.go", Line: 1}}}},
		Prepare: func(json.RawMessage) (gimbal.Invocation, error) {
			return func(context.Context, gimbal.Env) error { ran = true; return nil }, nil
		},
	}
	if err := gimbal.DefaultRegistry.RegisterWorkflow(definition); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Prepare(Request{Workflow: definition.Name, Input: json.RawMessage(`{}`)}); err == nil {
		t.Fatal("accepted computed selector")
	}
	if ran {
		t.Fatal("workflow ran during preflight")
	}
}
