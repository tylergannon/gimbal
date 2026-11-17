package modelalias

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestConfiguredAliasesReloadAndPreserveExplicitSelections(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	dir := filepath.Join(home, ".gimbal")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	selection := Selection{Name: "astra", Effort: "low", EffortPresent: true}
	builtin, err := Resolve(selection)
	if err != nil {
		t.Fatal(err)
	}

	for _, target := range []string{"gpt-future-astra", "gpt-next-astra"} {
		config := `{"modelAliases":{"astra":"` + target + `"},"secrets":{}}`
		if err := os.WriteFile(path, []byte(config), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := Resolve(selection)
		if err != nil || got.Model != target || got.Name != "astra" || got.Effort != "low" || got.Harness != "codex" {
			t.Fatalf("override = %+v, %v", got, err)
		}
		models, err := Newest()
		if err != nil || !slices.Contains(models["codex"], target) || slices.Contains(models["codex"], builtin.Model) {
			t.Fatalf("effective Codex models = %v, %v", models["codex"], err)
		}
	}
	explicit := selection
	explicit.Version, explicit.VersionPresent = builtin.Version, true
	if got, err := Resolve(explicit); err != nil || got != builtin {
		t.Fatalf("explicit version = %+v, %v, want %+v", got, err, builtin)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got, err := Resolve(selection); err != nil || got != builtin {
		t.Fatalf("after removing override = %+v, %v, want %+v", got, err, builtin)
	}
}

func TestInvalidConfiguredAliasesReportConfigPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	dir := filepath.Join(home, ".gimbal")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	for _, config := range []string{
		`{"modelAliases":`,
		`{"modelAliases":{"asttra":"gpt-future-astra"}}`,
		`{"modelAliases":{"astra":"sol"}}`,
		`{"modelAliases":{"astra":"claude-future"}}`,
	} {
		if err := os.WriteFile(path, []byte(config), 0600); err != nil {
			t.Fatal(err)
		}
		if got, err := Resolve(Selection{Name: "astra"}); err == nil || !strings.Contains(err.Error(), path) {
			t.Errorf("config %s resolved to %+v with error %v", config, got, err)
		}
	}
}
