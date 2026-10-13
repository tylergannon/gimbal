package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"

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

type cleanupMessages struct {
	context context.Context
	closed  *atomic.Int32
	request backend.MessageRequest
}

func (b *cleanupMessages) Close() error { b.closed.Add(1); return nil }
func (b *cleanupMessages) PublishRequest(_ context.Context, r backend.MessageRequest) error {
	if err := b.context.Err(); err != nil {
		return err
	}
	b.request = r
	return nil
}
func (b *cleanupMessages) PublishEvent(context.Context, backend.MessageEvent) error { return nil }
func (b *cleanupMessages) ListenEvents(_ context.Context, _ backend.MessageSelection, _ time.Time, f func(backend.MessageEvent) error) error {
	return f(backend.MessageEvent{RequestID: b.request.ID, Kind: "completed", Result: &backend.MessageResult{}})
}
func TestMessagingOrphanCleanupOwnsConnectionUntilOperationEnds(t *testing.T) {
	state := t.TempDir()
	id := "orphan"
	dir := filepath.Join(state, "runs", id)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	var opened, closed, constructed atomic.Int32
	name := fmt.Sprintf("cleanup-%d", time.Now().UnixNano())
	err := gimbal.DefaultRegistry.RegisterSecrets(gimbal.SecretsDefinition{Name: name, ValidateConfiguration: func(json.RawMessage) error { return nil }, Construct: func(context.Context, json.RawMessage) (backend.SecretsBackend, error) {
		constructed.Add(1)
		return backend.NewMemorySecrets(), nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	err = gimbal.DefaultRegistry.RegisterMessaging(gimbal.MessagingDefinition{Name: name, ValidateConfiguration: func(json.RawMessage) error { return nil }, Connect: func(ctx context.Context, _ json.RawMessage, resources backend.Dependencies) (backend.Messaging, error) {
		if resources.Secrets == nil {
			return nil, fmt.Errorf("cleanup connection lost its dependency")
		}
		opened.Add(1)
		return &cleanupMessages{context: ctx, closed: &closed}, nil
	}, Worker: func(context.Context, json.RawMessage, backend.Dependencies, backend.MessageHandler) error {
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	route, _ := json.Marshal(backend.Route{Name: name, Backends: &backend.BackendSelection{Secrets: backend.Selection{Name: name}}})
	if err = os.WriteFile(filepath.Join(dir, "task-route"), route, 0600); err != nil {
		t.Fatal(err)
	}
	c := remote{state: state, runID: id}
	for range 2 {
		pending, err := c.call(context.Background(), operation{Action: "cleanup"})
		if err != nil || pending {
			t.Fatalf("cleanup pending=%v err=%v", pending, err)
		}
	}
	if opened.Load() != 2 || closed.Load() != 2 || constructed.Load() != 2 {
		t.Fatalf("opened=%d closed=%d", opened.Load(), closed.Load())
	}
}
