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
	"time"

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
	defer func() { _ = service.Close() }()
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

func TestRemoteCleanupCannotSettleOriginalWorkOnRestartedEmptyDaemon(t *testing.T) {
	fixture := &remoteFixture{forgetOnCleanup: true}
	manager, _, _ := remoteManager(t, fixture)
	item, err := manager.Start(t.Context(), NewConversation{Provider: "claude", Environment: "remote-project", SharedCheckout: true}, "Keep ownership through lost daemon state")
	if err != nil {
		t.Fatal(err)
	}
	waitRemoteAdmission(t, fixture)
	err = manager.Cleanup(t.Context(), item.ID)
	if err == nil {
		t.Fatal("empty daemon's cleanup acknowledged original cessation")
	}
	current, _ := manager.Get(item.ID)
	if !current.Live || !current.CleanupPending || current.ActiveRequest != item.Requests[0].ID || current.Status != "unresolved" {
		t.Fatalf("lost ownership: %+v", current)
	}
	for _, row := range current.Requests {
		if row.ID == item.Requests[0].ID && (row.Status == "canceled" || row.Status == "completed") {
			t.Fatalf("invented original settlement: %+v", row)
		}
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.admissions != 1 || fixture.sessions != 1 {
		t.Fatal("cleanup replayed native admission")
	}
}

func TestRemoteCleanupConfirmsInterruptionOrCompletionRace(t *testing.T) {
	for _, completed := range []bool{false, true} {
		t.Run(map[bool]string{false: "interrupted", true: "completed"}[completed], func(t *testing.T) {
			fixture := &remoteFixture{cleanupCompletes: completed}
			manager, _, _ := remoteManager(t, fixture)
			item, err := manager.Start(t.Context(), NewConversation{Provider: "claude", Environment: "remote-project", SharedCheckout: true}, "Observe original outcome")
			if err != nil {
				t.Fatal(err)
			}
			waitRemoteAdmission(t, fixture)
			if _, err = manager.Submit(item.ID, Request{ID: "never-start-during-end", Created: time.Now().UnixMilli(), Kind: "prompt", Message: "Queued before End"}); err != nil {
				t.Fatal(err)
			}
			if err = manager.Cleanup(t.Context(), item.ID); err != nil {
				t.Fatal(err)
			}
			current, _ := manager.Get(item.ID)
			if current.Live || current.CleanupPending || current.ActiveRequest != "" {
				t.Fatalf("cleanup retained settled work: %+v", current)
			}
			want := "canceled"
			if completed {
				want = "completed"
			}
			fixture.mu.Lock()
			admitted := fixture.admissions
			fixture.mu.Unlock()
			if admitted != 1 {
				t.Fatalf("End admitted queued native work: %d", admitted)
			}
			if current.Requests[1].Status != "canceled" {
				t.Fatalf("queued request survived End: %+v", current.Requests[1])
			}
			if current.Requests[0].Status != want {
				t.Fatalf("original=%s want %s", current.Requests[0].Status, want)
			}
			if completed {
				count := 0
				for _, message := range current.Messages {
					if message.Role == "assistant" {
						count++
					}
				}
				if count != 1 {
					t.Fatalf("completion applied %d times", count)
				}
			}
		})
	}
}

func waitRemoteAdmission(t *testing.T, fixture *remoteFixture) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		fixture.mu.Lock()
		started := fixture.admissions > 0
		fixture.mu.Unlock()
		if started {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("remote turn not admitted")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestRetainedConversationRebindsExpiredConnectionForStopAndCleanup(t *testing.T) {
	f := &remoteFixture{}
	m, service, _ := remoteManager(t, f)
	item, err := m.Start(t.Context(), NewConversation{Provider: "claude", Environment: "remote-project", SharedCheckout: true}, "Keep this request identity")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		f.mu.Lock()
		admitted := f.admissions == 1
		f.mu.Unlock()
		if admitted {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("turn not admitted")
		}
		time.Sleep(time.Millisecond)
	}
	// A dead SSH tunnel leaves a cached HTTP binding at its former local port.
	// Keep the durable target intact; only the derived connection has expired.
	stale := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "expired tunnel", http.StatusServiceUnavailable)
	}))
	stale.Close()
	m.mu.RLock()
	target := m.remoteStates[item.ID].Target
	m.mu.RUnlock()
	expired := target
	expired.Machine.Kind = environments.KindAddress
	expired.Machine.Address = stale.URL
	expired.Route = backend.Route{}
	binding, err := service.Acquire(t.Context(), expired, backend.Owner{Run: item.ID, Scope: item.ID, Task: "session"})
	if err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.active[item.ID].remote = &binding
	m.mu.Unlock()
	if _, err = m.Submit(item.ID, Request{ID: "reconnected-stop", Created: time.Now().UnixMilli(), Kind: "interrupt", Target: item.Requests[0].ID}); err != nil {
		t.Fatal(err)
	}
	waitRequest(t, m, item.ID, item.Requests[0].ID, "canceled")
	waitRequest(t, m, item.ID, "reconnected-stop", "completed")
	m.mu.Lock()
	m.active[item.ID].remote = &binding
	m.mu.Unlock()
	budget, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err = m.Cleanup(budget, item.ID); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.admissions != 1 || f.sessions != 1 || f.interruptions != 1 || f.cleanup != 1 {
		t.Fatalf("reconnect replayed or lost original control: %+v", f)
	}
}

