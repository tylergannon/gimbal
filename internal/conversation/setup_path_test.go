package conversation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/binding"
	"github.com/tylergannon/gimbal/internal/environments"
)

// The native UI offers what SetupEnvironment returns for the local machine.
// Each offered selection must create, survive a reload, and bind its harness.
func TestLocalSetupSelectionsCreateReloadAndBind(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "opencode"), []byte(`#!/bin/sh
[ "$1" = models ] || exit 1
printf 'opencode/beam-free\nopenai/gpt-5-nano\n'
`), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	project, instance := t.TempDir(), t.TempDir()
	service, err := environments.New(t.Context(), environments.Options{InstanceDir: instance})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	if err = service.RegisterProject("project", project); err != nil {
		t.Fatal(err)
	}
	factory := func(string) (gimbal.HarnessAdapter, error) { return &chatAdapter{sessions: map[string][]string{}}, nil }
	state := filepath.Join(project, ".gimbal")
	m, err := newMetadataManager(t.Context(), state, factory, "gimbal", instance, project, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	m.SetEnvironments(service, "project")
	setup, err := m.SetupEnvironment(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	offered := map[string]Harness{}
	for _, harness := range setup.Harnesses {
		offered[harness.ID] = harness
	}
	if got := offered["opencode"]; got.Default != "opencode/opencode/beam-free" || !slices.Equal(got.Models, []string{"opencode/opencode/beam-free", "opencode/openai/gpt-5-nano"}) {
		t.Errorf("opencode offers default %q models %q", got.Default, got.Models)
	}
	if _, err := m.Create(t.Context(), NewConversation{Provider: "opencode", Model: "opencode/gpt-5-nano", SharedCheckout: true}); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("stale OpenCode model was not rejected: %v", err)
	}
	if got := len(m.List()); got != 0 {
		t.Fatalf("unavailable selection created %d conversations", got)
	}
	wantPi := []string{"pi/diffusion/deepseek-4.1-flash", "pi/diffusion/glm-5.3", "pi/diffusion/glm-5.3-flash", "pi/diffusion/glm-5.3-vision"}
	if got := offered["pi"]; got.Default != wantPi[0] || !slices.Equal(got.Models, wantPi) {
		t.Errorf("pi offers default %q models %q", got.Default, got.Models)
	}
	natives := map[string]string{"opencode/opencode/beam-free": "opencode/beam-free", "opencode/openai/gpt-5-nano": "openai/gpt-5-nano", "pi/diffusion/glm-5.3": "diffusion/glm-5.3", "gpt-6-luna": "gpt-6-luna", "claude-haiku-4-5": "claude-haiku-4-5"}
	for _, harness := range setup.Harnesses {
		for _, model := range harness.Models {
			if n := len(slices.DeleteFunc(slices.Clone(harness.Models), func(other string) bool { return other != model })); n != 1 {
				t.Errorf("%s offers %q %d times", harness.ID, model, n)
			}
			item, err := m.Create(t.Context(), NewConversation{Provider: harness.ID, Model: model, SharedCheckout: true})
			if err != nil {
				t.Fatalf("%s %q: %v", harness.ID, model, err)
			}
			if item.Provider != harness.ID || item.Model != model {
				t.Errorf("created %s/%q, offered %s/%q", item.Provider, item.Model, harness.ID, model)
			}
			reloaded, err := newMetadataManager(t.Context(), state, factory, "gimbal", instance, project, nil)
			if err != nil {
				t.Fatal(err)
			}
			saved, ok := reloaded.Get(item.ID)
			var started Conversation
			reloaded.WorkflowStart = func(_ context.Context, item Conversation, _ backend.Route) error {
				started = item
				_, err := RoleSpec(item)
				return err
			}
			reloaded.WorkflowInput = func(string, string, string) error { return nil }
			if _, err := reloaded.Submit(item.ID, Request{ID: "request-" + item.ID, Created: time.Now().UnixMilli(), Kind: "prompt", Message: "hello"}); err != nil {
				t.Fatalf("%s %q: %v", harness.ID, model, err)
			}
			reloaded.Close()
			if !ok || saved.Provider != harness.ID || saved.Model != model || started.Provider != harness.ID || started.Model != model {
				t.Errorf("%s %q reloaded as %s/%q, started as %s/%q", harness.ID, model, saved.Provider, saved.Model, started.Provider, started.Model)
			}
			spec, _ := RoleSpec(started)
			models, err := binding.Roles(map[gimbal.WorkflowRole]string{gimbal.RoleAgenticDialogues: spec})
			if err != nil {
				t.Fatal(err)
			}
			if want, ok := natives[model]; ok && models[gimbal.RoleAgenticDialogues].Model != want {
				t.Errorf("%s %q bound native %q, want %q", harness.ID, model, models[gimbal.RoleAgenticDialogues].Model, want)
			}
		}
	}
}

