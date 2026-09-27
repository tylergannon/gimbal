package execution

import (
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/codex"
)

func TestRoleBindingsComeFromWorkflowModelBindings(t *testing.T) {
	adapter := codex.New()
	models := map[gimbal.WorkflowRole]gimbal.ModelBinding{
		"planner": {Adapter: adapter, Harness: "codex", Model: "gpt-5.6-luna", Effort: "low"},
		"coder":   {Adapter: adapter, Harness: "codex", Model: "gpt-5.6", Effort: "high"},
	}
	roles, err := roleBindings(models)
	if err != nil {
		t.Fatal(err)
	}
	for role, model := range models {
		want := RoleBinding{Harness: model.Harness, Model: model.Model, Effort: model.Effort}
		if roles[role] != want {
			t.Errorf("role binding for %q = %#v, want %#v", role, roles[role], want)
		}
	}
	models["coder"] = gimbal.ModelBinding{Adapter: adapter, Harness: "claude", Model: "sonnet"}
	if _, err := roleBindings(models); err == nil {
		t.Fatal("unsupported harness binding was accepted")
	}
}
