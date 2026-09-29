package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/contextdata"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/observation"
	routes "github.com/tylergannon/gimbal/internal/skgo/links/onzggl3sn52xizlt"
	"github.com/tylergannon/gimbal/web"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// Uses the generated workflow, real Temporal scheduling, real hosted command,
// and deterministic blocking harness. Provisioning remains local. An optional
// browser port leaves the command to a human/browser and retains the terminal
// view briefly for visual inspection.
func TestLiveCompiledConsoleCancellation(t *testing.T) {
	address := os.Getenv("SPECIMEN_TEMPORAL_ADDRESS")
	if address == "" {
		t.Skip("requires isolated Temporal dev server; no provider calls")
	}
	modes := []string{"accepted", "terminated", "timeout"}
	if selected := os.Getenv("SPECIMEN_CONSOLE_MODE"); selected != "" {
		modes = []string{selected}
	}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("TYPESAFE_API_KEY", "test-key")
			ctx, stop := context.WithCancel(t.Context())
			defer stop()
			root := t.TempDir()
			projectDir := filepath.Join(root, "workspace")
			if err := os.MkdirAll(projectDir, 0755); err != nil {
				t.Fatal(err)
			}
			store := contextdata.Store{Root: filepath.Join(root, "context"), LocalDir: filepath.Join(root, "worker-materialized")}
			hostCtx, err := host.WithContextStore(ctx, projectDir, &store, filepath.Join(root, "host-cache"))
			if err != nil {
				t.Fatal(err)
			}
			portText := os.Getenv("SPECIMEN_BROWSER_PORT")
			option := web.WithNoWeb()
			if portText != "" {
				port, err := strconv.Atoi(portText)
				if err != nil {
					t.Fatal(err)
				}
				option = web.WithPort(port)
			}
			instance, err := web.NewInstance(hostCtx, t.TempDir(), []string{projectDir}, option)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { stop(); instance.Wait() }()
			project, err := instance.Owner.Project(projectDir)
			if err != nil {
				t.Fatal(err)
			}
			c, err := client.Dial(client.Options{HostPort: address, DataConverter: dataConverter(root)})
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			id := fmt.Sprintf("console-%s-%d", mode, time.Now().UnixNano())
			entered := make(chan struct{})
			var once sync.Once
			a := &Activities{project: project, workdir: projectDir, store: store}
			a.models = map[gimbal.WorkflowRole]gimbal.ModelBinding{coder: {Model: "deterministic-blocking", Adapter: &specimenAdapter{turn: func(ctx context.Context, _, _ string) (any, error) {
				once.Do(func() { close(entered) })
				<-ctx.Done()
				return nil, context.Cause(ctx)
			}}}}
			a.controls = host.CompiledControls{DeliveryTimeout: 100 * time.Millisecond, CancelRun: func(ctx context.Context, _ gimbal.Killed) error {
				if mode == "timeout" {
					<-ctx.Done()
					return ctx.Err()
				}
				return c.CancelWorkflow(ctx, id, "")
			}}
			w := worker.New(c, id, worker.Options{MaxHeartbeatThrottleInterval: time.Second, DefaultHeartbeatThrottleInterval: time.Second})
			w.RegisterWorkflow(ContinuityWorkflow)
			w.RegisterActivity(a)
			control := worker.New(c, controlQueue, worker.Options{})
			control.RegisterActivityWithOptions(func(context.Context) (Environment, error) { return Environment{Queue: id}, nil }, activity.RegisterOptions{Name: "ProvisionEnvironment"})
			control.RegisterActivityWithOptions(func(context.Context) error { return nil }, activity.RegisterOptions{Name: "ReleaseEnvironment"})
			if err = control.Start(); err != nil {
				t.Fatal(err)
			}
			defer control.Stop()
			if err = w.Start(); err != nil {
				t.Fatal(err)
			}
			defer w.Stop()
			run, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: id, TaskQueue: id}, ContinuityWorkflow, Input{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = c.TerminateWorkflow(context.WithoutCancel(ctx), id, run.GetRunID(), "test completed") }()
			select {
			case <-entered:
			case <-time.After(30 * time.Second):
				t.Fatal("generated turn did not start")
			}
			paths, err := filepath.Glob(filepath.Join(project.Dir(), "runs", "*"))
			if err != nil || len(paths) != 1 {
				t.Fatalf("runs %v: %v", paths, err)
			}
			runID := filepath.Base(paths[0])
			// Temporal accepts RequestCancel for a retained terminated execution too;
			// a nil callback means delivery acceptance, not backend completion.
			if mode == "terminated" {
				if err = c.TerminateWorkflow(ctx, id, run.GetRunID(), "controller terminated before console stop"); err != nil {
					t.Fatal(err)
				}
			}
			if portText != "" {
				t.Logf("BROWSER http://127.0.0.1:%s project=%s run=%s mode=%s", portText, projectDir, runID, mode)
			} else {
				result, commandErr := routes.Skgo_cancelRun(project.Context(), routes.CancelRun{Run: runID})
				if mode != "timeout" && (commandErr != nil || !result.Accepted) {
					t.Fatalf("cancel: %+v %v", result, commandErr)
				}
				if mode == "timeout" && commandErr == nil {
					t.Fatal("undelivered cancellation reported accepted")
				}
			}
			select {
			case <-a.drainDone:
			case <-time.After(5 * time.Minute):
				t.Fatal("local drain did not finish")
			}
			snapshot, err := observation.NewRegistry(project.Dir()).Snapshot(runID)
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.Run.Status != observation.StatusCancelled {
				t.Fatalf("local status %s", snapshot.Run.Status)
			}
			if portText != "" {
				time.Sleep(45 * time.Second)
			}
			if mode == "accepted" {
				wait, cancel := context.WithTimeout(ctx, 20*time.Second)
				defer cancel()
				if err = run.Get(wait, nil); err == nil {
					t.Fatal("backend run succeeded after cancellation")
				}
			}
			t.Logf("%s: generated turn stopped; local resources drained; retained status %s", mode, snapshot.Run.Status)
		})
	}
}
