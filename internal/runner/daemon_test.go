package runner

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/runstore"
)

func TestWorkflowDaemonRestoresIndexAndDeduplicatesAdmission(t *testing.T) {
	project, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	state := t.TempDir()
	id := string(runstore.NewRunID())
	request := backend.Workflow{ID: id, Name: "conversation", Project: project, Workdir: project, Input: json.RawMessage(`{"purpose":"Discuss."}`), Roles: map[string]string{"agentic-dialogues": "luna"}}
	first := &WorkflowDaemon{Root: project, StateDir: state}
	status := first.admit(request)
	if status.Error != "" || status.State != "running" {
		t.Fatalf("admission=%+v", status)
	}
	second := &WorkflowDaemon{Root: project, StateDir: state}
	if err := second.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	if restored, err := second.project(id); err != nil || restored != project {
		t.Fatalf("project=%q err=%v", restored, err)
	}
	if status := second.admit(request); status.Error != "" {
		t.Fatalf("same admission=%+v", status)
	}
	request.Input = json.RawMessage(`{"purpose":"Conflicting work."}`)
	if status := second.admit(request); status.Error == "" {
		t.Fatal("conflicting identity accepted")
	}
	server := httptest.NewServer(second.Handler())
	defer server.Close()
	transport := backend.HTTP{Endpoint: server.URL}
	if _, err := transport.ControlWorkflow(t.Context(), backend.Control{Run: id, Action: "interactive", Target: "conversation", Message: `{"id":"finish","finish":true}`}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		status, err := transport.WorkflowStatus(t.Context(), backend.Target{Run: id})
		if err != nil {
			t.Fatal(err)
		}
		if status.State == observation.StatusCompleted {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("finish=%+v", status)
		}
		time.Sleep(10 * time.Millisecond)
	}
	third := &WorkflowDaemon{Root: project, StateDir: state}
	if err := third.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got, err := third.project(id); err != nil || got != project {
		t.Fatalf("terminal index=%q %v", got, err)
	}
	snapshot, err := observation.NewRegistry(filepath.Join(project, ".gimbal")).Snapshot(id)
	if err != nil || snapshot.Run.Status != observation.StatusCompleted {
		t.Fatalf("terminal=%+v err=%v", snapshot.Run, err)
	}
}
