package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
)

// The barrier would deadlock if the workflow serialized the branches. Failure
// must drain both before Finish/Release, not just schedule them.
func TestLoopParallelJoinAndCleanup(t *testing.T) {
	for _, mode := range []string{"success", "failure"} {
		t.Run(mode, func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
			env.SetTestTimeout(15 * time.Second)
			env.SetWorkerOptions(worker.Options{MaxHeartbeatThrottleInterval: 10 * time.Millisecond, DefaultHeartbeatThrottleInterval: 10 * time.Millisecond})
			var mu sync.Mutex
			var order []string
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
			register("Initialize", func(_ context.Context, d Data) error {
				if d.Task != "root task" {
					t.Error("root context lost")
				}
				record("init")
				return nil
			})
			register("Prepare", func(context.Context) error { record("prepare"); return nil })
			register("BeginIteration", func(_ context.Context, d IterationData) error {
				mu.Lock()
				defer mu.Unlock()
				if active != 0 {
					t.Error("next iteration overlaps unfinished branches")
				}
				iteration++
				arrived = 0
				barrier = make(chan struct{})
				if d.Pair.Number != iteration || (iteration == 2 && d.Previous.Stdout != "checks-1") {
					t.Errorf("lost iteration context: %+v", d)
				}
				order = append(order, fmt.Sprintf("begin-%d", iteration))
				return nil
			})
			branch := func(ctx context.Context, a Assignment) (Report, error) {
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
				if mode == "failure" && a.File == "add.go" {
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
				return Report{File: a.File}, nil
			}
			register("FixLeft", branch)
			register("FixRight", branch)
			register("IterationTests", func(_ context.Context, d IterationData) (Checks, error) {
				mu.Lock()
				defer mu.Unlock()
				if active != 0 {
					t.Error("test ran before join")
				}
				order = append(order, fmt.Sprintf("test-%d", iteration))
				return Checks{Stdout: fmt.Sprintf("checks-%d", iteration)}, nil
			})
			register("EndIteration", func(context.Context) error { record("end"); return nil })
			register("FinalTests", func(context.Context) (Checks, error) { record("final"); return Checks{Stdout: "all pass"}, nil })
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
				order = append(order, "finish")
				return nil
			})
			register("ReleaseEnvironment", func(context.Context) error { record("release"); return nil })
			env.ExecuteWorkflow(ReviewWorkflow, Input{Task: "root task"})
			if (env.GetWorkflowError() != nil) != (mode != "success") {
				t.Fatalf("result: %v", env.GetWorkflowError())
			}
			got := strings.Join(order, ",")
			want := "provision,init,prepare,begin-1,finish,release"
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
				if len(out.Reports) != 4 || len(out.Iterations) != 2 || out.Final.Stdout != "all pass" {
					t.Fatalf("outcome=%+v", out)
				}
			}
		})
	}
}
