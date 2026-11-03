package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/environments"
	"github.com/tylergannon/gimbal/internal/runstate"
)

func TestConfiguredLocalNamespaceKeepsBusAcrossReload(t *testing.T) {
	project, selected, instance := t.TempDir(), t.TempDir(), t.TempDir()
	selected, _ = filepath.EvalSymlinks(selected)
	registry := &gimbal.Registry{}
	err := registry.RegisterMessaging(gimbal.MessagingDefinition{Name: "local-namespace-test", ValidateConfiguration: func(json.RawMessage) error { return nil }, ValidateEnvironment: func(_ json.RawMessage, name string) error {
		if name != "local-notes" {
			return errors.New("unknown")
		}
		return nil
	}, Connect: func(context.Context, json.RawMessage, backend.Dependencies) (backend.Messaging, error) {
		t.Fatal("namespace selection connected")
		return nil, nil
	}, Worker: func(context.Context, json.RawMessage, backend.Dependencies, backend.MessageHandler) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	route := backend.Route{Name: "local-namespace-test", Configuration: json.RawMessage(`{"selected":true}`)}
	cfg := environments.Config{Version: 1, Machines: []environments.Machine{{ID: "bus", Name: "bus", Kind: environments.KindRoute, Route: &route}}, Environments: []environments.Environment{{ID: "local-notes", Project: "project", Machine: environments.Local, Directory: selected}, {ID: "unlisted", Project: "project", Machine: environments.Local, Directory: selected}}}
	raw, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(instance, "environments.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	service, err := environments.New(t.Context(), environments.Options{InstanceDir: instance, Registry: registry})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = service.Close() }()
	if err := service.RegisterProject("project", project); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(project, ".gimbal")
	create := func() *Manager {
		m, err := New(t.Context(), dir, func(string) (gimbal.HarnessAdapter, error) { t.Fatal("namespace used local adapter"); return nil, nil }, "", instance, project, nil)
		if err != nil {
			t.Fatal(err)
		}
		m.SetEnvironments(service, "project")
		return m
	}
	m := create()
	if _, err := m.SetupEnvironment(t.Context(), "unlisted"); err == nil || !strings.Contains(err.Error(), "requires a messaging namespace") {
		t.Fatalf("setup fell back for an unlisted environment: %v", err)
	}
	m.WorkflowPrepare = func(context.Context, string, backend.Route) error { return errors.New("preflight rejected") }
	if _, err := m.Create(t.Context(), NewConversation{Provider: "claude", Environment: "local-notes", Branch: "feature/test"}); err == nil || !strings.Contains(err.Error(), "preflight rejected") || len(m.List()) != 0 {
		t.Fatalf("preflight did not reject before preparation: %v", err)
	}
	prepared := false
	m.WorkflowPrepare = func(_ context.Context, env string, got backend.Route) error {
		prepared = env == "local-notes" && got.Name == route.Name && string(got.Configuration) == string(route.Configuration)
		return nil
	}
	item, err := m.Create(t.Context(), NewConversation{Provider: "claude", Environment: "local-notes", SharedCheckout: true})
	if err != nil {
		t.Fatal(err)
	}
	if !prepared || item.Worktree != selected {
		t.Fatalf("incorrect preflight or directory: %+v", item)
	}
	defaultCheckout, err := m.Create(t.Context(), NewConversation{Title: "Notes", Provider: "claude", Environment: "local-notes"})
	if err != nil || defaultCheckout.Branch != "" || defaultCheckout.SharedCheckout {
		t.Fatalf("default checkout must not invent an explicit branch: %+v %v", defaultCheckout, err)
	}
	isolated, err := m.Create(t.Context(), NewConversation{Provider: "claude", Environment: "local-notes", Branch: "feature/test", Base: "main"})
	if err != nil {
		t.Fatal(err)
	}
	m.Close()
	m = create()
	defer m.Close()
	if got, ok := m.Get(isolated.ID); !ok || got.ID != isolated.ID || got.SharedCheckout || got.Branch != "feature/test" || got.Base != "main" {
		t.Fatalf("lost checkout choices on reload: %+v", got)
	}
	m.WorkflowStart = func(_ context.Context, got Conversation, selectedRoute backend.Route) error {
		var payload map[string]bool
		if err := json.Unmarshal(selectedRoute.Configuration, &payload); err != nil || got.Environment != "local-notes" || selectedRoute.Name != route.Name || !payload["selected"] {
			t.Errorf("lost namespace: %+v %+v", got, selectedRoute)
		}
		return nil
	}
	fileID := cacheFixture(t, m, []byte("namespace attachment"))
	m.WorkflowInput = func(_, _, raw string) error {
		var input runstate.InteractionInput
		if err := json.Unmarshal([]byte(raw), &input); err != nil {
			return err
		}
		if len(input.Files) != 1 || string(input.Files[0].Bytes) != "namespace attachment" || strings.Contains(input.Message, m.projectDir) {
			t.Errorf("attachment was not sent as bytes: %+v", input)
		}
		return nil
	}
	if _, err := m.Submit(item.ID, Request{ID: "first", Kind: "prompt", Message: "hello", FileID: fileID}); err != nil {
		t.Fatal(err)
	}
	starts := 0
	m.WorkflowStart = func(_ context.Context, got Conversation, selected backend.Route) error {
		starts++
		if got.ID != isolated.ID || got.Branch != "feature/test" || got.Base != "main" || selected.Name != route.Name {
			t.Errorf("lost checkout identity or namespace: %+v %+v", got, selected)
		}
		return &backend.DeliveryUncertain{RequestID: got.WorkflowRun, Err: errors.New("admission reply lost")}
	}
	m.WorkflowInput = func(string, string, string) error { return nil }
	request := Request{ID: "isolated-first", Kind: "prompt", Message: "hello"}
	if _, err := m.Submit(isolated.ID, request); !errors.As(err, new(*backend.DeliveryUncertain)) {
		t.Fatalf("expected uncertain admission: %v", err)
	}
	retained, _ := m.Get(isolated.ID)
	if retained.WorkflowRun == "" {
		t.Fatal("uncertain admission lost workflow identity")
	}
	if _, err := m.Submit(isolated.ID, request); err != nil {
		t.Fatal(err)
	}
	again, _ := m.Get(isolated.ID)
	if starts != 1 || again.WorkflowRun != retained.WorkflowRun {
		t.Fatalf("retry duplicated checkout: %d starts, %q then %q", starts, retained.WorkflowRun, again.WorkflowRun)
	}
}
