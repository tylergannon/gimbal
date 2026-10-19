package taskdaemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/observation"
)

type gapHarness struct {
	testHarness
	release chan struct{}
}

func (h *gapHarness) RunTurn(ctx context.Context, id, _ string, _ json.RawMessage, emit func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	_ = emit(gimbal.AgentEvent{Type: "session.execution.started", Data: json.RawMessage(`{"sessionID":"native"}`), NativeRef: json.RawMessage(`{"provider":"test","sessionID":"native"}`)})
	select {
	case <-h.release:
		return gimbal.TurnResult{Output: json.RawMessage(`"done"`)}, nil
	case <-ctx.Done():
		return gimbal.TurnResult{}, ctx.Err()
	}
}
func TestEventGapSurvivesRuntimeStampAndRecoversWithoutStoppingTurn(t *testing.T) {
	native := &gapHarness{release: make(chan struct{}), steered: make(chan string, 1)}
	d := New(t.TempDir())
	d.adapter = func(string) (gimbal.HarnessAdapter, error) { return native, nil }
	var offline atomic.Bool
	offline.Store(true)
	handler := d.Handler()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tasks/events" && offline.Load() {
			http.Error(w, "events unavailable", http.StatusServiceUnavailable)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	project := t.TempDir()
	registry := observation.NewRegistry(project)
	ctx := observation.WithRegistry(gimbal.Project(context.Background(), project), registry)
	ctx = effects.WithEndpoint(ctx, server.URL)
	sessions := make(chan *gimbal.Session, 1)
	done := make(chan error, 1)
	go func() {
		done <- gimbal.Run(ctx, "01ARZ3NDEKTSV4RRFFQ69G5FA0", "gap", map[gimbal.WorkflowRole]gimbal.ModelBinding{"agent": {Adapter: &Harness{Endpoint: server.URL, Name: "test"}, Model: "test"}}, func(ctx context.Context) error {
			s := gimbal.NewSession(ctx, "agent", t.TempDir())
			sessions <- s
			value, err := s.Generate[gimbal.Text](ctx, "work")
			if err == nil && value != "done" {
				t.Errorf("result %q", value)
			}
			return err
		})
	}()
	session := <-sessions
	waitFor := func(unavailable bool) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			snapshot, err := registry.Snapshot("01ARZ3NDEKTSV4RRFFQ69G5FA0")
			if err == nil {
				for _, turn := range snapshot.Turns {
					if turn.EventsUnavailable == unavailable && (unavailable || turn.Started != 0) {
						return
					}
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("missing events unavailable=%v projection", unavailable)
	}
	waitFor(true)
	if ok, err := session.Steer(context.Background(), "keep working"); err != nil || !ok {
		t.Fatalf("control during event outage: %v %v", ok, err)
	}
	if got := <-native.steered; got != "keep working" {
		t.Fatal(got)
	}
	offline.Store(false)
	waitFor(false)
	close(native.release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("successful result blocked by events")
	}
	snapshot, err := registry.Snapshot("01ARZ3NDEKTSV4RRFFQ69G5FA0")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Run.Status != observation.StatusCompleted {
		t.Fatalf("execution status %s", snapshot.Run.Status)
	}
}
