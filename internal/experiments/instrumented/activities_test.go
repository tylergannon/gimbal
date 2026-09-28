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

func TestRootCancellationJoinsGroupExactlyOnce(t *testing.T) {
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
	if _, err := env.ExecuteActivity(a.BeginIteration, IterationData{Pair: pairs[0]}); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{}, 2)
	a.iteration.group.Go("left", func(ctx context.Context) error { started <- struct{}{}; <-ctx.Done(); return ctx.Err() })
	a.iteration.group.Go("right", func(ctx context.Context) error { started <- struct{}{}; <-ctx.Done(); return ctx.Err() })
	<-started
	<-started
	done := make(chan struct{})
	go func() { defer close(done); _, _ = env.ExecuteActivity(a.IterationTests, IterationData{Pair: pairs[0]}) }()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("group didn't join")
	}
	if _, err := env.ExecuteActivity(a.Finish, "cancelled"); err != nil {
		t.Fatal(err)
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
