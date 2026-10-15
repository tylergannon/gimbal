package modelalias

import (
	"slices"
	"testing"
)

func TestExplicitEffortUsesKnownModelSupport(t *testing.T) {
	for _, tc := range []struct {
		harness, model, effort string
		supported              bool
	}{
		{"claude", "claude-haiku-4-5", "high", false},
		{"claude", "claude-sonnet-4-5", "high", false},
		{"claude", "claude-sonnet-4-6", "max", true},
		{"claude", "claude-opus-4-6", "xhigh", false},
		{"claude", "claude-fable-5-1", "xhigh", true},
		{"claude", "claude-unverified-9", "high", false},
		{"codex", "gpt-5.6-luna", "high", true},
		{"codex", "unknown-model", "high", false},
		{"agy", "gemini-3.8-flash-low", "high", false},
		{"agy", "gemini-3.8-flash-low", "low", true},
		{"pi", "diffusion/glm-5.3", "high", false},
		{"opencode", "opencode/gpt-5-nano", "high", false},
	} {
		if got := slices.Contains(Efforts(tc.harness, tc.model), tc.effort); got != tc.supported {
			t.Errorf("%s/%s/%s: %v", tc.harness, tc.model, tc.effort, got)
		}
	}
}
