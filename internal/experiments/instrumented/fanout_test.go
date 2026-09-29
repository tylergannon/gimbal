package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/experiments/instrumented/fanout"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"
)

func TestFixedFanoutCapturesAndJoins(t *testing.T) {
	for _, n := range []int{2} {
		for _, fail := range []bool{false, true} {
			if n == 0 && fail {
				continue
			}
			t.Run(fmt.Sprintf("%d/failure=%v", n, fail), func(t *testing.T) {
				a := newTestActivities(t)
				var mu sync.Mutex
				active, arrived := 0, 0
				barrier := make(chan struct{})
				var completed []int
				adapter := &specimenAdapter{turn: func(ctx context.Context, id, prompt string) (any, error) {
					var index int
					for i := range n {
						if strings.Contains(prompt, fmt.Sprintf("file=item-%d.txt", i)) {
							index = i
							break
						}
					}
					mu.Lock()
					active++
					arrived++
					if arrived == n {
						close(barrier)
					}
					mu.Unlock()
					defer func() { mu.Lock(); active--; mu.Unlock() }()
					select {
					case <-barrier:
					case <-ctx.Done():
						return nil, ctx.Err()
					}
					if fail {
						if index == 0 {
							return nil, errors.New("ordinary branch failure")
						}
						<-ctx.Done()
						return nil, ctx.Err()
					}
					select {
					case <-time.After(time.Duration(n-index) * 25 * time.Millisecond):
					case <-ctx.Done():
						return nil, ctx.Err()
					}
					mu.Lock()
					completed = append(completed, index)
					mu.Unlock()
					return Report{File: fmt.Sprintf("item-%d.txt", index), Receipt: "duplicate"}, nil
				}}
				a.models = map[gimbal.WorkflowRole]gimbal.ModelBinding{coder: {Adapter: adapter, Model: "test"}}
				var suite testsuite.WorkflowTestSuite
				e := suite.NewTestWorkflowEnvironment()
				e.SetTestTimeout(10 * time.Second)
				e.RegisterActivity(a)
				e.RegisterActivityWithOptions(func(context.Context) (Environment, error) { return Environment{Queue: "test"}, nil }, activity.RegisterOptions{Name: "ProvisionEnvironment"})
				e.RegisterActivityWithOptions(func(context.Context) error {
					mu.Lock()
					defer mu.Unlock()
					if active != 0 {
						t.Error("release before drain")
					}
					if len(a.scopes) != 0 {
						t.Error("scope leak")
					}
					return nil
				}, activity.RegisterOptions{Name: "ReleaseEnvironment"})
				var items [2]fanout.Work
				for i := range n {
					items[i] = fanout.Work{File: fmt.Sprintf("item-%d.txt", i), Receipt: "duplicate"}
				}
				e.ExecuteWorkflow(FanoutWorkflow, fanoutInput{Items: items})
				if (e.GetWorkflowError() != nil) != fail {
					t.Fatal(e.GetWorkflowError())
				}
				if !fail {
					var out [2]Report
					if err := e.GetWorkflowResult(&out); err != nil {
						t.Fatal(err)
					}
					for i, r := range out {
						if r.File != items[i].File || r.Receipt != "duplicate" {
							t.Fatal(out)
						}
					}
					if fmt.Sprint(completed) != "[1 0]" {
						t.Fatal(completed)
					}
				}
				if len(adapter.closed) != n {
					t.Fatalf("closed %v", adapter.closed)
				}
			})
		}
	}
}
func TestLiveFanout(t *testing.T) {
	if os.Getenv("SPECIMEN_LIVE_WORKFLOW") != "1" {
		t.Skip("requires live workers and paid Haiku access")
	}
	c, err := client.Dial(client.Options{DataConverter: dataConverter(stateRoot())})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	id := fmt.Sprintf("fanout-%d", time.Now().UnixMilli())
	input, err := prepareInput(stateRoot(), id, "Independent file writes.")
	if err != nil {
		t.Fatal(err)
	}
	items := [2]fanout.Work{{File: "one.txt", Receipt: "duplicate", Delay: 8}, {File: "two.txt", Receipt: "duplicate", Delay: 4}}
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Minute)
	defer cancel()
	run, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: id, TaskQueue: controlQueue}, FanoutWorkflow, fanoutInput{Input: input, Items: items})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("workflow %s", id)
	var out [2]Report
	if err = run.Get(ctx, &out); err != nil {
		t.Fatal(err)
	}
	for i, item := range items {
		b, err := os.ReadFile(filepath.Join(stateRoot(), environmentID(id), "workspace", item.File))
		if err != nil || strings.TrimSpace(string(b)) != item.Receipt || out[i].File != item.File {
			t.Fatalf("%s %q %+v %v", item.File, b, out, err)
		}
	}
	t.Logf("two indexed results and two retained files: %+v", out)
}
