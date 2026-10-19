package host

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/internal/observation"
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
