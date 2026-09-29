package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/compiledscope"
	"github.com/tylergannon/gimbal/internal/host"
	"go.temporal.io/sdk/testsuite"
)

func TestRootCancellationClosesExplicitScopesExactlyOnce(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "test-key")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	dir := t.TempDir()
	a := &Activities{workdir: dir, store: compiledscope.Store{Root: t.TempDir()}, controls: host.CompiledControls{CancelRun: func(context.Context, gimbal.Killed) error { return nil }}}
	a.store.LocalDir = filepath.Join(a.store.Root, "materialized")
	ctx, err := host.WithContextStore(ctx, dir, &a.store, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	owner := host.New(ctx, t.TempDir())
	defer owner.Close()
	project, err := owner.AdmitProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	a.project = project
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(a)
	input, err := a.store.Extend(t.Context(), "", contextEntry("task", "cancel group"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.ExecuteActivity(a.Initialize, Data{Context: input}); err != nil {
		t.Fatal(err)
	}
	for _, in := range []ScopeInput{
		{"pairs.1", "", "pairs"},
		{"pairs.1/fixes.1", "pairs.1", "fixes"},
		{"pairs.1/fixes.1/left.1", "pairs.1/fixes.1", "left"},
		{"pairs.1/fixes.1/right.1", "pairs.1/fixes.1", "right"},
	} {
		if _, err := env.ExecuteActivity(a.EnterScope, in); err != nil {
			t.Fatal(err)
		}
	}
	left, releaseLeft, err := a.operation(t.Context(), "pairs.1/fixes.1/left.1")
	if err != nil {
		t.Fatal(err)
	}
	right, releaseRight, err := a.operation(t.Context(), "pairs.1/fixes.1/right.1")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	for _, branch := range []context.Context{left, right} {
		select {
		case <-branch.Done():
		case <-time.After(time.Second):
			t.Fatal("branch not cancelled")
		}
	}
	// Closing a branch must wait for its active operation, even after cancellation.
	done := make(chan error, 1)
	go func() { done <- a.ExitScope(t.Context(), "pairs.1/fixes.1/left.1", "cancelled") }()
	select {
	case <-done:
		t.Fatal("scope closed before activity joined")
	case <-time.After(20 * time.Millisecond):
	}
	releaseLeft()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	releaseRight()
	// No controller cleanup is required for local resources to finish.
	select {
	case <-a.drainDone:
	case <-time.After(5 * time.Second):
		t.Fatal("local cancellation did not drain without controller")
	}
	// Late controller cleanup receives the recorded outcome.
	for _, id := range []string{"pairs.1/fixes.1/right.1", "pairs.1/fixes.1", "pairs.1", ""} {
		if err := a.ExitScope(t.Context(), id, "cancelled"); err != nil {
			t.Fatal(err)
		}
	}
	paths, err := filepath.Glob(filepath.Join(project.Dir(), "runs", "*", "run.jsonl"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("runs: %v %v", paths, err)
	}
	file, err := os.Open(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	dec := json.NewDecoder(file)
	ends := map[string]int{}
	for dec.More() {
		var r struct {
			Scope string
			Event struct{ Kind string }
		}
		if err := dec.Decode(&r); err != nil {
			t.Fatal(err)
		}
		if r.Event.Kind == "scope_ended" {
			ends[r.Scope]++
		}
	}
	for _, scope := range []string{"", "pairs.1", "pairs.1/fixes.1", "pairs.1/fixes.1/left.1", "pairs.1/fixes.1/right.1"} {
		if ends[scope] != 1 {
			t.Errorf("%s ended %d times", scope, ends[scope])
		}
	}
}

func TestInitialTaskIsStoredBeforeWorkflowSubmission(t *testing.T) {
	root := t.TempDir()
	task := strings.Repeat("initial task input ", 200000)
	input, err := prepareInput(root, "large-input", task)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) > 100 {
		t.Fatal("task escaped into workflow payload")
	}
	// Use the worker-side path computation, with no live producer objects.
	store := compiledscope.Store{Root: filepath.Join(root, environmentID("large-input"), "context")}
	entries, err := store.Load(t.Context(), input.Context)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.Value(t.Context(), entries[0])
	if err != nil {
		t.Fatal(err)
	}
	var got string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got != task {
		t.Fatal("initial task lost contents")
	}
}

func TestCancellationDrainRetainsCleanupFailureForLateController(t *testing.T) {
	a := newTestActivities(t)
	closeFailure := errors.New("session close unavailable")
	adapter := &specimenAdapter{closeErr: closeFailure, turn: func(context.Context, string, string) (any, error) { return Report{}, nil }}
	a.models = map[gimbal.WorkflowRole]gimbal.ModelBinding{coder: {Adapter: adapter, Model: "deterministic"}}
	registerTestGraph("drain-failure")
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(a)
	if _, err := env.ExecuteActivity(a.Initialize, Data{Name: "drain-failure"}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.ExecuteActivity(a.EnterScope, ScopeInput{"child.1", "", "child"}); err != nil {
		t.Fatal(err)
	}
	encoded, err := env.ExecuteActivity(a.OpenSession, "child.1", "worker", coder, a.workdir)
	if err != nil {
		t.Fatal(err)
	}
	var session sessionHandle
	if err = encoded.Get(&session); err != nil {
		t.Fatal(err)
	}
	if _, err = env.ExecuteActivity(a.ContinuityGenerate1, operationInput{Scope: "child.1", Session: session}); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	root := a.scopes[""].ctx
	a.mu.Unlock()
	if err = compiledscope.CancelRun(root); err != nil {
		t.Fatal(err)
	}
	select {
	case <-a.drainDone:
	case <-time.After(5 * time.Second):
		t.Fatal("drain did not finish")
	}
	for range 2 {
		if err = a.FinishCancelled(t.Context(), "cancelled"); err == nil || !strings.Contains(err.Error(), closeFailure.Error()) {
			t.Fatalf("late finish lost cleanup outcome: %v", err)
		}
	}
	if len(adapter.closed) != 1 {
		t.Fatalf("closed sessions: %v", adapter.closed)
	}
}

func TestInvalidInitialSnapshotDoesNotLeakHostedRun(t *testing.T) {
	a := newTestActivities(t)
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(a)
	if _, err := env.ExecuteActivity(a.Initialize, Data{Name: "continuity", Context: "missing"}); err == nil {
		t.Fatal("missing snapshot accepted")
	}
	// Owner.Close is a registered cleanup: a leaked active-run registration
	// would prevent this test from completing.
}
