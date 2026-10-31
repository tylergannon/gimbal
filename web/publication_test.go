package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/runstore"
	"github.com/tylergannon/gimbal/web/runview"
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
	runtime, err := instance.Owner.AdmitProject(project)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	base := "http://" + instance.address + "/projects/" + runtime.ID()
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

	// Open uses the same directory callback as gimbal.Run. Hold it after
	// mkdir, when the list can discover the run but no observation table or
	// run log exists yet. Readers must retain the incomplete reservation.
	const id = "publication-boundary"
	dir := filepath.Join(project, ".gimbal", "runs", id)
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
		store, err := observation.Open(runtime.Registry(), id, "publication", dir, func() error {
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
	if _, err := runtime.Registry().Snapshot(id); !errors.Is(err, runstore.ErrIncomplete) {
		t.Fatalf("initializing reservation replaced by external read: %v", err)
	}
	ids, err := runtime.Registry().IDs()
	if err != nil || len(ids) != 0 {
		t.Fatalf("unaccepted initialization listed: %v %v", ids, err)
	}
	next := watchTestRuns(t, instance, runtime.ID())
	reserved := next(func(runview.RunsData) bool { return true })
	if len(reserved.Items) != 0 {
		t.Fatalf("unaccepted reservation published: %+v", reserved)
	}
	close(continueOpen)
	store, err := <-opened, <-openErr
	if err != nil {
		t.Fatal(err)
	}
	if got, live := runtime.Registry().Live(id); !live || got != store {
		t.Fatal("accepted observation was not the reserved store")
	}
	published := next(func(data runview.RunsData) bool { return len(data.Items) == 1 })
	if published.Items[0].Run.ID != id || published.Items[0].Run.Name != "publication" || published.Items[0].Run.Status != observation.StatusRunning {
		t.Fatalf("accepted publication inventory: %+v", published)
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
	completed := next(func(data runview.RunsData) bool {
		return len(data.Items) == 1 && data.Items[0].Run.Status == observation.StatusCompleted
	})
	if completed.Items[0].Run.ID != id || completed.Items[0].Summary != "Completed" {
		t.Fatalf("completed publication inventory: %+v", completed)
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
	entries, err := os.ReadDir(filepath.Join(project, ".gimbal", "runs"))
	if err != nil {
		t.Fatal(err)
	}
	var hostedID string
	for _, entry := range entries {
		if entry.Name() != id && runstore.ValidID(entry.Name()) {
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
	hostedInventory := next(func(data runview.RunsData) bool { return len(data.Items) == 2 })
	if hostedInventory.Items[0].Run.ID != hostedID || hostedInventory.Items[0].Run.Name != "hosted" || hostedInventory.Items[0].Run.Status != observation.StatusRunning {
		t.Fatalf("hosted inventory during body: %+v", hostedInventory)
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
	next(func(data runview.RunsData) bool {
		return len(data.Items) == 2 && data.Items[0].Run.Status == observation.StatusCompleted && data.Items[1].Run.Status == observation.StatusCompleted
	})
	cancel()
	<-instance.done
	restartCtx, stop := context.WithCancel(t.Context())
	defer stop()
	restarted, err := NewInstance(restartCtx, instanceDir, nil, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := restarted.Owner.AdmitProject(project)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { restarted.cancel(nil); restarted.Wait() })
	for _, runID := range []string{id, hostedID} {
		saved, err := reopened.Registry().Snapshot(runID)
		if err != nil || saved.Run.Status != observation.StatusCompleted {
			t.Fatalf("reopened %s = %+v, %v", runID, saved.Run, err)
		}
	}
	reopenedInventory := watchTestRuns(t, restarted, reopened.ID())(func(data runview.RunsData) bool { return len(data.Items) == 2 })
	for _, item := range reopenedInventory.Items {
		if item.Run.Status != observation.StatusCompleted || item.Run.ID != id && item.Run.ID != hostedID {
			t.Fatalf("reopened inventory: %+v", reopenedInventory)
		}
	}
}
