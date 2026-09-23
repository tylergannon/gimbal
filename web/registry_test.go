package web

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/runlog"
)

// blocking is a HarnessAdapter whose "wait" turns block until their ctx
// ends and whose other turns answer "ok" at once. It remembers the steers
// that reached a running turn.
type blocking struct {
	mu      sync.Mutex
	made    int
	started int // turns that entered RunTurn
	running int // turns inside RunTurn now
	steers  []string
}

func (b *blocking) CreateSession(ctx context.Context, model, effort, workdir string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.made++
	return "native-" + string(rune('0'+b.made)), nil
}

func (b *blocking) RunTurn(ctx context.Context, session, prompt string, schema json.RawMessage, onEvent func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	b.mu.Lock()
	b.started++
	b.running++
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.running--
		b.mu.Unlock()
	}()
	if prompt == "wait" {
		<-ctx.Done()
		return gimble.TurnResult{}, ctx.Err()
	}
	out, err := json.Marshal("ok")
	return gimble.TurnResult{Output: out}, err
}

func (b *blocking) Steer(ctx context.Context, session, message string) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.running > 0 {
		b.steers = append(b.steers, message)
		return true, nil
	}
	return false, nil
}

func (b *blocking) Fork(ctx context.Context, session string) (string, error) {
	return session + "-fork", nil
}

func (b *blocking) Close(ctx context.Context, session string) error { return nil }

// startedTurns waits until n turns have entered the fake's RunTurn.
func startedTurns(t *testing.T, b *blocking, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		b.mu.Lock()
		started := b.started
		b.mu.Unlock()
		if started == n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d turns started, want %d", started, n)
		}
		time.Sleep(time.Millisecond)
	}
}

// runID is the id of the one run under the project's runs directory, the
// same id Run produced and the page shows.
func runID(t *testing.T, project string) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(project, ".gimble", "runs"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("runs = %v, %v", entries, err)
	}
	return entries[0].Name()
}

// TestRuntimeReachesALiveRunByID is the #176 check: a run under NewRuntime
// is steered, has a turn killed, and has a scope killed from another
// goroutine holding only ids; the run log carries the Steer with Source
// person and the two Killed records; unknown and finished ids are errors.
func TestRuntimeReachesALiveRunByID(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	project := t.TempDir()
	_, runtime, err := newProject(ctx, project, WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	b := &blocking{}
	const (
		session = "lap.1/coder.1"
		turn    = "lap.1/coder.1/turn.1"
		scope   = "lap.1"
	)
	var first, second, scopeErr error
	var runErr error
	var runWG sync.WaitGroup
	runWG.Go(func() {
		runErr = runtime.Run(ctx, "registry", map[gimble.WorkflowRole]gimble.ModelBinding{"coder": {Adapter: b, Model: "m"}}, func(ctx context.Context) error {
			scopeErr = gimble.Scope(ctx, "lap", func(ctx context.Context) error {
				coder := gimble.NewSession(ctx, "coder", "/w")
				_, first = coder.Generate[gimble.Text](ctx, "wait")
				_, second = coder.Generate[gimble.Text](ctx, "wait")
				return second
			})
			return nil
		})
	})

	startedTurns(t, b, 1)
	id := runID(t, project)
	if landed, err := runtime.Steer(ctx, id, session, "look at the tests"); err != nil || !landed {
		t.Fatalf("Steer = %v, %v; want landed", landed, err)
	}
	if _, err := runtime.Steer(ctx, id, "lap.1/nobody.1", "hello"); err == nil {
		t.Error("Steer of an unknown session returned nil")
	}
	if _, err := runtime.Steer(ctx, "nope", session, "hello"); err == nil {
		t.Error("Steer of an unknown run returned nil")
	}
	if err := runtime.KillTurn(id, "lap.1/coder.1/turn.9", "tyler", "wrong file"); err == nil {
		t.Error("KillTurn of an unknown turn returned nil")
	}
	if err := runtime.KillScope(id, "lap.9", "tyler", "off the rails"); err == nil {
		t.Error("KillScope of an unknown scope returned nil")
	}
	if err := runtime.KillTurn(id, turn, "tyler", "wrong file"); err != nil {
		t.Fatalf("KillTurn = %v", err)
	}
	startedTurns(t, b, 2) // the second turn on the same session
	if err := runtime.KillScope(id, scope, "tyler", "off the rails"); err != nil {
		t.Fatalf("KillScope = %v", err)
	}
	runWG.Wait()
	if runErr != nil {
		t.Fatalf("Run = %v", runErr)
	}
	if _, err := runtime.Steer(ctx, id, session, "too late"); err == nil {
		t.Error("Steer of a finished run returned nil")
	}

	wantTurn := gimble.Killed{Target: turn, By: "tyler", Reason: "wrong file"}
	wantScope := gimble.Killed{Target: scope, By: "tyler", Reason: "off the rails"}
	var killed gimble.Killed
	if !errors.As(first, &killed) || killed != wantTurn {
		t.Errorf("first Generate = %v, want the Killed cause %v", first, wantTurn)
	}
	if !errors.As(second, &killed) || killed != wantScope {
		t.Errorf("second Generate = %v, want the Killed cause %v", second, wantScope)
	}
	if !errors.As(scopeErr, &killed) || killed != wantScope {
		t.Errorf("Scope = %v, want the Killed cause %v", scopeErr, wantScope)
	}
	b.mu.Lock()
	steers := append([]string(nil), b.steers...)
	b.mu.Unlock()
	if len(steers) != 1 || steers[0] != "look at the tests" {
		t.Errorf("steers that reached the adapter = %q, want the one sent", steers)
	}

	var steered []gimble.LifecycleRecord
	var kills []gimble.LifecycleRecord
	if err := runlog.Read[gimble.LifecycleRecord](ctx, filepath.Join(project, ".gimble", "runs", id), func(record gimble.LifecycleRecord) error {
		switch record.Event.(type) {
		case gimble.Steer:
			steered = append(steered, record)
		case gimble.Killed:
			kills = append(kills, record)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(steered) != 1 {
		t.Fatalf("Steer records = %+v, want one", steered)
	}
	steer := steered[0].Event.(gimble.Steer)
	if steered[0].Scope != scope || steered[0].Session.Value != session || steered[0].Turn.Value != turn || steer.Source != "person" || steer.Target != session || !steer.Landed || steer.Message != "look at the tests" {
		t.Errorf("Steer record = %+v %+v, want Source person on %s", steered[0], steer, turn)
	}
	if len(kills) != 2 {
		t.Fatalf("Killed records = %+v, want two", kills)
	}
	if kills[0].Scope != scope || kills[0].Session.Value != session || kills[0].Turn.Value != turn || kills[0].Event != wantTurn {
		t.Errorf("first Killed record = %+v, want %v placed on the turn", kills[0], wantTurn)
	}
	if kills[1].Scope != scope || kills[1].Session.Value != "" || kills[1].Event != wantScope {
		t.Errorf("second Killed record = %+v, want %v placed on the scope", kills[1], wantScope)
	}
	if !strings.HasSuffix(id, ".registry") {
		t.Errorf("run id = %q, want <ulid>.registry", id)
	}
}