func TestCleanupAfterLostSessionAdmissionDoesNotRequireNeverSubmittedTurn(t *testing.T) {
	f := &remoteFixture{sessionReplyLost: true}
	m, _, _ := remoteManager(t, f)
	item, err := m.Start(t.Context(), NewConversation{Provider: "claude", Environment: "remote-project", SharedCheckout: true}, "Session admission reply may be lost")
	if err != nil {
		t.Fatal(err)
	}
	waitRequest(t, m, item.ID, item.Requests[0].ID, "unresolved")
	budget, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err = m.Cleanup(budget, item.ID); err != nil {
		t.Fatal(err)
	}
	closed, _ := m.Get(item.ID)
	if closed.Live || closed.CleanupPending || closed.ActiveRequest != "" || closed.Requests[0].Status != "canceled" {
		t.Fatalf("never-submitted prompt retained forever: %+v", closed)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sessions != 1 || f.admissions != 0 || f.cleanup != 1 {
		t.Fatalf("wrong admission/control: %+v", f)
	}
}

func TestRemoteEffectsRequireIdentityJournal(t *testing.T) {
	for _, operation := range []string{"prompt", "interrupt", "cleanup"} {
		t.Run(operation, func(t *testing.T) {
			f := &remoteFixture{}
			m, _, _ := remoteManager(t, f)
			item, err := m.Create(t.Context(), NewConversation{Provider: "claude", Environment: "remote-project", SharedCheckout: true})
			if err != nil {
				t.Fatal(err)
			}
			r := request("journal-guard", "Persist before delivery")
			if operation == "interrupt" {
				r.Kind = "interrupt"
				r.Target = "original"
			}
			m.mu.Lock()
			active := m.active[item.ID]
			if operation != "cleanup" {
				if err = m.reduce(m.items[item.ID], requestEvent{kind: "accepted", request: r}); err != nil {
					m.mu.Unlock()
					t.Fatal(err)
				}
			}
			m.mu.Unlock()
			// Atomic replacement cannot overwrite a directory: admission above was
			// durable, but the effect's later identity journal must now fail.
			path := filepath.Join(m.projectDir, "conversations", item.ID+".json")
			if err = os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err = os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(path); m.mu.Lock(); _ = m.save(m.items[item.ID]); m.mu.Unlock() })
			switch operation {
			case "prompt":
				m.remoteTurn(item.ID, r, active)
			case "interrupt":
				m.remoteInterrupt(item.ID, r)
			case "cleanup":
				if err = m.Cleanup(t.Context(), item.ID); err == nil {
					t.Fatal("cleanup ignored failed journal")
				}
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.sessions != 0 || f.admissions != 0 || f.interruptions != 0 || f.cleanup != 0 {
				t.Fatalf("effect escaped failed journal: %+v", f)
			}
		})
	}
}

func TestCleanupWaitsForSessionAdmissionAndPreventsFollowingTurn(t *testing.T) {
	f := &remoteFixture{sessionStarted: make(chan struct{}), sessionRelease: make(chan struct{})}
	m, _, _ := remoteManager(t, f)
	item, err := m.Start(t.Context(), NewConversation{Provider: "claude", Environment: "remote-project", SharedCheckout: true}, "End during session admission")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-f.sessionStarted:
	case <-time.After(time.Second):
		t.Fatal("session not admitted")
	}
	done := make(chan error, 1)
	go func() { done <- m.Cleanup(t.Context(), item.ID) }()
	deadline := time.Now().Add(time.Second)
	for {
		current, _ := m.Get(item.ID)
		if current.CleanupPending {
			break
		}
		if time.Now().After(deadline) {
			close(f.sessionRelease)
			t.Fatal("cleanup not pending")
		}
		time.Sleep(time.Millisecond)
	}
	close(f.sessionRelease)
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cleanup did not finish")
	}
	closed, _ := m.Get(item.ID)
	if closed.Live || closed.CleanupPending || closed.ActiveRequest != "" {
		t.Fatalf("cleanup lost admission race: %+v", closed)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sessions != 1 || f.admissions != 0 || f.cleanup != 1 {
		t.Fatalf("turn began after End: %+v", f)
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
	restored, err := New(t.Context(), filepath.Join(project, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) {
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
