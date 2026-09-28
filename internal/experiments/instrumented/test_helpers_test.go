package main

import (
	"context"
	"testing"

	"github.com/tylergannon/gimbal/internal/compiledscope"
	"github.com/tylergannon/gimbal/internal/host"
	"go.temporal.io/sdk/testsuite"
)

func newTestActivities(t *testing.T) *Activities {
	t.Helper()
	t.Setenv("TYPESAFE_API_KEY", "test-key")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	owner := host.New(ctx, t.TempDir())
	t.Cleanup(owner.Close)
	dir := t.TempDir()
	project, err := owner.AdmitProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := &Activities{project: project, workdir: dir, store: compiledscope.Store{Root: t.TempDir()}}

	return a
}
func testActivities(t *testing.T, name string) *Activities {
	a := newTestActivities(t)
	var s testsuite.WorkflowTestSuite
	e := s.NewTestActivityEnvironment()
	e.RegisterActivity(a)
	if _, err := e.ExecuteActivity(a.Initialize, Data{Name: name}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := a.Finish(t.Context(), ""); err != nil {
			t.Error(err)
		}
	})
	return a
}
