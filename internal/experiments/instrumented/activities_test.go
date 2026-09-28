package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	a := &Activities{project: project, workdir: dir}
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(a)
	if _, err := env.ExecuteActivity(a.Initialize, Data{Task: "cancel group"}); err != nil {
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
