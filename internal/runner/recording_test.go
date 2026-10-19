package runner

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
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/internal/runstate"
	"github.com/tylergannon/gimbal/runstore"
	"github.com/tylergannon/gimbal/runstore/directory"
)

type countedStore struct {
	runstore.Store
	opens *atomic.Int32
}

func (s countedStore) Open(ctx context.Context, id runstore.RunID) (runstore.Run, error) {
	s.opens.Add(1)
	return s.Store.Open(ctx, id)
}
func orphanFixture(t *testing.T, endpoint string) (string, string, runstore.Store, runstore.Run) {
	t.Helper()
	state := t.TempDir()
	id := runstore.NewRunID()
	factory := directory.Store{Root: filepath.Join(state, "runs")}
	r, e := factory.Create(t.Context(), runstore.Header{RunID: id, Workflow: "orphan"})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = r.Close() })
	h, e := r.Header(t.Context())
	if e != nil {
		t.Fatal(e)
	}
	dir := filepath.Join(state, "runs", string(id))
	s, e := observation.Open(nil, string(id), "orphan", dir)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(map[string]any{"time": time.Now().UTC(), "scope": "", "event": map[string]string{"kind": "run_started", "name": "orphan"}})
	if e = runstate.Append(t.Context(), r, h.Root, h.Root, "lifecycle", json.RawMessage(raw)); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "run.jsonl"), append(append([]byte(nil), raw...), '\n'), 0600); e != nil {
		t.Fatal(e)
	}
	if e = s.Lifecycle(raw); e != nil {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "runner.lock"), nil, 0600); e != nil {
		t.Fatal(e)
	}
	if endpoint != "" {
		if e = os.WriteFile(filepath.Join(dir, "task-owner"), []byte(endpoint), 0600); e != nil {
			t.Fatal(e)
		}
	}
	return state, string(id), factory, r
}
func recordCount(t *testing.T, f runstore.Store, id string) int {
	t.Helper()
	r, e := f.Open(t.Context(), runstore.RunID(id))
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	count := 0
	var cursor uint64
	for {
		records, e := r.Read(t.Context(), cursor, 128)
		if e != nil {
			t.Fatal(e)
		}
		if len(records) == 0 {
			return count
		}
		count += len(records)
		cursor = records[len(records)-1].Sequence
	}
}
func TestNewCleanupOwnerExcludesLiveWriterAndDispatchesOnce(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tasks/cleanup" {
			calls.Add(1)
		}
		_ = json.NewEncoder(w).Encode(backend.Reply{})
	}))
	defer server.Close()
	state, id, f, owner := orphanFixture(t, server.URL)
	if settled, e := CleanupExited(state, id); settled || e != nil || calls.Load() != 0 {
		t.Fatalf("live owner bypassed: %v %v %d", settled, e, calls.Load())
	}
	if e := owner.Close(); e != nil {
		t.Fatal(e)
	}
	c := remote{state: state, runID: id}
	if _, e := c.call(t.Context(), operation{Action: "cleanup"}); e != nil {
		t.Fatal(e)
	}
	if calls.Load() != 0 {
		t.Fatal("status dispatched cleanup")
	}
	if _, e := c.call(t.Context(), operation{Action: "scope"}); e != nil {
		t.Fatal(e)
	}
	if calls.Load() != 1 {
		t.Fatalf("operator stop dispatched %d times", calls.Load())
	}
	var opens atomic.Int32
	if settled, e := cleanupStopped(state, id, nil, countedStore{f, &opens}); !settled || e != nil || opens.Load() != 0 {
		t.Fatalf("settled run reopened: %v %v opens=%d", settled, e, opens.Load())
	}
}
func TestSweeperRetainsOneEpisodeAndSuppressesUnchangedFacts(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks/cleanup" {
			http.NotFound(w, r)
			return
		}
		calls.Add(1)
		reply := backend.Reply{}
		if fail.Load() {
			reply.Error = "still stopping"
		}
		_ = json.NewEncoder(w).Encode(reply)
	}))
	defer server.Close()
	state, id, f, owner := orphanFixture(t, server.URL)
	if e := owner.Close(); e != nil {
		t.Fatal(e)
	}
	if settled, e := CleanupExited(state, id); settled || e == nil {
		t.Fatalf("failed cleanup lost: %v %v", settled, e)
	}
	before := recordCount(t, f, id)
	if settled, e := CleanupExited(state, id); settled || e == nil {
		t.Fatalf("failed cleanup changed: %v %v", settled, e)
	}
	if after := recordCount(t, f, id); after != before {
		t.Fatalf("unchanged poll appended facts: %d -> %d", before, after)
	}
	fail.Store(false)
	if settled, e := CleanupExited(state, id); !settled || e != nil {
		t.Fatalf("successful cleanup: %v %v", settled, e)
	}
	r, e := f.Open(t.Context(), runstore.RunID(id))
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	summary, e := runstate.Inspect(t.Context(), r)
	if e != nil {
		t.Fatal(e)
	}
	if !summary.Ended || summary.Cleanup[summary.Header.Root].Status != "released" || len(summary.Outcomes) != 1 {
		t.Fatalf("inconsistent terminal history: %+v", summary)
	}
	if calls.Load() != 3 {
		t.Fatal("native dispatch count", calls.Load())
	}
}
func TestOrphanWithoutRunnerLockDoesNotInferDeath(t *testing.T) {
	state, id, f, owner := orphanFixture(t, "")
	if e := owner.Close(); e != nil {
		t.Fatal(e)
	}
	before := recordCount(t, f, id)
	if e := os.Remove(filepath.Join(state, "runs", id, "runner.lock")); e != nil {
		t.Fatal(e)
	}
	if settled, e := CleanupExited(state, id); !settled || e != nil {
		t.Fatalf("non-runner entry: %v %v", settled, e)
	}
	if after := recordCount(t, f, id); after != before {
		t.Fatalf("in-process death inferred: %d -> %d", before, after)
	}
	if settled, err := cleanupStopped(state, id, &operation{Action: "scope"}, f); settled || err == nil {
		t.Fatalf("explicit stop reported success without runner: %v %v", settled, err)
	}
}
func TestIncompleteStoreDoesNotAcquireLivenessOrDispatch(t *testing.T) {
	state := t.TempDir()
	id := string(runstore.NewRunID())
	dir := filepath.Join(state, "runs", id, "state")
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	if settled, e := CleanupExited(state, id); settled || e != nil {
		t.Fatalf("incomplete run settled: %v %v", settled, e)
	}
	if _, e := os.Stat(filepath.Join(filepath.Dir(dir), "runner.lock")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("liveness lock was created", e)
	}
}

