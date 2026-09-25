package main

import "testing"

func TestResolvePromptModelPrecedence(t *testing.T) {
	tests := []struct {
		name    string
		options runPromptOptions
		caller  promptCaller
		model   string
		effort  string
		wantErr bool
	}{
		{name: "Codex gets opposite provider", caller: promptCallerCodex, model: "claude-fable-5-1", effort: "high"},
		{name: "Claude gets opposite provider", caller: promptCallerClaude, model: "gpt-5.6-sol", effort: "high"},
		{name: "explicit model wins", caller: promptCallerCodex, options: runPromptOptions{model: "gpt", modelVersion: "5.6", effort: "max"}, model: "gpt-5.6-sol", effort: "max"},
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
			if got.Model != test.model || got.Effort != test.effort {
				t.Fatalf("selection = %#v", got)
			}
		})
	}
}

func TestDetectPromptCallerPrefersClaude(t *testing.T) {
	environment := map[string]string{"CLAUDE_CODE_SESSION_ID": "claude", "CODEX_THREAD_ID": "codex"}
	if got := detectPromptCaller(func(name string) string { return environment[name] }); got != promptCallerClaude {
		t.Fatalf("caller = %q", got)
	}
}
