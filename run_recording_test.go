package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/runstore"
	"github.com/tylergannon/gimbal/runstore/directory"
)

func TestRunRequiresExplicitIdentityAndExcludesDuplicate(t *testing.T) {
	project := t.TempDir()
	registry := observation.NewRegistry(project)
	ctx := observation.WithRegistry(Project(t.Context(), project), registry)
	var bodies atomic.Int32
	for _, id := range []string{"", "legacy.name", "../run"} {
		if e := Run(ctx, id, "test", nil, func(context.Context) error { bodies.Add(1); return nil }); e == nil {
			t.Fatalf("accepted %q", id)
		}
	}
	id := string(runstore.NewRunID())
	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, id, "test", nil, func(context.Context) error { bodies.Add(1); close(started); <-release; return nil })
	}()
	<-started
	if e := Run(ctx, id, "test", nil, func(context.Context) error { bodies.Add(1); return nil }); e == nil {
		t.Error("duplicate accepted")
	}
	if _, ok := registry.Live(id); !ok {
		t.Error("duplicate displaced live owner")
	}
	close(release)
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	if bodies.Load() != 1 {
		t.Fatal("body executions", bodies.Load())
	}
	if e := Run(ctx, id, "test", nil, func(context.Context) error { bodies.Add(1); return nil }); e == nil {
		t.Error("completed run restarted")
	}
}

func TestIdentityTreeBehindOrdinaryWorkflow(t *testing.T) {
	project := t.TempDir()
	id := string(runstore.NewRunID())
	adapter := &fake{answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
		return "ok", nil
	}}
	body := func(ctx context.Context) error {
		s := NewSession(ctx, "coder", project)
		for range 2 {
			if _, e := s.Generate[Text](ctx, "answer briefly"); e != nil {
				return e
			}
		}
		g := Group(ctx, "branches")
		for range 2 {
			g.Go("check", func(ctx context.Context) error {
				_, _, _, e := RunCommand(ctx, "echo", project, "printf", "ok")
				return e
			})
		}
		return g.Wait()
	}
	if e := Run(Project(t.Context(), project), id, "identity", bind(adapter, "test", "coder"), body); e != nil {
		t.Fatal(e)
	}
	store := directory.Store{Root: filepath.Join(project, "runs")}
	r, e := store.Open(t.Context(), runstore.RunID(id))
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	records, e := r.Read(t.Context(), 0, 100)
	if e != nil {
		t.Fatal(e)
	}
	nodes := map[runstore.NodeID]runstore.Node{}
	counts := map[runstore.Kind]int{}
	for _, rec := range records {
		if !runstore.ValidID(string(rec.ID)) {
			t.Fatal("invalid record identity")
		}
		if rec.Kind == "node-created" {
			var n runstore.Node
			if e = json.Unmarshal(rec.Payload, &n); e != nil {
				t.Fatal(e)
			}
			if !runstore.ValidID(string(n.ID)) {
				t.Fatal("invalid node identity")
			}
			nodes[n.ID] = n
			counts[n.Kind]++
		}
	}
	if counts[runstore.Scope] != 4 || counts[runstore.Session] != 1 || counts[runstore.Action] != 5 {
		t.Fatal(counts)
	}
	for _, n := range nodes {
		if n.Parent != "" {
			if _, ok := nodes[n.Parent]; !ok {
				t.Fatal("missing parent", n)
			}
		}
		if n.Name == "turn" && nodes[n.Parent].Kind != runstore.Session {
			t.Fatal("turn not owned by session")
		}
		same, e := r.EnsureNode(t.Context(), n.NodeSpec)
		if e != nil || same.ID != n.ID {
			t.Fatalf("identity changed: %v %v", same, e)
		}
	}
}

type failingCreateStore struct{ err error }

func (s failingCreateStore) Create(context.Context, runstore.Header) (runstore.Run, error) {
	return nil, s.err
}
func (s failingCreateStore) Open(context.Context, runstore.RunID) (runstore.Run, error) {
	return nil, s.err
}
func TestFailedInitializationDoesNotExecuteOrReserveLiveRun(t *testing.T) {
	project := t.TempDir()
	registry := observation.NewRegistry(project)
	injected := errors.New("storage unavailable")
	ctx := WithRunStore(observation.WithRegistry(Project(t.Context(), project), registry), failingCreateStore{injected})
	id := string(runstore.NewRunID())
	if e := Run(ctx, id, "test", nil, func(context.Context) error { t.Fatal("body executed"); return nil }); !errors.Is(e, injected) {
		t.Fatal(e)
	}
	if _, ok := registry.Live(id); ok {
		t.Fatal("failed initialization remained live")
	}
}

