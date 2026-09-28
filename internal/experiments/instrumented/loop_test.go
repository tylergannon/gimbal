package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/compiledscope"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
)

// The barrier would deadlock if the workflow serialized the branches. Failure
// must drain both before Finish/Release, not just schedule them.
func TestLoopParallelJoinAndCleanup(t *testing.T) {
	for _, mode := range []string{"success", "failure", "entry-failure"} {
		t.Run(mode, func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
			env.SetTestTimeout(15 * time.Second)
			env.SetWorkerOptions(worker.Options{MaxHeartbeatThrottleInterval: 10 * time.Millisecond, DefaultHeartbeatThrottleInterval: 10 * time.Millisecond})
			var mu sync.Mutex
			var order []string
			openScopes := map[string]string{}
			active := 0
			iteration := 0
			arrived := 0
			var barrier chan struct{}
			record := func(s string) { mu.Lock(); defer mu.Unlock(); order = append(order, s) }
			register := func(n string, f any) { env.RegisterActivityWithOptions(f, activity.RegisterOptions{Name: n}) }
			register("ProvisionEnvironment", func(context.Context) (Environment, error) {
				record("provision")
				return Environment{Queue: "specimen"}, nil
			})
			register("Initialize", func(_ context.Context, d Data) (compiledscope.Snapshot, error) {
				if d.Task != "root task" {
					t.Error("root context lost")
				}
				record("init")
				return "root", nil
			})
			register("Prepare", func(context.Context) error { record("prepare"); return nil })
			register("SetIteration", func(_ context.Context, id string, ref compiledscope.Snapshot, d IterationData) (compiledscope.Snapshot, error) {
				mu.Lock()
				defer mu.Unlock()
				if active != 0 {
					t.Error("next iteration overlaps unfinished branches")
				}
				iteration++
				arrived = 0
				barrier = make(chan struct{})
				if d.Pair.Number != iteration || (iteration == 2 && d.Previous.Context != "checks-1") {
					t.Errorf("lost iteration context: %+v", d)
				}
				order = append(order, fmt.Sprintf("begin-%d", iteration))
				if ref != "root" {
					t.Error("parent snapshot leaked")
				}
				return "iteration", nil
			})
			branch := func(ctx context.Context, id string, ref compiledscope.Snapshot) (Report, error) {
				mu.Lock()
				active++
				arrived++
				if arrived == 2 {
					close(barrier)
				}
				gate := barrier
				mu.Unlock()
				defer func() { mu.Lock(); active--; mu.Unlock() }()
				select {
				case <-gate:
				case <-ctx.Done():
					return Report{}, ctx.Err()
				}
				if mode == "failure" && ref == "add.go" {
					return Report{}, errors.New("branch failed")
				}
				if mode != "success" {
					ticker := time.NewTicker(10 * time.Millisecond)
					defer ticker.Stop()
					for {
						select {
						case <-ctx.Done():
							return Report{}, ctx.Err()
						case <-ticker.C:
							activity.RecordHeartbeat(ctx)
						}
					}
				}
				return Report{File: string(ref)}, nil
			}
			register("SetOuterContext", func(_ context.Context, _ string, ref compiledscope.Snapshot) (compiledscope.Snapshot, error) {
				if ref != "iteration" {
					t.Error("outer inheritance")
				}
				return "outer", nil
			})
			register("SetInnerContext", func(_ context.Context, _ string, ref compiledscope.Snapshot) (compiledscope.Snapshot, error) {
				if ref != "outer" {
					t.Error("inner inheritance")
				}
				return "inner", nil
			})
			register("SetAssignment", func(_ context.Context, _ string, ref compiledscope.Snapshot, a Assignment) (compiledscope.Snapshot, error) {
				if ref != "inner" {
					t.Error("sibling leaked")
				}
				return compiledscope.Snapshot(a.File), nil
			})
			register("Repair", branch)
			register("EnterScope", func(_ context.Context, in ScopeInput) error {
				mu.Lock()
				defer mu.Unlock()
				if mode == "entry-failure" && in.Name == "details" {
					return errors.New("scope entry failed")
				}
				if in.Parent != "" {
					if _, ok := openScopes[in.Parent]; !ok {
						t.Errorf("missing parent %s", in.Parent)
					}
				}
				if _, ok := openScopes[in.ID]; ok {
					t.Errorf("scope entered twice: %s", in.ID)
				}
				openScopes[in.ID] = in.Parent
				return nil
			})
			register("IterationTests", func(_ context.Context, id string, ref compiledscope.Snapshot, d IterationData) (ChecksResult, error) {
				mu.Lock()
				defer mu.Unlock()
				if active != 0 {
					t.Error("test ran before join")
				}
				order = append(order, fmt.Sprintf("test-%d", iteration))
				if ref != "iteration" {
					t.Error("child context leaked after cleanup")
				}
				return ChecksResult{Context: compiledscope.Snapshot(fmt.Sprintf("checks-%d", iteration))}, nil
			})
			register("ExitScope", func(_ context.Context, id, reason string) error {
				mu.Lock()
				defer mu.Unlock()
				for child, parent := range openScopes {
					if parent == id {
						t.Errorf("closed %s before child %s", id, child)
					}
				}
				delete(openScopes, id)
				if !strings.Contains(id, "/") {
					order = append(order, "end")
				}
				return nil
			})
			register("FinalTests", func(_ context.Context, ref compiledscope.Snapshot) (ChecksResult, error) {
				if ref != "root" {
					t.Error("iteration leaked")
				}
				record("final")
				return ChecksResult{Context: "all pass"}, nil
			})
			register("Finish", func(_ context.Context, reason string) error {
				// The SDK test environment resolves cancellation before the mock
				// goroutine exits. Mirror the real Finish's local join here.
				deadline := time.Now().Add(2 * time.Second)
				for {
					mu.Lock()
					if active == 0 {
						break
					}
					mu.Unlock()
					if time.Now().After(deadline) {
						return errors.New("branches did not stop")
					}
					time.Sleep(time.Millisecond)
				}
				defer mu.Unlock()
				if (reason != "") != (mode != "success") {
					t.Errorf("reason=%q", reason)
				}
				if len(openScopes) != 0 {
					t.Errorf("root closed with scopes still open: %v", openScopes)
				}
				order = append(order, "finish")
				return nil
			})
			register("ReleaseEnvironment", func(context.Context) error { record("release"); return nil })
			env.ExecuteWorkflow(ReviewWorkflow, Input{Task: "root task"})
			if (env.GetWorkflowError() != nil) != (mode != "success") {
				t.Fatalf("result: %v", env.GetWorkflowError())
			}
			got := strings.Join(order, ",")
			want := "provision,init,prepare,begin-1,end,finish,release"
			if mode == "success" {
				want = "provision,init,prepare,begin-1,test-1,end,begin-2,test-2,end,final,finish,release"
			}
			if got != want {
				t.Fatalf("order=%s, want=%s", got, want)
			}
			if mode == "success" {
				var out Outcome
				if err := env.GetWorkflowResult(&out); err != nil {
					t.Fatal(err)
				}
				if len(out.Reports) != 4 || len(out.Iterations) != 2 || out.Final.Context != "all pass" {
					t.Fatalf("outcome=%+v", out)
				}
			}
		})
	}
}