// A background cleanup episode retains its request identity across owner opens.
type episodeMessages struct {
	requests []backend.MessageRequest
	fail     bool
}

func (m *episodeMessages) PublishRequest(_ context.Context, r backend.MessageRequest) error {
	m.requests = append(m.requests, r)
	return nil
}
func (*episodeMessages) PublishEvent(context.Context, backend.MessageEvent) error { return nil }
func (m *episodeMessages) ListenEvents(_ context.Context, s backend.MessageSelection, _ time.Time, fn func(backend.MessageEvent) error) error {
	r := m.requests[len(m.requests)-1]
	result := &backend.MessageResult{}
	if m.fail {
		result.Unresolved = true
		result.Error = "still stopping"
	}
	return fn(backend.MessageEvent{ID: string(runstore.NewRecordID()), RequestID: r.ID, Owner: r.Owner, Created: time.Now(), Kind: "completed", Result: result})
}
func TestOrphanCleanupRecordsTheIdentityAdmittedByMessaging(t *testing.T) {
	bus := &episodeMessages{fail: true}
	name := "orphan-identity-test"
	if err := gimbal.DefaultRegistry.RegisterBackend(gimbal.BackendDefinition{ExecutionIdentity: true, Name: name, ConfigurationSchema: json.RawMessage(`{}`), ValidateConfiguration: func(json.RawMessage) error { return nil }, Connect: func(context.Context, json.RawMessage) (backend.Connection, error) {
		return backend.Connection{Messages: bus}, nil
	}}); err != nil {
		t.Fatal(err)
	}
	state, id, f, owner := orphanFixture(t, "")
	route, _ := json.Marshal(backend.Route{Name: name})
	if err := os.WriteFile(filepath.Join(state, "runs", id, "task-route"), route, 0600); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if settled, err := cleanupStopped(state, id, nil, f); settled || err == nil {
			t.Fatalf("lost failed cleanup: %v %v", settled, err)
		}
	}
	bus.fail = false
	if settled, err := cleanupStopped(state, id, nil, f); !settled || err != nil {
		t.Fatalf("cleanup: %v %v", settled, err)
	}
	r, err := f.Open(t.Context(), runstore.RunID(id))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	records, err := r.Read(t.Context(), 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	var recorded backend.ExecutionIdentity
	for _, rec := range records {
		if rec.Kind == "intent" {
			var intent struct {
				Route backend.Route `json:"route"`
			}
			if err = json.Unmarshal(rec.Payload, &intent); err != nil {
				t.Fatal(err)
			}
			if intent.Route.Name != name {
				t.Fatalf("wrong owner route: %+v", intent.Route)
			}
		}
		if rec.Kind == "binding" {
			if err = json.Unmarshal(rec.Payload, &recorded); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(bus.requests) != 3 || recorded.RequestID == "" || recorded.ActionID != recorded.RequestID {
		t.Fatalf("missing admissions/binding: %d %+v", len(bus.requests), recorded)
	}
	for _, request := range bus.requests {
		if request.ID != recorded.RequestID || request.Run != id {
			t.Fatalf("unrecorded admission: %+v binding %+v", request, recorded)
		}
	}
}
