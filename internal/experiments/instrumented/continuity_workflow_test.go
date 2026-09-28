package main

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimbal/internal/compiledscope"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

func TestContinuityWorkflowBranchesAndFullResults(t *testing.T) {
	for _, mode := range []string{"edit", "skip", "agent-error"} {
		t.Run(mode, func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			e := suite.NewTestWorkflowEnvironment()
			var mu sync.Mutex
			var trace []string
			record := func(s string) { mu.Lock(); defer mu.Unlock(); trace = append(trace, s) }
			reg := func(name string, fn any) { e.RegisterActivityWithOptions(fn, activity.RegisterOptions{Name: name}) }
			reg("ProvisionEnvironment", func(context.Context) (Environment, error) { return Environment{Queue: "test"}, nil })
			reg("ReleaseEnvironment", func(context.Context) error { record("release"); return nil })
			reg("Initialize", func(context.Context, Data) (compiledscope.Snapshot, error) { return "root", nil })
			reg("Finish", func(context.Context, string) error { record("finish"); return nil })
			reg("SetValue", func(_ context.Context, id string, ref compiledscope.Snapshot, entry compiledscope.Entry) (compiledscope.Snapshot, error) {
				if id == "" && strings.Contains(string(ref), "child") {
					t.Error("child leaked")
				}
				return compiledscope.Snapshot(string(ref) + "/" + entry.Key + "/" + id), nil
			})
			reg("EnterScope", func(context.Context, ScopeInput) error { record("enter"); return nil })
			reg("ExitScope", func(context.Context, string, string) error { record("exit"); return nil })
			reg("NewParent", func(context.Context) (sessionHandle, error) { return sessionHandle{ID: "parent"}, nil })
			reg("Remember", func(context.Context, operationInput) (generateResult, error) {
				record("remember")
				if mode == "agent-error" {
					return generateResult{Failure: failure(errors.New("provider unavailable"))}, nil
				}
				return generateResult{Value: Report{Summary: mode, Receipt: "amber-17", File: "continuity.txt"}}, nil
			})
			reg("Edit", func(_ context.Context, in operationInput) (generateResult, error) {
				record("edit")
				if in.Session.Owner != "" || in.Scope != "child.1" {
					t.Error("ownership changed")
				}
				return generateResult{Value: Report{Summary: "child", Receipt: "amber-17"}}, nil
			})
			reg("ForkSession", func(context.Context, operationInput) (sessionHandle, error) {
				record("fork")
				return sessionHandle{Owner: "child.1", ID: "fork"}, nil
			})
			reg("Diverge", func(context.Context, operationInput) (generateResult, error) {
				record("diverge")
				return generateResult{Value: Report{Receipt: "violet-29"}}, nil
			})
			reg("Diagnostic", func(context.Context, operationInput) (commandResult, error) {
				record("diagnostic")
				return commandResult{ExitCode: 7, Stdout: "observed", Stderr: "diagnostic"}, nil
			})
			reg("Recover", func(context.Context, operationInput) (commandResult, error) {
				record("recover")
				return commandResult{}, nil
			})
			reg("Resume", func(_ context.Context, in operationInput) (generateResult, error) {
				record("resume")
				if in.Session.ID != "parent" || in.Scope != "" || strings.Contains(string(in.Context), "child") {
					t.Error("parent not restored")
				}
				return generateResult{Value: Report{Receipt: "amber-17", Summary: "parent"}}, nil
			})
			e.ExecuteWorkflow(ContinuityWorkflow, Input{})
			if (e.GetWorkflowError() != nil) != (mode == "agent-error") {
				t.Fatal(e.GetWorkflowError())
			}
			want := []string{"remember", "diagnostic", "recover", "resume", "finish", "release"}
			if mode == "edit" {
				want = []string{"remember", "enter", "edit", "fork", "diverge", "exit", "diagnostic", "recover", "resume", "finish", "release"}
			}
			if mode == "agent-error" {
				want = []string{"remember", "finish", "release"}
			}
			if !slices.Equal(trace, want) {
				t.Fatalf("trace %v want %v", trace, want)
			}
		})
	}
}