// An older remote helper may report a Pi default that is no longer routable.
// Only its valid reported models are offered, led by the first; nothing the
// machine did not report is added.
func TestObservedHarnessesOfferOnlyValidReportedModels(t *testing.T) {
	got := observedHarnesses([]environments.Harness{
		{ID: "pi", Default: "anthropic/claude-haiku-4-5", Models: []string{"anthropic/claude-haiku-4-5", "diffusion/glm-5.3", "pi/diffusion/glm-5.3"}},
		{ID: "opencode", Default: "gpt-5-nano", Models: []string{"gpt-5-nano"}},
		{ID: "agy", Default: "claude-haiku-4-5", Models: []string{"claude-haiku-4-5"}},
	}, nil)
	if len(got) != 2 || got[0].Default != "pi/diffusion/glm-5.3" || !slices.Equal(got[0].Models, []string{"pi/diffusion/glm-5.3"}) {
		t.Fatalf("pi offered %+v", got)
	}
	if got[1].Default != "opencode/gpt-5-nano" || !slices.Equal(got[1].Models, []string{"opencode/gpt-5-nano"}) {
		t.Fatalf("opencode offered %+v", got[1])
	}
}

// A run that never started leaves no run identifier; the conversation stays
// idle, and the next message starts a fresh run.
func TestFailedWorkflowStartLeavesRecoverableConversation(t *testing.T) {
	dir := t.TempDir()
	m, err := newMetadataManager(t.Context(), filepath.Join(dir, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) { return &chatAdapter{sessions: map[string][]string{}}, nil }, "gimbal", "instance", dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	ok := m.WorkflowStart
	m.WorkflowStart = func(context.Context, Conversation, backend.Route) error { return errors.New("route unavailable") }
	item, err := m.Create(t.Context(), NewConversation{Provider: "claude", SharedCheckout: true})
	if err == nil {
		_, err = m.Submit(item.ID, Request{ID: "first", Created: 1, Kind: "prompt", Message: "hello"})
	}
	if err == nil || err.Error() != "route unavailable" {
		t.Fatalf("Submit = %v", err)
	}
	got, _ := m.Get(item.ID)
	if got.WorkflowRun != "" || got.Status != StatusIdle || !got.Live || got.Error != "" || len(got.Requests) != 0 {
		t.Fatalf("after failed start: run %q status %q live %v error %q requests %d", got.WorkflowRun, got.Status, got.Live, got.Error, len(got.Requests))
	}
	m.WorkflowStart = ok
	state, err := m.Submit(item.ID, Request{ID: "second", Created: 2, Kind: "prompt", Message: "hello again"})
	if err != nil || state.Status != "accepted" {
		t.Fatalf("retry = %+v, %v", state, err)
	}
}

// When the host cannot say whether the run was admitted, the assigned run
// identity stays, and the retry sends input to it instead of starting another.
func TestUncertainWorkflowStartKeepsRunIdentity(t *testing.T) {
	dir := t.TempDir()
	m, err := newMetadataManager(t.Context(), filepath.Join(dir, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) { return &chatAdapter{sessions: map[string][]string{}}, nil }, "gimbal", "instance", dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	starts := 0
	m.WorkflowStart = func(_ context.Context, item Conversation, _ backend.Route) error {
		starts++
		return &backend.DeliveryUncertain{RequestID: item.WorkflowRun, Err: errors.New("admission reply lost")}
	}
	m.WorkflowInput = func(string, string, string) error { return nil }
	item, err := m.Create(t.Context(), NewConversation{Provider: "claude", SharedCheckout: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.Submit(item.ID, Request{ID: "first", Created: 1, Kind: "prompt", Message: "hello"})
	if _, ok := errors.AsType[*backend.DeliveryUncertain](err); !ok {
		t.Fatalf("Submit = %v", err)
	}
	kept, _ := m.Get(item.ID)
	if kept.WorkflowRun == "" {
		t.Fatal("uncertain start released its run identifier")
	}
	if _, err := m.Submit(item.ID, Request{ID: "first", Created: 1, Kind: "prompt", Message: "hello"}); err != nil {
		t.Fatal(err)
	}
	if again, _ := m.Get(item.ID); starts != 1 || again.WorkflowRun != kept.WorkflowRun {
		t.Fatalf("retry started %d runs; run %q then %q", starts, kept.WorkflowRun, again.WorkflowRun)
	}
}
