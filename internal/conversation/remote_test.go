package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/environments"
)

type remoteFixture struct {
	mu                sync.Mutex
	processes         map[string][]byte
	processAdmissions int
	artifactFailures  int
	turns             map[string]backend.Result
	sessions          int
	admissions        int
	receivedTurns     []backend.Turn
	interruptions     int
	cleanup           int
	workdir           string
	autoComplete      bool
	sessionReplyLost  bool
	sessionStarted    chan struct{}
	sessionRelease    chan struct{}
	forgetOnCleanup   bool
	cleanupCompletes  bool
}

func (f *remoteFixture) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	encode := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	switch r.URL.Path {
	case "/health":
		w.WriteHeader(200)
	case "/tasks/identified-process":
		f.processAdmissions++
		var p backend.Process
		_ = json.NewDecoder(r.Body).Decode(&p)
		var in environments.HelperRequest
		_ = json.Unmarshal([]byte(p.Args[1]), &in)
		directory := environments.Directory{Path: in.Path, Exists: true, Git: true, Branch: "main", Base: "main", Bases: []string{"main", "release"}, ExecutionDirectory: in.Path, Entries: []environments.DirectoryEntry{}}
		result := environments.HelperResult{Directory: directory, Build: "fixture-build", Harnesses: []environments.Harness{{ID: "claude", Default: "claude-haiku-4-5", Models: []string{"claude-haiku-4-5"}, State: environments.HarnessReady}}}
		if in.Operation == "prepare" {
			result.Prepared = environments.Prepared{Directory: in.Path, Branch: "main", BaseCommit: "base"}
			if !in.Checkout.SharedCheckout {
				result.Prepared.Directory = in.Path + "/worktrees/" + in.OperationID
				result.Prepared.Branch = in.Checkout.Branch
			}
		}
		f.processes[p.Run+"/"+p.Task], _ = json.Marshal(result)
		encode(backend.Reply{})
	case "/tasks/bytes":
		if f.artifactFailures > 0 {
			f.artifactFailures--
			http.Error(w, "temporary stdout outage", http.StatusServiceUnavailable)
			return
		}
		var a backend.Artifact
		_ = json.NewDecoder(r.Body).Decode(&a)
		_, _ = w.Write(f.processes[a.Run+"/"+a.Task])
	case "/tasks/identified-session":
		var s backend.Session
		_ = json.NewDecoder(r.Body).Decode(&s)
		f.sessions++
		if f.sessionStarted != nil {
			close(f.sessionStarted)
			<-f.sessionRelease
		}
		f.workdir = s.Workdir
		if f.sessionReplyLost {
			http.Error(w, "lost session admission reply", http.StatusServiceUnavailable)
			return
		}
		encode(backend.Reply{ID: s.Task})
	case "/tasks/identified-turn":
		var turn backend.Turn
		_ = json.NewDecoder(r.Body).Decode(&turn)
		f.admissions++
		f.receivedTurns = append(f.receivedTurns, turn)
		raw, _ := json.Marshal(backend.TurnResult{Output: []byte(`"remote reply"`)})
		f.turns[turn.Task] = backend.Result{Done: f.autoComplete, Result: raw}
		encode(backend.Reply{})
	case "/tasks/result":
		var target backend.Target
		_ = json.NewDecoder(r.Body).Decode(&target)
		if _, ok := f.processes[target.Run+"/"+target.ID]; ok {
			raw, _ := json.Marshal(backend.ProcessResult{})
			encode(backend.Result{Done: true, Result: raw})
			return
		}
		result, ok := f.turns[target.ID]
		if !ok {
			result = backend.Result{Done: true, Error: "task not found"}
		}
		encode(result)
	case "/tasks/events":
		encode(backend.Events{})
	case "/tasks/interrupt":
		var target backend.Target
		_ = json.NewDecoder(r.Body).Decode(&target)
		f.interruptions++
		result := f.turns[target.ID]
		result.Done, result.Interrupted = true, true
		f.turns[target.ID] = result
		encode(backend.Reply{})
	case "/tasks/cleanup":
		var scope backend.Scope
		_ = json.NewDecoder(r.Body).Decode(&scope)
		if scope.Run == scope.Scope {
			f.cleanup++
			if f.forgetOnCleanup {
				f.turns = map[string]backend.Result{}
			}
			for id, result := range f.turns {
				result.Done, result.Interrupted = true, !f.cleanupCompletes
				f.turns[id] = result
			}
		}
		encode(backend.Reply{})
	case "/tasks/steer":
		encode(backend.Reply{Value: false})
	default:
		http.Error(w, "missing", 404)
	}
}
func remoteManager(t *testing.T, f *remoteFixture) (*Manager, *environments.Service, string) {
	t.Helper()
	f.processes = map[string][]byte{}
	f.turns = map[string]backend.Result{}
	server := httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(server.Close)
	project, instance := t.TempDir(), t.TempDir()
	raw, _ := json.Marshal(environments.Config{Version: 1, Machines: []environments.Machine{{ID: "remote", Name: "Build box", Kind: environments.KindAddress, Address: server.URL}}, Environments: []environments.Environment{{ID: "remote-project", Project: "project", Machine: "remote", Directory: "/remote/project"}}})
	if err := os.WriteFile(filepath.Join(instance, "environments.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	service, err := environments.New(t.Context(), environments.Options{InstanceDir: instance})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	if err = service.RegisterProject("project", project); err != nil {
		t.Fatal(err)
	}
	m, err := newMetadataManager(t.Context(), filepath.Join(project, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) {
		t.Fatal("remote chat used local adapter")
		return nil, nil
	}, "/local/gimbal", instance, project, nil)
	if err != nil {
		t.Fatal(err)
	}
	m.SetEnvironments(service, "project")
	t.Cleanup(m.Close)
	return m, service, project
}
func TestAlternateLocalEnvironmentUsesLocalHarnessAndSelectedDirectory(t *testing.T) {
	project, alternate, instance := t.TempDir(), t.TempDir(), t.TempDir()
	alternate, _ = filepath.EvalSymlinks(alternate)
	raw, _ := json.Marshal(environments.Config{Version: 1, Machines: []environments.Machine{}, Environments: []environments.Environment{{ID: "local-notes", Project: "project", Machine: environments.Local, Directory: alternate}}})
	if err := os.WriteFile(filepath.Join(instance, "environments.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	service, err := environments.New(t.Context(), environments.Options{InstanceDir: instance})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = service.Close() }()
	if err = service.RegisterProject("project", project); err != nil {
		t.Fatal(err)
	}
	adapter := &chatAdapter{sessions: map[string][]string{}}
	manager, err := newMetadataManager(t.Context(), filepath.Join(project, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) { return adapter, nil }, "/local/gimbal", instance, project, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.SetEnvironments(service, "project")
	item, err := manager.Start(t.Context(), NewConversation{Provider: "claude", Environment: "local-notes"}, "Remember cobalt.")
	if err != nil {
		t.Fatal(err)
	}
	done := waitRequest(t, manager, item.ID, item.Requests[0].ID, "completed")
	if done.Worktree != alternate || done.Placement.Value.Machine.ID != environments.Local || done.Environment != "local-notes" {
		t.Fatalf("alternate placement=%+v", done)
	}
}

func TestRemotePreparationRetryRetainsOriginalCheckoutIdentity(t *testing.T) {
	f := &remoteFixture{artifactFailures: 2}
	m, service, project := remoteManager(t, f)
	in := NewConversation{Provider: "claude", Title: "Retained setup", Environment: "remote-project", Branch: "feature/retained", Base: "main"}
	_, err := m.Create(t.Context(), in)
	var uncertain *backend.DeliveryUncertain
	if !errors.As(err, &uncertain) {
		t.Fatalf("lost Prepare stdout was not retained as uncertain: %v", err)
	}
	pending := m.List()
	if len(pending) != 1 || pending[0].Status != "unresolved" || !pending[0].CleanupPending {
		t.Fatalf("unconfirmed preparation disappeared: %+v", pending)
	}
	original := pending[0].ID
	f.mu.Lock()
	admissions, cleanup := f.processAdmissions, f.cleanup
	f.mu.Unlock()
	if admissions != 1 || cleanup != 0 {
		t.Fatalf("uncertain preparation was replayed or cleaned before result reconciliation: admissions=%d cleanup=%d", admissions, cleanup)
	}
	m.Close()
	doc, err := service.Config(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.SaveConfig(t.Context(), environments.ConfigEdit{Revision: doc.Revision, Text: `{"version":1,"machines":[],"environments":[]}`}); err != nil {
		t.Fatal(err)
	}
	restored, err := newMetadataManager(t.Context(), filepath.Join(project, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) {
		t.Fatal("remote retry used local adapter")
		return nil, nil
	}, "/local/gimbal", m.instanceDir, project, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	restored.SetEnvironments(service, "project")

	item, err := restored.Create(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != original || !strings.Contains(item.Worktree, original) || item.Status != StatusIdle || item.Error != "" {
		t.Fatalf("retry replaced its original preparation: %+v", item)
	}
	f.mu.Lock()
	admissions = f.processAdmissions
	f.mu.Unlock()
	if admissions != 1 {
		t.Fatalf("explicit retry executed another checkout mutation: %d", admissions)
	}
	if err = restored.Cleanup(t.Context(), item.ID); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteConversationPassesItsRetainedTaskRouteToWorkflow(t *testing.T) {
	m, _, _ := remoteManager(t, &remoteFixture{})
	var admitted Conversation
	var taskRoute backend.Route
	m.WorkflowStart = func(_ context.Context, item Conversation, route backend.Route) error {
		admitted = item
		taskRoute = route
		return nil
	}
	item, err := m.Start(t.Context(), NewConversation{Provider: "claude", Environment: "remote-project", SharedCheckout: true}, "Remember remote context")
	if err != nil {
		t.Fatal(err)
	}
	if admitted.WorkflowRun == "" || admitted.Worktree != "/remote/project" || admitted.Environment != "remote-project" || taskRoute.Name != "http" {
		t.Fatalf("admission=%+v route=%+v", admitted, taskRoute)
	}
	m.mu.RLock()
	expected := m.remoteStates[item.ID].Target.Route
	m.mu.RUnlock()
	gotRaw, _ := json.Marshal(taskRoute)
	wantRaw, _ := json.Marshal(expected)
	if string(gotRaw) != string(wantRaw) {
		t.Fatalf("route changed: got %s want %s", gotRaw, wantRaw)
	}
}

func TestConversationPassesSelectedNamespaceBusSeparately(t *testing.T) {
	m, _, _ := remoteManager(t, &remoteFixture{})
	item, err := m.Create(t.Context(), NewConversation{Provider: "claude", Environment: "remote-project", SharedCheckout: true})
	if err != nil {
		t.Fatal(err)
	}
	bus := backend.Route{Name: "selected-bus", Configuration: json.RawMessage(`{"namespace":"configured"}`)}
	m.mu.Lock()
	m.remoteStates[item.ID].Target.Route = bus
	m.active[item.ID].remote.Target.Route = bus
	m.mu.Unlock()
	var got Conversation
	var route backend.Route
	m.WorkflowStart = func(_ context.Context, item Conversation, selected backend.Route) error {
		got, route = item, selected
		return nil
	}
	m.WorkflowInput = func(string, string, string) error { return nil }
	_, err = m.Submit(item.ID, Request{ID: "first", Kind: "prompt", Message: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Environment != "remote-project" || route.Name != bus.Name || string(route.Configuration) != string(bus.Configuration) || route.Environment != "" {
		t.Fatalf("input=%+v route=%+v", got, route)
	}
}

func TestConversationPreflightRejectsBeforeRemotePreparation(t *testing.T) {
	fixture := &remoteFixture{}
	m, _, _ := remoteManager(t, fixture)
	called := false
	m.WorkflowPrepare = func(_ context.Context, environment string, route backend.Route) error {
		called = environment == "remote-project" && route.Name == "http"
		return errors.New("preflight rejected")
	}
	_, err := m.Create(t.Context(), NewConversation{Provider: "claude", Environment: "remote-project", SharedCheckout: true})
	if err == nil || !strings.Contains(err.Error(), "preflight rejected") || !called {
		t.Fatalf("preflight: called=%v err=%v", called, err)
	}
	fixture.mu.Lock()
	processes, sessions := fixture.processAdmissions, fixture.sessions
	fixture.mu.Unlock()
	if processes != 0 || sessions != 0 {
		t.Fatalf("remote effects before preflight: processes=%d sessions=%d", processes, sessions)
	}
}

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
	cfg := environments.Config{Version: 1, Machines: []environments.Machine{{ID: "bus", Name: "bus", Kind: environments.KindRoute, Route: &route}}, Environments: []environments.Environment{{ID: "local-notes", Project: "project", Machine: environments.Local, Directory: selected}}}
	raw, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(instance, "environments.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	service, err := environments.New(t.Context(), environments.Options{InstanceDir: instance, Registry: registry})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
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
	m.Close()
	m = create()
	defer m.Close()
	m.WorkflowStart = func(_ context.Context, got Conversation, selectedRoute backend.Route) error {
		var payload map[string]bool
		if err := json.Unmarshal(selectedRoute.Configuration, &payload); err != nil || got.Environment != "local-notes" || selectedRoute.Name != route.Name || !payload["selected"] {
			t.Errorf("lost namespace: %+v %+v", got, selectedRoute)
		}
		return nil
	}
	m.WorkflowInput = func(string, string, string) error { return nil }
	if _, err := m.Submit(item.ID, Request{ID: "first", Kind: "prompt", Message: "hello"}); err != nil {
		t.Fatal(err)
	}
}
