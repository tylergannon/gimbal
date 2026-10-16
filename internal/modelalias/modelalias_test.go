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
		{Selection{Name: "sonnet"}, "claude", "high"},
		{Selection{Name: "haiku"}, "claude", "high"},
		{Selection{Name: "luna"}, "codex", "high"},
		{Selection{Name: "terra"}, "codex", "high"},
		{Selection{Name: "sol"}, "codex", "high"},
		{Selection{Name: "astra"}, "codex", "high"},
		{Selection{Name: "deepseek-flash"}, "pi", ""},
		{Selection{Name: "glm"}, "pi", ""},
		{Selection{Name: "glm-flash"}, "pi", ""},
		{Selection{Name: "glm-vision"}, "pi", ""},
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

// The unversioned alias is the newest generated version of its family, and an
// explicit version selects that version, whatever today's catalog holds.
func TestFamilyAliasIsNewestGeneratedVersion(t *testing.T) {
	for name, versions := range families {
		if name == "flash" {
			continue
		}
		newest := ""
		for version := range versions {
			if newest == "" || newerVersion(version, newest) {
				newest = version
			}
		}
		got, err := Resolve(Selection{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		if got.Version != newest || got.Model != versions[newest].native {
			t.Errorf("Resolve(%q) = %+v, want version %s", name, got, newest)
		}
		for version, entry := range versions {
			got, err := Resolve(Selection{Name: name, Version: version, VersionPresent: true})
			if err != nil || got.Model != entry.native {
				t.Errorf("Resolve(%q, version %s) = %+v, %v", name, version, got, err)
			}
		}
	}
}

func TestNewerVersionComparesNumerically(t *testing.T) {
	if !newerVersion("5.10", "5.9") || !newerVersion("5.5", "5") || newerVersion("5", "5.5") || newerVersion("6", "6") {
		t.Fatal("versions do not compare as dotted numbers")
	}
}
