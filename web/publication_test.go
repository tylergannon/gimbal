package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimble/internal/observation"
)

func TestHostedRunPublicationBeforeDirectoryIsReadable(t *testing.T) {
	project := t.TempDir()
	instanceDir := filepath.Join(t.TempDir(), "instance")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	instance, err := NewInstance(ctx, instanceDir, nil, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := instance.AdmitProject(project)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	base := "http://" + instance.address + "/projects/" + runtime.id
	get := func(path string) *http.Response {
		t.Helper()
		response, err := client.Get(base + path)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(response.Body)
			_ = response.Body.Close()
			t.Fatalf("GET %s: HTTP %d: %s", path, response.StatusCode, body)
		}
		return response
	}

	// Open uses the same directory callback as gimble.Run. Hold it after
	// mkdir, when the list can discover the run but no observation table or
	// run log exists yet. All three readers must use the published store.
	const id = "publication-boundary"
	dir := filepath.Join(project, ".gimble", "runs", id)
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	created := make(chan struct{})
	continueOpen := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-continueOpen:
		default:
			close(continueOpen)
		}
	})
	opened := make(chan *observation.Store, 1)
	openErr := make(chan error, 1)
	go func() {
		store, err := observation.Open(runtime.registry, id, "publication", dir, func() error {
			if err := os.Mkdir(dir, 0o755); err != nil {
				return err
			}
			close(created)
			<-continueOpen
			return nil
		})
		opened <- store
		openErr <- err
	}()
	<-created
	if _, err := os.Stat(filepath.Join(dir, "run.json")); !os.IsNotExist(err) {
		t.Fatalf("run table already exists at the publication boundary: %v", err)
	}
	store, live := runtime.registry.Live(id)
	if !live {
		t.Fatal("directory is visible but its hosted observation is not live")
	}
	list := get("")
	listBody, err := io.ReadAll(list.Body)
	_ = list.Body.Close()
	if err != nil || !strings.Contains(string(listBody), "publication") {
		t.Fatalf("list during publication = %q, %v", listBody, err)
	}
	snapshotResponse := get("/api/runs/" + id)
	var snapshot observation.RunSnapshot
	if err := json.NewDecoder(snapshotResponse.Body).Decode(&snapshot); err != nil {
		t.Fatal(err)
	}
	_ = snapshotResponse.Body.Close()
	if snapshot.Run.ID != id || snapshot.Run.Name != "publication" || snapshot.Run.Status != observation.StatusRunning {
		t.Fatalf("snapshot during publication = %+v", snapshot.Run)
	}
	stream := get("/api/runs/" + id + "/events")
	if stream.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("stream content type = %q", stream.Header.Get("Content-Type"))
	}
	close(continueOpen)
	if got, err := <-opened, <-openErr; err != nil || got != store {
		t.Fatalf("published store changed during open: %p, %p, %v", store, got, err)
	}
	if err := store.Lifecycle(json.RawMessage(`{"seq":1,"time":"2026-09-22T12:00:00Z","scope":"","event":{"kind":"run_started","name":"publication"}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Lifecycle(json.RawMessage(`{"seq":2,"time":"2026-09-22T12:00:01Z","scope":"","event":{"kind":"run_ended","name":"publication","error":""}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	streamBody, err := io.ReadAll(stream.Body)
	_ = stream.Body.Close()
	if err != nil || !strings.Contains(string(streamBody), `"status":"running"`) || !strings.Contains(string(streamBody), `"status":"completed"`) {
		t.Fatalf("stream across completion = %q, %v", streamBody, err)
	}
	completedList := get("")
	completedBody, err := io.ReadAll(completedList.Body)
	_ = completedList.Body.Close()
	if err != nil || !strings.Contains(string(completedBody), "publication") || !strings.Contains(string(completedBody), "Ended") {
		t.Fatalf("list after completion = %q, %v", completedBody, err)
	}

	// A real hosted run uses that publication path and remains readable on
	// all views through its body and after reopening a new instance.
	entered := make(chan struct{})
	finish := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-finish:
		default:
			close(finish)
		}
	})
	runErr := make(chan error, 1)
	go func() {
		runErr <- runtime.Run(ctx, "hosted", nil, func(context.Context) error {
			close(entered)
			<-finish
			return nil
		})
	}()
	<-entered
	entries, err := os.ReadDir(filepath.Join(project, ".gimble", "runs"))
	if err != nil {
		t.Fatal(err)
	}
	var hostedID string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".hosted") {
			hostedID = entry.Name()
		}
	}
	if hostedID == "" {
		t.Fatal("hosted run directory was not created")
	}
	hosted := get("/api/runs/" + hostedID)
	if err := json.NewDecoder(hosted.Body).Decode(&snapshot); err != nil {
		t.Fatal(err)
	}
	_ = hosted.Body.Close()
	if snapshot.Run.Status != observation.StatusRunning {
		t.Fatalf("hosted snapshot during body = %+v", snapshot.Run)
	}
	page := get("")
	pageBody, _ := io.ReadAll(page.Body)
	_ = page.Body.Close()
	if !strings.Contains(string(pageBody), hostedID) {
		t.Fatal("project list omitted the hosted run")
	}
	hostedStream := get("/api/runs/" + hostedID + "/events")
	close(finish)
	if err := <-runErr; err != nil {
		t.Fatal(err)
	}
	hostedFrames, err := io.ReadAll(hostedStream.Body)
	_ = hostedStream.Body.Close()
	if err != nil || !strings.Contains(string(hostedFrames), `"status":"completed"`) {
		t.Fatalf("hosted stream completion = %q, %v", hostedFrames, err)
	}
	completedHosted := get("/api/runs/" + hostedID)
	if err := json.NewDecoder(completedHosted.Body).Decode(&snapshot); err != nil {
		t.Fatal(err)
	}
	_ = completedHosted.Body.Close()
	if snapshot.Run.Status != observation.StatusCompleted {
		t.Fatalf("hosted snapshot after completion = %+v", snapshot.Run)
	}
	cancel()
	<-instance.done
	restartCtx, stop := context.WithCancel(t.Context())
	defer stop()
	restarted, err := NewInstance(restartCtx, instanceDir, nil, WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := restarted.AdmitProject(project)
	if err != nil {
		t.Fatal(err)
	}
	for _, runID := range []string{id, hostedID} {
		saved, err := reopened.registry.Snapshot(runID)
		if err != nil || saved.Run.Status != observation.StatusCompleted {
			t.Fatalf("reopened %s = %+v, %v", runID, saved.Run, err)
		}
	}
}
