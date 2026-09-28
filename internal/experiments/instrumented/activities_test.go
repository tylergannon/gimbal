package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/compiledscope"
	"github.com/tylergannon/gimbal/internal/host"
	"go.temporal.io/sdk/testsuite"
)

func TestRootCancellationClosesExplicitScopesExactlyOnce(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "test-key")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	owner := host.New(ctx, t.TempDir())
	defer owner.Close()
	dir := t.TempDir()
	project, err := owner.AdmitProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := &Activities{project: project, workdir: dir, store: compiledscope.Store{Root: t.TempDir()}}
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(a)
	input, err := a.store.Extend("", contextEntry("task", "cancel group"))
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

func TestLargeChecksAreStoredAtProducingBoundary(t *testing.T) {
	a := Activities{store: compiledscope.Store{Root: t.TempDir()}}
	stdout := strings.Repeat("test output\n", 200000)
	out, err := a.checked(0, stdout, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) > 200 {
		t.Fatal("large output escaped into activity result")
	}
	entries, err := a.store.Load(out.Context)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := a.store.Value(entries[0])
	if err != nil {
		t.Fatal(err)
	}
	var restored Checks
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Stdout != stdout {
		t.Fatal("stored checks lost output")
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
	entries, err := store.Load(input.Context)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.Value(entries[0])
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
