package conversation

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/binding"
)

// The saved provider and model are the whole selection; the workflow role
// must bind the adapter and provider-native model that selection names.
func TestSelectedHarnessSurvivesRoleBinding(t *testing.T) {
	for _, tc := range []struct {
		provider, model, effort string
		stored, native, adapter string
	}{
		{"opencode", "opencode/gpt-5-nano", "", "opencode/gpt-5-nano", "gpt-5-nano", "opencode"},
		{"opencode", "gpt-5-nano", "", "opencode/gpt-5-nano", "gpt-5-nano", "opencode"},
		{"opencode", "", "", "opencode/gpt-5-nano", "gpt-5-nano", "opencode"},
		{"opencode", "anthropic/claude-haiku-4-5", "", "opencode/anthropic/claude-haiku-4-5", "anthropic/claude-haiku-4-5", "opencode"},
		{"opencode", "opencode/opencode/gpt-5-nano", "", "opencode/opencode/gpt-5-nano", "opencode/gpt-5-nano", "opencode"},
		{"codex", "gpt-6-luna", "high", "gpt-6-luna", "gpt-6-luna", "codex"},
		{"claude", "claude-haiku-4-5", "", "claude-haiku-4-5", "claude-haiku-4-5", "claude"},
		{"pi", "diffusion/glm-5.3", "", "pi/diffusion/glm-5.3", "diffusion/glm-5.3", "pi"},
		{"pi", "", "", "pi/diffusion/deepseek-4.1-flash", "diffusion/deepseek-4.1-flash", "pi"},
	} {
		t.Run(tc.provider+"/"+tc.model+tc.effort, func(t *testing.T) {
			stored, err := selectModel(tc.provider, tc.model, tc.effort)
			if err != nil {
				t.Fatal(err)
			}
			if stored != tc.stored {
				t.Fatalf("stored model = %q, want %q", stored, tc.stored)
			}
			spec, err := RoleSpec(Conversation{Provider: tc.provider, Model: stored, Effort: tc.effort})
			if err != nil {
				t.Fatal(err)
			}
			models, err := binding.Roles(map[gimbal.WorkflowRole]string{gimbal.RoleAgenticDialogues: spec})
			if err != nil {
				t.Fatal(err)
			}
			bound := models[gimbal.RoleAgenticDialogues]
			if bound.Model != tc.native {
				t.Errorf("bound model = %q, want %q", bound.Model, tc.native)
			}
			adapter := fmt.Sprintf("%T", bound.Adapter)
			if (tc.adapter == "opencode") != strings.Contains(adapter, "opencode") {
				t.Errorf("adapter %s does not serve %s", adapter, tc.adapter)
			}
			if tc.adapter != "opencode" {
				want, _ := binding.Adapter(tc.adapter)
				if adapter != fmt.Sprintf("%T", want) {
					t.Errorf("adapter %s, want %T for %s", adapter, want, tc.adapter)
				}
			}
		})
	}
}

// A saved session whose model would run on another harness fails before
// the workflow starts, instead of surfacing that harness's account error.
func TestRoleSpecRefusesModelOfAnotherHarness(t *testing.T) {
	_, err := RoleSpec(Conversation{Provider: "opencode", Model: "gpt-5-nano"})
	if err == nil || !strings.Contains(err.Error(), "runs on codex") {
		t.Fatalf("RoleSpec = %v, want refusal naming codex", err)
	}
	for _, tc := range [][2]string{{"codex", "claude-haiku-4-5"}, {"opencode", "claude-haiku-4-5"}, {"pi", "anthropic/claude-haiku-4-5"}, {"claude", "gpt-6-luna"}} {
		if _, err := RoleSpec(Conversation{Provider: tc[0], Model: tc[1]}); err == nil {
			t.Errorf("%s accepted %q", tc[0], tc[1])
		}
	}
}

func TestCreateRefusesUnroutableModel(t *testing.T) {
	for _, tc := range [][2]string{{"pi", "anthropic/claude-haiku-4-5"}, {"opencode", "opencode/"}, {"codex", "claude-haiku-4-5"}} {
		if _, err := selectModel(tc[0], tc[1], ""); err == nil {
			t.Errorf("%s accepted %q", tc[0], tc[1])
		}
	}
}

func TestSetupOffersOnlyModelsThatKeepTheirHarness(t *testing.T) {
	dir := t.TempDir()
	m, err := newMetadataManager(t.Context(), dir+"/.gimbal", func(string) (gimbal.HarnessAdapter, error) { return nil, nil }, "gimbal", "instance", dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	setup, err := m.Setup(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, harness := range setup.Harnesses {
		for _, model := range append([]string{harness.Default}, harness.Models...) {
			stored, err := selectModel(harness.ID, model, "")
			if err != nil || stored != model {
				t.Errorf("%s offers %q: selects %q, %v", harness.ID, model, stored, err)
			}
			if _, err := RoleSpec(Conversation{Provider: harness.ID, Model: stored}); err != nil {
				t.Errorf("%s offers %q: %v", harness.ID, model, err)
			}
		}
	}
}
