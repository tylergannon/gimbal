package modelalias

import (
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		selection Selection
		harness   string
		effort    string
	}{
		{Selection{Name: "gpt"}, "codex", "high"},
		{Selection{Name: "fable"}, "claude", "high"},
		{Selection{Name: "flash"}, "agy", "medium"},
		{Selection{Name: "opus"}, "claude", "high"},
		{Selection{Name: "flash", Version: "3.7", VersionPresent: true, Effort: "low", EffortPresent: true}, "agy", "low"},
		{Selection{Name: "gemini-model-low"}, "agy", "low"},
		{Selection{Name: "opencode/ling-3.0-flash-fin-free"}, "opencode", ""},
		{Selection{Name: "opencode/gemini-model-name-low"}, "opencode", ""},
		{Selection{Name: "opencode/openrouter/vendor/future-model", Effort: "medium", EffortPresent: true}, "opencode", "medium"},
		{Selection{Name: "pi/diffusion/deepseek-4.1-flash"}, "pi", ""},
		{Selection{Name: "pi/diffusion/glm-5.3"}, "pi", ""},
		{Selection{Name: "pi/diffusion/glm-5.3-flash"}, "pi", ""},
	}
	for _, test := range tests {
		got, err := Resolve(test.selection)
		if err != nil {
			t.Fatal(err)
		}
		if got.Harness != test.harness || got.Effort != test.effort {
			t.Fatalf("Resolve(%+v) = %+v", test.selection, got)
		}
		if strings.HasPrefix(test.selection.Name, "pi/diffusion/") && (got.Provider != "diffusion" || got.Model != strings.TrimPrefix(test.selection.Name, "pi/")) {
			t.Fatalf("Pi route = %+v, want provider diffusion and native model %q", got, strings.TrimPrefix(test.selection.Name, "pi/"))
		}
	}
}

func TestResolveRejectsInvalidSelections(t *testing.T) {
	tests := []struct {
		selection Selection
		want      string
	}{
		{Selection{Name: ""}, "nonblank"},
		{Selection{Name: "flash", VersionPresent: true}, "version must be a nonblank"},
		{Selection{Name: "flash", Version: "9", VersionPresent: true}, "does not support version"},
		{Selection{Name: "mystery"}, "cannot determine provider"},
		{Selection{Name: "fable", Effort: "ultra", EffortPresent: true}, "unsupported model effort"},
		{Selection{Name: "gemini-model-low", Effort: "high", EffortPresent: true}, "fixes effort"},
		{Selection{Name: "opencode/"}, "expected opencode/<model-id>"},
		{Selection{Name: "opencode/provider/"}, "expected opencode/<model-id>"},
		{Selection{Name: "opencode/ling-3.0", Version: "1", VersionPresent: true}, "cannot also declare version"},
		{Selection{Name: "pi/diffusion/"}, "expected pi/diffusion/<model-id>"},
		{Selection{Name: "pi/diffusion//bad"}, "expected pi/diffusion/<model-id>"},
		{Selection{Name: "pi/diffusion/deepseek-4.1-flash", Version: "1", VersionPresent: true}, "cannot also declare version"},
	}
	for _, test := range tests {
		_, err := Resolve(test.selection)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("Resolve(%+v) error = %v, want %q", test.selection, err, test.want)
		}
	}
}
