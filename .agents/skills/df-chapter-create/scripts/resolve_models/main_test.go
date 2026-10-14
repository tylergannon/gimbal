package main

import (
	"os"
	"path/filepath"
	"testing"
)

func put(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}
func TestOverridesAndBindings(t *testing.T) {
	skill := t.TempDir()
	project := t.TempDir()
	user := filepath.Join(t.TempDir(), "models.yaml")
	put(t, filepath.Join(skill, "SKILL.md"), "name: df-sprint-plan\n```yaml\nmodel_defaults:\n  main:\n    model: inherit\n  codex:\n    model: gpt-test\n    reasoning_effort: high\n  gemini:\n    model: cli-default\n```\n")
	put(t, user, "skills:\n  df-sprint-plan:\n    codex:\n      reasoning_effort: low\n")
	put(t, filepath.Join(project, ".diffusion/skills/models.yaml"), "skills:\n  df-sprint-plan:\n    codex:\n      model: gpt-project\n      reasoning_effort: null\n")
	result, err := resolve(skill, project, user)
	if err != nil {
		t.Fatal(err)
	}
	roles := result["roles"].(map[string]any)
	codex := roles["codex"].(map[string]any)
	if codex["model"] != "gpt-project" || codex["reasoning_effort"] != nil || len(codex["model_args"].([]string)) != 2 {
		t.Fatalf("%#v", codex)
	}
	if roles["main"].(map[string]any)["cli"] != (*string)(nil) {
		t.Fatal("inherit binding")
	}
	_, _, err = invocation("df-sprint-plan", "codex", spec{"model": "claude-test"})
	if err == nil {
		t.Fatal("cross-provider accepted")
	}
	_, _, err = invocation("df-easy-loop-simple", "coding", spec{"model": "gemini-test"})
	if err == nil {
		t.Fatal("Gemini easy loop accepted")
	}
}
func TestYAMLRejectsDuplicateOrInvalidKeys(t *testing.T) {
	for _, text := range []string{"skills: {}\nskills: {}\n", "skills:\n  x:\n    model: a\n    model: b\n", "1: value\n", "[]\n", "{}\n---\n{}\n"} {
		if _, err := readYAML([]byte(text)); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
}
func TestBundledDefaults(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	name, roles, err := defaults(filepath.Join(dir, "../.."))
	if err != nil {
		t.Fatal(err)
	}
	for role, fields := range roles {
		if err := validate(fields, false); err != nil {
			t.Fatal(err)
		}
		if _, _, err := invocation(name, role, fields); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPartialOverrideSourcesAndErrors(t *testing.T) {
	skill, project := t.TempDir(), t.TempDir()
	user := filepath.Join(t.TempDir(), "models.yaml")
	put(t, filepath.Join(skill, "SKILL.md"), "name: df-sprint-plan\n```yaml\nmodel_defaults:\n  codex:\n    model: gpt-default\n    reasoning_effort: high\n```\n")
	put(t, user, "skills:\n  unrelated:\n    invalid_role: any value\n  df-sprint-plan:\n    codex:\n      reasoning_effort: low\n")
	projectConfig := filepath.Join(project, ".diffusion/skills/models.yaml")
	put(t, projectConfig, "skills:\n  df-sprint-plan:\n    codex:\n      model: gpt-project\n")
	result, err := resolve(skill, project, user)
	if err != nil {
		t.Fatal(err)
	}
	codex := result["roles"].(map[string]any)["codex"].(map[string]any)
	sources := codex["sources"].(map[string]string)
	if codex["reasoning_effort"] != "low" || sources["model"] != projectConfig || sources["reasoning_effort"] != user {
		t.Fatal(codex)
	}
	for _, text := range []string{"skills: null\n", "unexpected: {}\n", "skills:\n  df-sprint-plan:\n    unknown:\n      model: gpt-test\n", "skills:\n  df-sprint-plan:\n    codex:\n      reasoning_effort: extreme\n", "skills:\n  df-sprint-plan:\n    codex:\n      model: false\n"} {
		put(t, projectConfig, text)
		if _, err := resolve(skill, project, user); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
	for _, text := range []string{"", "# no overrides\n", "{}\n"} {
		put(t, projectConfig, text)
		if _, err := resolve(skill, project, user); err != nil {
			t.Fatalf("%q: %v", text, err)
		}
	}
}
