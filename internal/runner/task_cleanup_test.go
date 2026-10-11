package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/observation"
)

func TestExitedCleanupRecordsFailureThenSuccessfulManualRetry(t *testing.T) {
	state := t.TempDir()
	id := "lost"
	dir := filepath.Join(state, "runs", id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	var pending atomic.Int32
	pending.Store(1)
	var fail atomic.Bool
	fail.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tasks/state":
			_ = json.NewEncoder(w).Encode(effects.State{Pending: int(pending.Load())})
		case "/tasks/cleanup":
			if fail.Load() {
				_ = json.NewEncoder(w).Encode(effects.Reply{Error: "service could not stop"})
			} else {
				pending.Store(0)
				_ = json.NewEncoder(w).Encode(effects.Reply{})
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	for name, content := range map[string]string{
		"task-owner":  server.URL,
		"runner.lock": "",
		"run.jsonl":   "{\"seq\":1,\"time\":\"2026-10-10T00:00:00Z\",\"scope\":\"\",\"event\":{\"kind\":\"run_started\",\"name\":\"lost\"}}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if done, err := CleanupExited(state, id); done || err == nil {
		t.Fatalf("failed cleanup: done=%v err=%v", done, err)
	}
	check := func(want string) {
		t.Helper()
		snapshot, err := observation.NewRegistry(state).Snapshot(id)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Run.Status != observation.StatusFailed || !snapshot.Run.Cancellation.Present || snapshot.Run.Cancellation.Value.Cleanup != want {
			t.Fatalf("cleanup %s: %+v", want, snapshot.Run)
		}
	}
	check("unconfirmed")
	fail.Store(false)
	if _, err := orphanCall(context.Background(), server.URL, id, operation{Action: "scope"}); err != nil {
		t.Fatal(err)
	}
	if done, err := CleanupExited(state, id); !done || err != nil {
		t.Fatalf("completed cleanup: done=%v err=%v", done, err)
	}
	check("completed")
}
