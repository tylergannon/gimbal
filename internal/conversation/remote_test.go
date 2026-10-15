package conversation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/environments"
)

type remoteFixture struct {
	mu            sync.Mutex
	processes     map[string][]byte
	turns         map[string]backend.Result
	sessions      int
	admissions    int
	interruptions int
	cleanup       int
	workdir       string
	autoComplete  bool
}

func (f *remoteFixture) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	encode := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	switch r.URL.Path {
	case "/health":
		w.WriteHeader(200)
	case "/tasks/process":
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
		var a backend.Artifact
		_ = json.NewDecoder(r.Body).Decode(&a)
		_, _ = w.Write(f.processes[a.Run+"/"+a.Task])
	case "/tasks/session":
		var s backend.Session
		_ = json.NewDecoder(r.Body).Decode(&s)
		f.sessions++
		f.workdir = s.Workdir
		encode(backend.Reply{ID: s.Task})
	case "/tasks/turn":
		var turn backend.Turn
		_ = json.NewDecoder(r.Body).Decode(&turn)
		f.admissions++
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
			for id, result := range f.turns {
				result.Done, result.Interrupted = true, true
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
	m, err := New(t.Context(), filepath.Join(project, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) {
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
func TestRemoteConversationPlacementStopAndExplicitCleanup(t *testing.T) {
	f := &remoteFixture{}
	m, _, _ := remoteManager(t, f)
	item, err := m.Start(t.Context(), NewConversation{Provider: "claude", Environment: "remote-project", Branch: "feature/remote", Base: "release"}, "Work remotely")
	if err != nil {
		t.Fatal(err)
	}
	waitRequest(t, m, item.ID, item.Requests[0].ID, "running")
	deadline := time.Now().Add(time.Second)
	for {
		f.mu.Lock()
		started := f.admissions > 0
		f.mu.Unlock()
		if started {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("turn never admitted")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err = m.Submit(item.ID, Request{ID: "stop", Created: time.Now().UnixMilli(), Kind: "interrupt", Target: item.Requests[0].ID}); err != nil {
		t.Fatal(err)
	}
	stopped := waitRequest(t, m, item.ID, item.Requests[0].ID, "canceled")
	waitRequest(t, m, item.ID, "stop", "completed")
	if !stopped.Live || stopped.Placement.Value.Machine.ID != "remote" || stopped.Branch != "feature/remote" {
		t.Fatalf("placement=%+v", stopped)
	}
	f.mu.Lock()
	wd, sessions := f.workdir, f.sessions
	f.mu.Unlock()
	if wd != "/remote/project/worktrees/"+item.ID || sessions != 1 {
		t.Fatalf("execution=%q sessions=%d", wd, sessions)
	}
	if err = m.Cleanup(t.Context(), item.ID); err != nil {
		t.Fatal(err)
	}
	closed, _ := m.Get(item.ID)
	if closed.Live || closed.CleanupPending {
		t.Fatalf("cleanup=%+v", closed)
	}
}
func TestRemoteConversationSurvivesDesktopCloseAndReconcilesOriginalRequest(t *testing.T) {
	f := &remoteFixture{}
	first, service, project := remoteManager(t, f)
	item, err := first.Start(t.Context(), NewConversation{Provider: "claude", Environment: "remote-project", SharedCheckout: true}, "Keep running after desktop closes")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		f.mu.Lock()
		started := f.admissions > 0
		f.mu.Unlock()
		if started {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("turn never admitted")
		}
		time.Sleep(time.Millisecond)
	}
	first.Close()
	f.mu.Lock()
	if f.interruptions != 0 || f.cleanup != 0 {
		t.Fatalf("desktop stopped remote work: %+v", f)
	}
	requestID := item.Requests[0].ID
	result := f.turns[requestID]
	result.Done = true
	f.turns[requestID] = result
	f.mu.Unlock()
	// Configuration removal must not retarget retained work or make history unreadable.
	doc, _ := service.Config(t.Context())
	if _, err = service.SaveConfig(t.Context(), environments.ConfigEdit{Revision: doc.Revision, Text: `{"version":1,"machines":[],"environments":[]}`}); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(t.Context(), filepath.Join(project, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) { t.Fatal("restart used local adapter"); return nil, nil }, "/local/gimbal", "/instance", project, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	restarted.SetEnvironments(service, "project")
	done := waitRequest(t, restarted, item.ID, requestID, "completed")
	if done.ActiveRequest != "" || len(done.Messages) != 2 || done.Messages[1].Text != "remote reply" || done.Contact != "connected" {
		t.Fatalf("reconciliation=%+v", done)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sessions != 1 || f.admissions != 1 || f.interruptions != 0 {
		t.Fatalf("replayed or canceled: %+v", f)
	}
	disk, err := os.ReadFile(filepath.Join(project, ".gimbal", "conversations", item.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(disk), requestID) || !strings.Contains(string(disk), `"transport"`) {
		t.Fatal("original request identity not persisted")
	}
	wire, _ := json.Marshal(done)
	if strings.Contains(string(wire), `"transport"`) || strings.Contains(string(wire), "/tasks/") {
		t.Fatal("private transport leaked into live conversation DTO")
	}
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
	defer service.Close()
	if err = service.RegisterProject("project", project); err != nil {
		t.Fatal(err)
	}
	adapter := &chatAdapter{sessions: map[string][]string{}}
	manager, err := New(t.Context(), filepath.Join(project, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) { return adapter, nil }, "/local/gimbal", instance, project, nil)
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
	if len(adapter.workdirs) != 1 || adapter.workdirs[0] != alternate {
		t.Fatalf("local harness dirs=%v", adapter.workdirs)
	}
}
