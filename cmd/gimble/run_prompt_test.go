package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePromptModelPrecedence(t *testing.T) {
	tests := []struct {
		name    string
		options runPromptOptions
		caller  promptCaller
		harness string
		wantErr bool
	}{
		{name: "Codex gets opposite provider", caller: promptCallerCodex, harness: "claude"},
		{name: "Claude gets opposite provider", caller: promptCallerClaude, harness: "codex"},
		{name: "explicit flash effort", caller: promptCallerCodex, options: runPromptOptions{model: "flash", effort: "low"}, harness: "agy"},
		{name: "OpenCode default provider", caller: promptCallerNone, options: runPromptOptions{model: "opencode/ling-3.0-flash-fin-free"}, harness: "opencode"},
		{name: "OpenCode explicit provider", caller: promptCallerNone, options: runPromptOptions{model: "opencode/opencode/ling-3.0-flash-fin-free"}, harness: "opencode"},
		{name: "standalone requires model", caller: promptCallerNone, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolvePromptModel(test.options, test.caller)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Harness != test.harness {
				t.Fatalf("selection = %#v", got)
			}
		})
	}
}

// TestResolvePromptModelCarriesEffort: every harness takes the reasoning
// effort a role is bound to, so only agy's own limit rejects one.
func TestResolvePromptModelCarriesEffort(t *testing.T) {
	for _, model := range []string{"gpt", "fable"} {
		resolved, err := resolvePromptModel(runPromptOptions{model: model, effort: "max"}, promptCallerNone)
		if err != nil {
			t.Fatalf("model %s error = %v", model, err)
		}
		if resolved.Effort != "max" {
			t.Fatalf("model %s effort = %q, want max", model, resolved.Effort)
		}
	}
	if _, err := resolvePromptModel(runPromptOptions{model: "flash", effort: "max"}, promptCallerNone); err == nil || !strings.Contains(err.Error(), "low, medium, or high") {
		t.Fatalf("flash max error = %v", err)
	}
}

func TestDetectPromptCallerPrefersClaude(t *testing.T) {
	environment := map[string]string{"CLAUDE_CODE_SESSION_ID": "claude", "CODEX_THREAD_ID": "codex"}
	if got := detectPromptCaller(func(name string) string { return environment[name] }); got != promptCallerClaude {
		t.Fatalf("caller = %q", got)
	}
}

func TestRunPromptJSONValidatesExactSchema(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}`)
	compiled, err := compilePromptSchema(schema)
	if err != nil {
		t.Fatal(err)
	}
	runPromptSchemaMu.Lock()
	defer runPromptSchemaMu.Unlock()
	runPromptSchema, runPromptValidator = schema, compiled
	if err := (runPromptJSON{}).ValidateJSON([]byte(`{"answer":"yes"}`)); err != nil {
		t.Fatal(err)
	}
	if err := (runPromptJSON{}).ValidateJSON([]byte(`{"answer":1}`)); err == nil {
		t.Fatal("invalid structured output was accepted")
	}
}

func TestPromptProjectDirRejectsNonemptyDirectory(t *testing.T) {
	dir := t.TempDir()
	if _, err := promptProjectDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/occupied", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := promptProjectDir(dir); err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("error = %v", err)
	}
}

func TestWriteRunPromptLogsNamesTheCompletedRun(t *testing.T) {
	project := t.TempDir()
	runDir := filepath.Join(project, "runs", "run-217")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if err := writeRunPromptLogs(&stderr, project); err != nil {
		t.Fatal(err)
	}
	if got, want := stderr.String(), "Logs: "+runDir+"\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}
