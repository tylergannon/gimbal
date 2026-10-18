package taskdaemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/effects"
)

func TestSuccessfulResultDespiteEventOutage(t *testing.T) {
	d := New()
	handler := d.Handler()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tasks/events" {
			http.Error(w, "offline", http.StatusServiceUnavailable)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	req := processRequest(t, "run", "", "task", "printf done", false)
	if err := d.process(req); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	raw, err := effects.Wait(ctx, server.URL, effects.Target{Run: "run", ID: "task"}, func(json.RawMessage) {})
	if err != nil || len(raw) == 0 {
		t.Fatalf("result %s: %v", raw, err)
	}
	if err = d.cleanup(ctx, effects.Scope{Run: "run"}); err != nil {
		t.Fatal(err)
	}
}
func TestBridgeRetentionPreservesExecutorFiles(t *testing.T) {
	d := New()
	req := processRequest(t, "run", "", "task", "printf bytes", false)
	if err := d.process(req); err != nil {
		t.Fatal(err)
	}
	task := d.tasks[key("run", "task")]
	<-task.done
	output := task.captures["stdout"]
	task.mu.Lock()
	task.completed = time.Now().Add(-6 * time.Minute)
	task.mu.Unlock()
	d.mu.Lock()
	d.expireLocked(time.Now())
	d.mu.Unlock()
	if task.expired {
		t.Fatal("expired unreleased ownership")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := d.cleanup(ctx, effects.Scope{Run: "run"}); err != nil {
		t.Fatal(err)
	}
	if p := d.poll(effects.Poll{Run: "run", ID: "task"}); p.Error != "task payload expired" {
		t.Fatalf("missing expired response %+v", p)
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != "bytes" {
		t.Fatalf("executor output retired with bridge projection: %q %v", data, err)
	}
}