type faultStore struct {
	runstore.Store
	failKind string
	failNode bool
}
type faultRun struct {
	runstore.Run
	owner faultStore
}

func (s faultStore) Create(ctx context.Context, h runstore.Header) (runstore.Run, error) {
	r, e := s.Store.Create(ctx, h)
	if e != nil {
		return nil, e
	}
	return faultRun{r, s}, nil
}
func (r faultRun) EnsureNode(ctx context.Context, n runstore.NodeSpec) (runstore.Node, error) {
	if r.owner.failNode && n.Kind == runstore.Action {
		return runstore.Node{}, errors.New("injected node failure")
	}
	return r.Run.EnsureNode(ctx, n)
}
func (r faultRun) Append(ctx context.Context, f []runstore.Fact, d runstore.Durability) ([]runstore.Record, error) {
	for _, fact := range f {
		if fact.Kind == r.owner.failKind {
			return nil, errors.New("injected " + fact.Kind + " failure")
		}
	}
	return r.Run.Append(ctx, f, d)
}
func TestPersistenceFailurePreventsDispatchAndDoesNotRepeatCompletedWork(t *testing.T) {
	for _, failure := range []string{"node", "intent", "outcome"} {
		t.Run(failure, func(t *testing.T) {
			project := t.TempDir()
			marker := filepath.Join(project, "submissions")
			s := faultStore{Store: directory.Store{Root: filepath.Join(project, "runs")}, failKind: failure, failNode: failure == "node"}
			ctx := WithRunStore(Project(t.Context(), project), s)
			err := Run(ctx, string(runstore.NewRunID()), "fault", nil, func(ctx context.Context) error {
				_, _, _, e := RunCommand(ctx, "mark", project, "sh", "-c", "printf x >> submissions")
				return e
			})
			if err == nil {
				t.Fatal("failure hidden")
			}
			data, e := os.ReadFile(marker)
			if failure == "outcome" {
				if e != nil || string(data) != "x" {
					t.Fatalf("completed command was lost/repeated: %q %v", data, e)
				}
			} else if !errors.Is(e, os.ErrNotExist) {
				t.Fatalf("work started after persistence failure: %q %v", data, e)
			}
		})
	}
}
func TestOldDaemonRejectsIdentifiedWorkWithoutPollingOrFallback(t *testing.T) {
	var starts, polls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tasks/process":
			starts.Add(1)
		case "/tasks/result":
			polls.Add(1)
		case "/tasks/cleanup":
			_ = json.NewEncoder(w).Encode(backend.Reply{})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	project := t.TempDir()
	ctx := effects.WithEndpoint(Project(t.Context(), project), server.URL)
	err := Run(ctx, string(runstore.NewRunID()), "old-daemon", nil, func(ctx context.Context) error { _, _, _, e := RunCommand(ctx, "command", project, "true"); return e })
	var rejected *backend.ContractRejected
	if !errors.As(err, &rejected) || starts.Load() != 0 || polls.Load() != 0 {
		t.Fatalf("rejected=%v starts=%d polls=%d", err, starts.Load(), polls.Load())
	}
}

func TestUnidentifiedBackendRejectedBeforeConnection(t *testing.T) {
	registry := &Registry{}
	var connected atomic.Int32
	err := registry.RegisterBackend(BackendDefinition{Name: "old", ConfigurationSchema: json.RawMessage(`{"type":"object"}`), ValidateConfiguration: func(json.RawMessage) error { return nil }, Connect: func(context.Context, json.RawMessage) (backend.Connection, error) {
		connected.Add(1)
		return backend.Connection{}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = registry.Connect(t.Context(), backend.Route{Name: "old"})
	var rejected *backend.ContractRejected
	if !errors.As(err, &rejected) || connected.Load() != 0 {
		t.Fatalf("unidentified backend connected: %v %d", err, connected.Load())
	}
}
