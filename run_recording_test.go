package gimbal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/live"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/internal/runstate"
	"github.com/tylergannon/gimbal/runstore"
	"github.com/tylergannon/gimbal/runstore/directory"
	"golang.org/x/sys/unix"
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
		g.Go("check", func(ctx context.Context) error {
			_, _, _, e := RunCommand(ctx, "echo", project, "printf", "ok")
			return e
		})
		g.Go("check", func(ctx context.Context) error {
			_, _, _, e := RunCommand(ctx, "echo", project, "printf", "ok")
			return e
		})
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

func TestRetainedCleanupOwnsStoreAndAppendsAfterImmutableOutcome(t *testing.T) {
	project := t.TempDir()
	id := runstore.NewRunID()
	registry := observation.NewRegistry(project)
	var owner *run
	var closes atomic.Int32
	adapter := &fake{answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
		return "ok", nil
	}, closeErr: func(string) error {
		if closes.Add(1) == 1 {
			return errors.New("initial close failure")
		}
		return nil
	}}
	ctx := live.WithHook(observation.WithRegistry(Project(t.Context(), project), registry), func(_ string, c live.Controller) func() { owner = c.(*run); return func() {} })
	err := Run(ctx, string(id), "retained", bind(adapter, "test", "agent"), func(ctx context.Context) error {
		_, e := NewSession(ctx, "agent", project).Generate[Text](ctx, "work")
		return e
	})
	if err == nil || owner == nil {
		t.Fatalf("%v %v", owner, err)
	}
	factory := directory.Store{Root: filepath.Join(project, "runs")}
	if _, e := factory.Open(t.Context(), id); !errors.Is(e, runstore.ErrBusy) {
		t.Fatalf("retained owner lost writer: %v", e)
	}
	if e := owner.retryCleanup(); e != nil {
		t.Fatal(e)
	}
	r, e := factory.Open(t.Context(), id)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	summary, e := runstate.Inspect(t.Context(), r)
	if e != nil {
		t.Fatal(e)
	}
	if summary.Cleanup[summary.Header.Root].Status != "released" {
		t.Fatal(summary.Cleanup)
	}
	snapshot, e := registry.Snapshot(string(id))
	if e != nil || (!snapshot.Run.CleanupStatus.Present || snapshot.Run.CleanupStatus.Value != "released") {
		t.Fatalf("stale live registry cleanup: %+v %v", snapshot.Run, e)
	}
	failed, succeeded := 0, 0
	var cursor uint64
	for {
		records, e := r.Read(t.Context(), cursor, 128)
		if e != nil {
			t.Fatal(e)
		}
		if len(records) == 0 {
			break
		}
		for _, rec := range records {
			cursor = rec.Sequence
			if rec.Kind != "outcome" {
				continue
			}
			var v struct {
				Error string `json:"error"`
			}
			if e = json.Unmarshal(rec.Payload, &v); e != nil {
				t.Fatal(e)
			}
			if v.Error != "" {
				failed++
			} else {
				succeeded++
			}
		}
	}
	if failed != 1 || succeeded < 3 {
		t.Fatalf("earlier outcome overwritten: failed=%d succeeded=%d", failed, succeeded)
	}
}
func TestNewFormatObservationReadsDoNotInferDeathOrMutate(t *testing.T) {
	project := t.TempDir()
	id := string(runstore.NewRunID())
	ctx := Project(t.Context(), project)
	err := Run(ctx, id, "read-only", nil, func(context.Context) error {
		dir := filepath.Join(project, "runs", id)
		lock, e := os.OpenFile(filepath.Join(dir, "runner.lock"), os.O_CREATE|os.O_RDWR, 0600)
		if e != nil {
			return e
		}
		defer lock.Close()
		path := filepath.Join(dir, "state", "00000000.segment")
		before, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		snapshot, e := observation.NewRegistry(project).Snapshot(id)
		if e != nil {
			return e
		}
		if snapshot.Run.Status != observation.StatusRunning {
			t.Fatalf("reader inferred terminal state: %+v", snapshot.Run)
		}
		if e = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); e != nil {
			t.Fatal("reader acquired runner lifetime lock", e)
		}
		after, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		if !bytes.Equal(before, after) {
			t.Fatal("read appended authoritative state")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSavedFactsRebuildTheSameObservationAfterCorrectionAndFailedCommand(t *testing.T) {
	project := t.TempDir()
	id := string(runstore.NewRunID())
	registry := observation.NewRegistry(project)
	calls := 0
	adapter := &fake{report: map[string]Usage{"test": {Tokens: Tokens{Input: 11, Output: 3}}}, answer: func(_ context.Context, _, prompt string, _ json.RawMessage, emit func(AgentEvent) error) (string, error) {
		calls++
		if err := emit(AgentEvent{Type: "session.step.started", NativeRef: json.RawMessage(fmt.Sprintf(`{"provider":"fixture","sessionID":"native-1","messageID":"message-%d"}`, calls)), Data: json.RawMessage(fmt.Sprintf(`{"sessionID":"native-1","assistantMessageID":"message-%d","model":{"id":"test","providerID":"fixture"}}`, calls))}); err != nil {
			return "", err
		}
		if err := emit(AgentEvent{Type: "session.step.ended", NativeRef: json.RawMessage(fmt.Sprintf(`{"provider":"fixture","sessionID":"native-1","messageID":"message-%d"}`, calls)), Data: json.RawMessage(fmt.Sprintf(`{"sessionID":"native-1","assistantMessageID":"message-%d","finish":"stop","cost":0,"tokens":{"input":11,"output":3,"reasoning":0,"cache":{"read":0,"write":0}}}`, calls))}); err != nil {
			return "", err
		}
		if calls == 1 {
			return `{"n":"bad"}`, nil
		}
		if !strings.Contains(prompt, "previous answer was invalid") {
			t.Error("missing correction")
		}
		return `{"n":2}`, nil
	}}
	ctx := observation.WithRegistry(Project(t.Context(), project), registry)
	if err := Run(ctx, id, "reducer", bind(adapter, "test", "agent"), func(ctx context.Context) error {
		code, _, _, err := RunCommand(ctx, "failed", project, "sh", "-c", "printf failure >&2; exit 3")
		if err != nil {
			return err
		}
		if code != 3 {
			t.Fatalf("exit=%d", code)
		}
		_, err = NewSession(ctx, "agent", project).Generate[schemaOnlyAnswer](ctx, "answer")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	original, err := registry.Snapshot(id)
	if err != nil {
		t.Fatal(err)
	}
	r, err := (directory.Store{Root: filepath.Join(project, "runs")}).Open(t.Context(), runstore.RunID(id))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	rebuilt, err := observation.Open(nil, id, "reducer", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer rebuilt.Close()
	var cursor uint64
	agents := 0
	for {
		records, err := r.Read(t.Context(), cursor, 128)
		if err != nil {
			t.Fatal(err)
		}
		if len(records) == 0 {
			break
		}
		for _, rec := range records {
			cursor = rec.Sequence
			raw, err := runstate.Payload(t.Context(), r, rec.Payload)
			if err != nil {
				t.Fatal(err)
			}
			switch rec.Kind {
			case "lifecycle":
				if err = rebuilt.Lifecycle(raw); err != nil {
					t.Fatal(err)
				}
			case "agent":
				var v AgentRecord
				if err = json.Unmarshal(raw, &v); err != nil {
					t.Fatal(err)
				}
				envelope, _ := json.Marshal(v.Event)
				if err = rebuilt.Event(observation.Placement{Scope: v.Scope, Session: v.Session, Turn: v.Turn}, envelope, v.NativeRef); err != nil {
					t.Fatal(err)
				}
				agents++
			case "cleanup-state":
				var v runstate.CleanupState
				if err = json.Unmarshal(raw, &v); err != nil {
					t.Fatal(err)
				}
				if err = rebuilt.CleanupStatus(v.Status, v.Error); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	got := rebuilt.Snapshot()
	original.Stream = ""
	got.Stream = ""
	original.Position = 0
	got.Position = 0
	if calls != 2 || agents < 4 || len(got.Turns) != 2 || len(got.Commands) != 1 || len(got.TurnUsage) != 2 {
		t.Fatalf("scenario missing: calls=%d agents=%d turns=%d usage=%d", calls, agents, len(got.Turns), len(got.TurnUsage))
	}
	a, _ := json.Marshal(original)
	b, _ := json.Marshal(got)
	if !bytes.Equal(a, b) {
		t.Fatalf("saved facts changed projection:\n%s\n%s", a, b)
	}
}

func TestCommandOutputSurvivesSpoolRemovalInFreshProcess(t *testing.T) {
	if os.Getenv("GIMBAL_OUTPUT_READBACK_CHILD") == "1" {
		project, id := os.Getenv("GIMBAL_OUTPUT_READBACK_ROOT"), os.Getenv("GIMBAL_OUTPUT_READBACK_ID")
		r, err := (directory.Store{Root: filepath.Join(project, "runs")}).Open(t.Context(), runstore.RunID(id))
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		var cursor uint64
		found := false
		for {
			records, err := r.Read(t.Context(), cursor, 128)
			if err != nil {
				t.Fatal(err)
			}
			if len(records) == 0 {
				break
			}
			for _, rec := range records {
				cursor = rec.Sequence
				if rec.Kind != "outcome" {
					continue
				}
				raw, err := runstate.Payload(t.Context(), r, rec.Payload)
				if err != nil {
					t.Fatal(err)
				}
				var out struct {
					Result struct {
						ExitCode int                         `json:"exit_code"`
						Output   map[string]runstore.BlobRef `json:"output"`
					} `json:"result"`
				}
				if err = json.Unmarshal(raw, &out); err != nil {
					t.Fatal(err)
				}
				if out.Result.Output == nil {
					continue
				}
				found = true
				for name, ref := range out.Result.Output {
					reader, err := r.OpenBlob(t.Context(), ref)
					if err != nil {
						t.Fatal(err)
					}
					data, err := io.ReadAll(reader)
					err = errors.Join(err, reader.Close())
					if err != nil {
						t.Fatal(err)
					}
					want := []byte("err")
					if name == "stdout" {
						want = bytes.Repeat([]byte("x"), 1<<20)
					}
					if !bytes.Equal(data, want) {
						t.Fatalf("%s output changed: %d bytes", name, len(data))
					}
				}
			}
		}
		if !found {
			t.Fatal("no complete command result")
		}
		return
	}
	project := t.TempDir()
	id := string(runstore.NewRunID())
	if err := Run(Project(t.Context(), project), id, "large-output", nil, func(ctx context.Context) error {
		code, out, _, err := RunCommand(ctx, "large", project, "sh", "-c", `head -c 1048576 /dev/zero | tr '\000' x; printf err >&2`)
		if code != 0 || len(out) >= 1<<20 {
			t.Fatalf("output was not bounded: code=%d length=%d", code, len(out))
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(project, "runs", id, "executor-spool")); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.CommandContext(t.Context(), executable, "-test.run=^TestCommandOutputSurvivesSpoolRemovalInFreshProcess$")
	child.Env = append(os.Environ(), "GIMBAL_OUTPUT_READBACK_CHILD=1", "GIMBAL_OUTPUT_READBACK_ROOT="+project, "GIMBAL_OUTPUT_READBACK_ID="+id)
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("fresh process readback: %v\n%s", err, output)
	}
}
