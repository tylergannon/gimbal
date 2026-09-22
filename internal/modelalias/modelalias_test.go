package modelalias

import (
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		selection Selection
		model     string
		harness   string
		effort    string
	}{
		{Selection{Name: "gpt"}, "gpt-6-sol", "codex", "high"},
		{Selection{Name: "fable"}, "claude-fable-5-1", "claude", "high"},
		{Selection{Name: "flash"}, "gemini-3.8-flash-medium", "agy", "medium"},
		{Selection{Name: "flash", Version: "3.7", VersionPresent: true, Effort: "low", EffortPresent: true}, "gemini-3.7-flash-low", "agy", "low"},
		{Selection{Name: "gemini-3.8-flash-low"}, "gemini-3.8-flash-low", "agy", "low"},
		{Selection{Name: "opencode/ling-3.0-flash-fin-free"}, "ling-3.0-flash-fin-free", "opencode", ""},
		{Selection{Name: "opencode/gemini-model-name-low"}, "gemini-model-name-low", "opencode", ""},
		{Selection{Name: "opencode/openrouter/vendor/future-model", Effort: "medium", EffortPresent: true}, "openrouter/vendor/future-model", "opencode", "medium"},
	}
	for _, test := range tests {
		got, err := Resolve(test.selection)
		if err != nil {
			t.Fatal(err)
		}
		if got.Model != test.model || got.Harness != test.harness || got.Effort != test.effort {
			t.Fatalf("Resolve(%+v) = %+v", test.selection, got)
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
		{Selection{Name: "gemini-3.8-flash-low", Effort: "high", EffortPresent: true}, "fixes effort"},
		{Selection{Name: "opencode/"}, "expected opencode/<model-id>"},
		{Selection{Name: "opencode/provider/"}, "expected opencode/<model-id>"},
		{Selection{Name: "opencode/ling-3.0", Version: "1", VersionPresent: true}, "cannot also declare version"},
	}
	for _, test := range tests {
		_, err := Resolve(test.selection)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("Resolve(%+v) error = %v, want %q", test.selection, err, test.want)
		}
	}
}
