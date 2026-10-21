package host

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/internal/runner"
)

func TestRemoteRoutesRestoreControlWithoutSharedRunFiles(t *testing.T) {
	var starts, controls atomic.Int32
	var admitted backend.Workflow
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workflows/start":
			starts.Add(1)
			_ = json.NewDecoder(r.Body).Decode(&admitted)
			_ = json.NewEncoder(w).Encode(backend.WorkflowStatus{ID: admitted.ID, State: "running"})
		case "/workflows/control":
			controls.Add(1)
			_ = json.NewEncoder(w).Encode(backend.Reply{Value: true})
		case "/workflows/observation":
			snapshot, _ := json.Marshal(observation.RunSnapshot{Stream: "s", Position: 1, Run: observation.RunRow{ID: admitted.ID, Name: "review", Status: observation.StatusRunning}})
			_ = json.NewEncoder(w).Encode(backend.Observation{Snapshot: snapshot, Cursor: "s:1"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	config, _ := json.Marshal(map[string]string{"endpoint": server.URL})
	instance, project := t.TempDir(), t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	first := New(ctx, instance)
	p, err := first.AdmitProject(project)
	if err != nil {
		t.Fatal(err)
	}
	id, err := p.Start(invocation.Request{Workflow: "review", Input: json.RawMessage(`{"goal":"inspect"}`), WorkDir: "/execution-A/repository", WorkflowProject: "/execution-A/project", WorkflowRoute: backend.Route{Name: "http", Configuration: config, Label: "A"}, TaskEndpoint: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if admitted.TaskRoute.Environment != "" || admitted.TaskRoute.Label != server.URL || admitted.TaskRoute.Name != "http" {
		t.Fatalf("task endpoint became orchestrated placement: %+v", admitted.TaskRoute)
	}
	if admitted.Project != "/execution-A/project" || admitted.Workdir != "/execution-A/repository" || admitted.ID != id {
		t.Fatalf("wrong admission %+v", admitted)
	}
	if _, err := os.Stat(filepath.Join(project, ".gimbal", "runs", id)); !os.IsNotExist(err) {
		t.Fatalf("central opened A run directory: %v", err)
	}
	cancel()
	first.Close()
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	second := New(ctx, instance)
	defer second.Close()
	if err := second.RestoreProjects(); err != nil {
		t.Fatal(err)
	}
	p, err = second.Project(project)
	if err != nil {
		t.Fatal(err)
	}
	controller, err := p.Runs().InProgress(id)
	if err != nil {
		t.Fatal(err)
	}
	landed, err := controller.Steer(context.Background(), "session", "continue")
	if err != nil || !landed {
		t.Fatalf("restored steer: %v %v", landed, err)
	}
	snapshot, err := p.Registry().Snapshot(id)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Run.WorkflowEnvironment != "A" || snapshot.Run.TaskEnvironment != server.URL {
		t.Fatalf("lost placement %+v", snapshot.Run)
	}
	ids, err := p.Registry().IDs()
	if err != nil || len(ids) != 1 || ids[0] != id {
		t.Fatalf("lost run catalog %v %v", ids, err)
	}
	if starts.Load() != 1 || controls.Load() != 1 {
		t.Fatalf("restart resubmitted or lost controls: %d %d", starts.Load(), controls.Load())
	}
}

func TestLostWorkflowAdmissionReplyReconcilesIdentityWithoutReplay(t *testing.T) {
	var starts atomic.Int32
	var id string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workflows/start":
			starts.Add(1)
			var v backend.Workflow
			_ = json.NewDecoder(r.Body).Decode(&v)
			id = v.ID
			http.Error(w, "reply lost", http.StatusServiceUnavailable)
		case "/workflows/status":
			_ = json.NewEncoder(w).Encode(backend.WorkflowStatus{ID: id, State: "running"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	config, _ := json.Marshal(map[string]string{"endpoint": server.URL})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	owner := New(ctx, t.TempDir())
	defer owner.Close()
	p, err := owner.AdmitProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Start(invocation.Request{Workflow: "review", Input: json.RawMessage(`{"goal":"inspect"}`), WorkDir: "/A/work", WorkflowProject: "/A/project", WorkflowRoute: backend.Route{Name: "http", Configuration: config}})
	if err != nil || got != id || starts.Load() != 1 {
		t.Fatalf("unsafe admission retry: %q %q %d %v", got, id, starts.Load(), err)
	}
}

func TestRejectedWorkflowAdmissionIsKnownFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(backend.WorkflowStatus{State: "failed", Error: "unknown task backend"})
	}))
	defer server.Close()
	owner := New(t.Context(), t.TempDir())
	defer owner.Close()
	project, err := owner.AdmitProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	config, _ := json.Marshal(map[string]string{"endpoint": server.URL})
	_, err = project.startRouted(invocation.Request{RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Workflow: "review", WorkflowRoute: backend.Route{Name: "http", Configuration: config}})
	if err == nil || err.Error() != "unknown task backend" {
		t.Fatalf("admission error %v", err)
	}
	record, err := project.readRoute("01ARZ3NDEKTSV4RRFFQ69G5FAV")
	if err != nil {
		t.Fatal(err)
	}
	if record.Admission != "failed" {
		t.Fatalf("known rejection marked %s", record.Admission)
	}
}

func TestLocalWorkflowRetainsRemoteTaskDirectoryAndRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks/cleanup" {
			t.Errorf("unexpected native dispatch %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(backend.Reply{})
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	owner := New(ctx, t.TempDir())
	defer owner.Close()
	project, err := owner.AdmitProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	configuration, _ := json.Marshal(map[string]string{"endpoint": server.URL})
	route := backend.Route{Name: "http", Configuration: configuration, Label: "Separate daemon"}
	id, err := project.Start(invocation.Request{Workflow: "conversation", Input: json.RawMessage(`{"purpose":"Remote placement."}`), WorkDir: "/only-on-remote-daemon/repository", TaskRoute: route})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(project.Dir(), "invocations", id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved invocation.Request
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.WorkDir != "/only-on-remote-daemon/repository" || saved.TaskRoute.Name != "http" || string(saved.TaskRoute.Configuration) != string(configuration) {
		t.Fatalf("saved placement=%+v", saved)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		control, err := project.Runs().InProgress(id)
		if err == nil {
			err = control.SubmitInteractive("conversation", `{"id":"finish","finish":true}`)
		}
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	for {
		snapshot, err := project.Registry().Snapshot(id)
		if err == nil && snapshot.Run.Status == observation.StatusCompleted {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("completion=%+v %v", snapshot.Run, err)
		}
		time.Sleep(time.Millisecond)
	}
	// The completed observation precedes the independent runner's final writes.
	// Retain its project until invocation cleanup and controller shutdown finish.
	for {
		_, invocationErr := os.Stat(filepath.Join(project.Dir(), "invocations", id+".json"))
		_, controllerErr := runner.Connect(project.Dir(), id)
		if os.IsNotExist(invocationErr) && controllerErr != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("completed workflow host did not finish teardown")
		}
		time.Sleep(time.Millisecond)
	}
}

// A failed conversation start only releases its run identifier when the route
// record proves the run never began; an ambiguous or accepted one keeps it.
func TestFailedStartOutcomeFollowsAdmissionRecord(t *testing.T) {
	owner := New(t.Context(), t.TempDir())
	defer owner.Close()
	project, err := owner.AdmitProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cause := errors.New("start failed")
	for _, tc := range []struct {
		name, run, admission string
		uncertain            bool
	}{
		{"never recorded", "01ARZ3NDEKTSV4RRFFQ69G5FA0", "", false},
		{"rejected", "01ARZ3NDEKTSV4RRFFQ69G5FA1", "failed", false},
		{"ambiguous", "01ARZ3NDEKTSV4RRFFQ69G5FA2", "unknown", true},
		{"accepted but record write failed", "01ARZ3NDEKTSV4RRFFQ69G5FA3", "accepted", true},
		{"pending", "01ARZ3NDEKTSV4RRFFQ69G5FA4", "pending", true},
	} {
		if tc.admission != "" {
			if err := project.writeRoute(routeRecord{ID: tc.run, Workflow: "conversation", Admission: tc.admission}); err != nil {
				t.Fatal(err)
			}
		}
		got := project.startOutcome(tc.run, cause)
		uncertain, ok := errors.AsType[*backend.DeliveryUncertain](got)
		if ok != tc.uncertain || !errors.Is(got, cause) || (ok && uncertain.RequestID != tc.run) {
			t.Errorf("%s: %v", tc.name, got)
		}
	}
}
