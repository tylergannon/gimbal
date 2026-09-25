package binding

import (
	"strings"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/pi"
)

func TestRolesShareABindingAndNameTheMissingFlag(t *testing.T) {
	models, err := Roles(map[gimbal.WorkflowRole]string{"coder": "gpt-5.6-luna", "planner": "gpt-5.6-luna", "reviewer": "claude-haiku-4-5-20251001"})
	if err != nil {
		t.Fatal(err)
	}
	if models["coder"].Adapter != models["planner"].Adapter {
		t.Error("two roles on one model got two harnesses")
	}
	if models["reviewer"].Adapter == models["coder"].Adapter {
		t.Error("a role on another model shares the first's harness")
	}
	if _, err := Roles(map[gimbal.WorkflowRole]string{"coder": "gpt-5.6-luna", "planner": ""}); err == nil || !strings.Contains(err.Error(), "give --planner") {
		t.Errorf("err = %v, want the missing role named", err)
	}
}

func TestRolesBindCurrentGeminiResearchModels(t *testing.T) {
	models, err := Roles(map[gimbal.WorkflowRole]string{
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

func TestParseBindsOpenCodeModelForms(t *testing.T) {
	tests := []struct {
		spec  string
		model string
	}{
		{spec: "opencode/ling-3.0-flash-fin-free", model: "ling-3.0-flash-fin-free"},
		{spec: "opencode/opencode/ling-3.0-flash-fin-free", model: "opencode/ling-3.0-flash-fin-free"},
		{spec: "opencode/future-provider/model-outside-gimbal-families:low", model: "future-provider/model-outside-gimbal-families"},
	}
	for _, test := range tests {
		binding, err := Parse(test.spec)
		if err != nil {
			t.Fatalf("Parse(%q): %v", test.spec, err)
		}
		if binding.Model != test.model {
			t.Errorf("Parse(%q).Model = %q, want %q", test.spec, binding.Model, test.model)
		}
		if binding.Adapter == nil {
			t.Errorf("Parse(%q).Adapter is nil", test.spec)
		}
	}
}

func TestRolesShareOpenCodeAdapterAcrossModels(t *testing.T) {
	models, err := Roles(map[gimbal.WorkflowRole]string{
		"first":  "opencode/ling-3.0-flash-fin-free",
		"second": "opencode/openrouter/model-outside-gimbal-families",
	})
	if err != nil {
		t.Fatal(err)
	}
	if models["first"].Adapter != models["second"].Adapter {
		t.Error("OpenCode roles on different models got separate adapters")
	}
	if models["first"].Model == models["second"].Model {
		t.Error("distinct OpenCode model selections collapsed to one model")
	}
}

func TestPiModelBindsForPromptAndRolesWithoutDefaultEffort(t *testing.T) {
	binding, err := Parse("pi/diffusion/glm-5.3-flash")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := binding.Adapter.(*pi.Adapter); !ok {
		t.Fatalf("adapter = %T, want *pi.Adapter", binding.Adapter)
	}
	if binding.Model != "diffusion/glm-5.3-flash" || binding.Effort != "" {
		t.Fatalf("binding = model %q, effort %q", binding.Model, binding.Effort)
	}
	roles, err := Roles(map[gimbal.WorkflowRole]string{"coding": "pi/diffusion/glm-5.3-flash"})
	if err != nil {
		t.Fatal(err)
	}
	if got := roles["coding"]; got.Model != binding.Model || got.Effort != "" {
		t.Fatalf("role binding = model %q, effort %q", got.Model, got.Effort)
	}
	if _, err := Parse("pi/diffusion/deepseek-4.1-flash:high"); err != nil {
		t.Fatalf("explicit effort reaches Pi adapter for its clear unsupported-effort error: %v", err)
	}
}
