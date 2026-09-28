package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// Set SPECIMEN_HISTORY to a history exported by temporal workflow show --output json.
func TestRecordedHistoryReplay(t *testing.T) {
	path := os.Getenv("SPECIMEN_HISTORY")
	if path == "" {
		t.Skip("no recorded history supplied")
	}
	replayer, err := worker.NewWorkflowReplayerWithOptions(worker.WorkflowReplayerOptions{DataConverter: dataConverter(stateRoot())})
	if err != nil {
		t.Fatal(err)
	}
	replayer.RegisterWorkflow(ContinuityWorkflow)
	replayer.RegisterWorkflow(FanoutWorkflow)
	replayer.RegisterWorkflow(PlanningWorkflow)
	replayer.RegisterWorkflow(ReviewWorkflow)
	if err := replayer.ReplayWorkflowHistoryFromJSONFile(nil, path); err != nil {
		t.Fatal(err)
	}
}

func TestControlRejectsCrossOriginBeforeMutation(t *testing.T) {
	for _, path := range []string{"/control/cancel", "/control/steer", "/_app/remote/steer"} {
		t.Run(path, func(t *testing.T) {
			invoked := false
			handler := sameOrigin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { invoked = true; w.WriteHeader(202) }))
			request := httptest.NewRequest("POST", "http://127.0.0.1:12345"+path, nil)
			request.Header.Set("Origin", "https://unrelated.example")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if invoked || response.Code != 403 {
				t.Fatalf("cross-origin mutation reached handler: invoked=%v status=%d", invoked, response.Code)
			}
			request.Header.Set("Origin", "http://127.0.0.1:12345")
			response = httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if !invoked || response.Code != 202 {
				t.Fatal("same-origin control was rejected")
			}
		})
	}
}

// Run against the prepared control worker and its shared SPECIMEN_STATE_ROOT.
// The task is larger than Temporal's payload limit; only its immutable reference
// may enter workflow history. This also exercises container-side initialization.
func TestLiveLargeInitialContext(t *testing.T) {
	if os.Getenv("SPECIMEN_LIVE_WORKFLOW") != "1" {
		t.Skip("requires prepared Temporal/control workers and paid provider access")
	}
	c, err := client.Dial(client.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	id := fmt.Sprintf("context-input-%d", time.Now().UnixMilli())
	task := "Repair the four assigned defects.\n" + strings.Repeat("Initial task background; no additional work required.\n", 60000)
	input, err := prepareInput(stateRoot(), id, task)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	run, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: id, TaskQueue: controlQueue}, ReviewWorkflow, input)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_ = c.CancelWorkflow(cleanup, id, run.GetRunID())
	}()
	t.Logf("workflow %s; initial task %d bytes; context %s", id, len(task), input.Context)
	var out Outcome
	if err := run.Get(ctx, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Reports) != 4 || len(out.Iterations) != 2 || out.Final.ExitCode != 0 {
		t.Fatalf("incomplete outcome: %+v", out)
	}
	for _, r := range out.Reports {
		if r.Receipt != "middle-of-external-context-42" {
			t.Fatalf("agent did not retrieve overflow: %+v", r)
		}
	}
	for _, name := range []string{"operations_test.go", "go.mod.txt"} {
		original, err := fixture.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		target := name
		if name == "go.mod.txt" {
			target = "go.mod"
		}
		actual, err := os.ReadFile(filepath.Join(stateRoot(), environmentID(id), "workspace", target))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(original, actual) {
			t.Fatalf("agent changed protected fixture %s", target)
		}
	}
	t.Logf("four real repairs and overflow receipts; two iteration checks and final suite passed")
}
