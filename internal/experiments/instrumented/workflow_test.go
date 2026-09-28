package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/tylergannon/gimbal/internal/host"
	"go.temporal.io/sdk/worker"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

func TestWorkflowCarriesCommandResultAndCleansUp(t *testing.T) {
	for _, failCommand := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "command-fails"}[failCommand], func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
			var order []string
			register := func(name string, f any) { env.RegisterActivityWithOptions(f, activity.RegisterOptions{Name: name}) }
			register("ProvisionEnvironment", func(context.Context) (Environment, error) {
				order = append(order, "provision")
				return Environment{Queue: "isolated"}, nil
			})
			register("Initialize", func(_ context.Context, data Data) error {
				order = append(order, "initialize")
				if data.Task != "root task" {
					t.Errorf("missing task: %+v", data)
				}
				return nil
			})
			register("Review_RunTests", func(_ context.Context, data Data) (Checks, error) {
				order = append(order, "command")
				if failCommand {
					return Checks{}, errors.New("command failed")
				}
				return Checks{Stdout: "actual result"}, nil
			})
			register("Review_GenerateReport", func(_ context.Context, data Data) (Report, error) {
				order = append(order, "generate")
				if data.Task != "root task" || data.Checks.Stdout != "actual result" {
					t.Errorf("lost effective context: %+v", data)
				}
				return Report{Summary: "checked", Marker: "actual result"}, nil
			})
			register("Finish", func(_ context.Context, reason string) error {
				order = append(order, "finish")
				if failCommand && reason == "" {
					t.Error("missing failure outcome")
				}
				return nil
			})
			register("ReleaseEnvironment", func(context.Context) error { order = append(order, "release"); return nil })
			env.ExecuteWorkflow(ReviewWorkflow, Input{Task: "root task"})
			if !env.IsWorkflowCompleted() {
				t.Fatal("not complete")
			}
			if (env.GetWorkflowError() != nil) != failCommand {
				t.Fatalf("unexpected workflow result: %v", env.GetWorkflowError())
			}
			want := []string{"provision", "initialize", "command", "generate", "finish", "release"}
			if failCommand {
				want = []string{"provision", "initialize", "command", "finish", "release"}
			}
			if !reflect.DeepEqual(order, want) {
				t.Fatalf("order %v, want %v", order, want)
			}
		})
	}
}

// Set SPECIMEN_HISTORY to a history exported by temporal workflow show --output json.
func TestRecordedHistoryReplay(t *testing.T) {
	path := os.Getenv("SPECIMEN_HISTORY")
	if path == "" {
		t.Skip("no recorded history supplied")
	}
	replayer := worker.NewWorkflowReplayer()
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

func TestCancellationStillFinishesAndReleases(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(func(context.Context) (Environment, error) { return Environment{Queue: "isolated"}, nil }, activity.RegisterOptions{Name: "ProvisionEnvironment"})
	env.RegisterActivityWithOptions(func(context.Context, Data) error { return nil }, activity.RegisterOptions{Name: "Initialize"})
	env.RegisterActivityWithOptions(func(context.Context, Data) (Checks, error) { return Checks{}, nil }, activity.RegisterOptions{Name: "Review_RunTests"})
	env.RegisterActivityWithOptions(func(context.Context, Data) (Report, error) { return Report{}, nil }, activity.RegisterOptions{Name: "Review_GenerateReport"})
	var cleanup []string
	env.RegisterActivityWithOptions(func(_ context.Context, reason string) error {
		cleanup = append(cleanup, "finish")
		if reason == "" {
			t.Error("cancel outcome lost")
		}
		return nil
	}, activity.RegisterOptions{Name: "Finish"})
	env.RegisterActivityWithOptions(func(context.Context) error { cleanup = append(cleanup, "release"); return nil }, activity.RegisterOptions{Name: "ReleaseEnvironment"})
	env.OnActivity("Review_GenerateReport", mock.Anything, mock.Anything).Return(Report{}, nil).After(time.Minute)
	env.RegisterDelayedCallback(env.CancelWorkflow, time.Second)
	env.ExecuteWorkflow(ReviewWorkflow, Input{Task: "cancel me"})
	if env.GetWorkflowError() == nil {
		t.Fatal("cancelled execution reported success")
	}
	if !reflect.DeepEqual(cleanup, []string{"finish", "release"}) {
		t.Fatalf("cleanup: %v", cleanup)
	}
}

func TestActivitiesKeepOneRuntimeAndWorkspace(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "test-key")
	ctx := t.Context()
	dir := t.TempDir()
	owner := host.New(ctx, t.TempDir())
	defer owner.Close()
	project, err := owner.AdmitProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := &Activities{project: project, workdir: dir}
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(a)
	if _, err := env.ExecuteActivity(a.Initialize, Data{Task: "fixture task"}); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = env.ExecuteActivity(a.Finish, "") }()
	encoded, err := env.ExecuteActivity(a.Review_RunTests, Data{Task: "fixture task"})
	if err != nil {
		t.Fatal(err)
	}
	var checks Checks
	if err := encoded.Get(&checks); err != nil {
		t.Fatal(err)
	}
	marker, err := os.ReadFile(filepath.Join(dir, "marker.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if checks.ExitCode != 0 || checks.Stdout != string(marker) || string(marker) != "workspace-preserved\n" {
		t.Fatalf("command/file mismatch: %+v marker=%q", checks, marker)
	}
	if _, err := env.ExecuteActivity(a.Finish, ""); err != nil {
		t.Fatal(err)
	}
	runs, err := filepath.Glob(filepath.Join(project.Dir(), "runs", "*", "run.jsonl"))
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs=%v err=%v", runs, err)
	}
	raw, err := os.ReadFile(runs[0])
	if err != nil {
		t.Fatal(err)
	}
	var last string
	for i, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
		var record struct {
			Seq   int
			Event struct{ Kind string }
		}
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal(err)
		}
		if record.Seq != i+1 {
			t.Fatalf("noncontiguous sequence: %d at %d", record.Seq, i)
		}
		last = record.Event.Kind
	}
	if last != "complete" {
		t.Fatalf("runtime did not finish recording: %s", last)
	}
}
