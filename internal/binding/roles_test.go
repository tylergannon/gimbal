package binding

import (
	"strings"
	"testing"

	"github.com/tylergannon/gimble"
)

func TestRolesShareABindingAndNameTheMissingFlag(t *testing.T) {
	models, err := Roles(map[gimble.WorkflowRole]string{"coder": "gpt-5.6-luna", "planner": "gpt-5.6-luna", "reviewer": "claude-haiku-4-5-20251001"})
	if err != nil {
		t.Fatal(err)
	}
	if models["coder"].Adapter != models["planner"].Adapter {
		t.Error("two roles on one model got two harnesses")
	}
	if models["reviewer"].Adapter == models["coder"].Adapter {
		t.Error("a role on another model shares the first's harness")
	}
	if _, err := Roles(map[gimble.WorkflowRole]string{"coder": "gpt-5.6-luna", "planner": ""}); err == nil || !strings.Contains(err.Error(), "give --planner") {
		t.Errorf("err = %v, want the missing role named", err)
	}
}

func TestRolesBindCurrentGeminiResearchModels(t *testing.T) {
	models, err := Roles(map[gimble.WorkflowRole]string{
		"research-indexing":  "gemini-3.8-flash-medium",
		"document-authoring": "gemini-3.1-pro-high",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := models["research-indexing"]; got.Model != "gemini-3.8-flash-medium" || got.Effort != "medium" {
		t.Errorf("research-indexing = model %q effort %q", got.Model, got.Effort)
	}
	if got := models["document-authoring"]; got.Model != "gemini-3.1-pro-high" || got.Effort != "high" {
		t.Errorf("document-authoring = model %q effort %q", got.Model, got.Effort)
	}
}
