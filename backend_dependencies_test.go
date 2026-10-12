package gimbal

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/backend"
)

func TestEnvironmentOwnershipReadinessAndScopedRelease(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "worker.sh")
	err := os.WriteFile(script, []byte(`#!/bin/sh
while [ "$#" -gt 0 ]; do
 case "$1" in --ready-file) ready="$2"; shift 2;; *) shift;; esac
done
trap 'exit 0' INT
printf ready > "$ready"
while :; do sleep 1; done
`), 0700)
	if err != nil {
		t.Fatal(err)
	}
	config := LocalBackends{Directory: dir, Environments: map[string]EnvironmentCommand{"dev": {Command: []string{script}}}}
	raw, _ := json.Marshal(config)
	deps, err := (&Registry{}).constructLocalBackends(raw)
	if err != nil {
		t.Fatal(err)
	}
	controller := deps.Orchestrator.(*localOrchestrator)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	a := backend.Owner{Run: "a", Scope: "root/child", Task: "session"}
	b := backend.Owner{Run: "b", Scope: "root", Task: "process"}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = controller.Release(ctx, backend.Scope{Run: "a"})
		_ = controller.Release(ctx, backend.Scope{Run: "b"})
	})
	if _, err := controller.AcquireEnvironment(ctx, a, "dev"); err != nil {
		t.Fatal(err)
	}
	first, _ := controller.load("dev")
	independent := &localOrchestrator{config: config}
	if _, err := independent.AcquireEnvironment(ctx, b, "dev"); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.AcquireEnvironment(ctx, a, "dev"); err != nil {
		t.Fatal(err)
	}
	shared, _ := controller.load("dev")
	if shared.PID != first.PID || len(shared.Owners) != 2 {
		t.Fatalf("ownership lost: %+v", shared)
	}
	names, err := controller.assigned(backend.MessageRequest{Run: "a", Scope: "root/child/nested", Task: "turn", SessionID: "session", Kind: "turn"})
	if err != nil || len(names) != 1 || names[0] != "dev" {
		t.Fatalf("child turn assignment %v %v", names, err)
	}
	cleanup := backend.MessageRequest{Run: "a", Scope: "root/child/nested", Kind: "cleanup"}
	names, err = controller.assigned(cleanup)
	if err != nil || len(names) != 1 || names[0] != "dev" {
		t.Fatalf("child scope cleanup missed ancestor session daemon: %v %v", names, err)
	}
	if err := controller.Release(ctx, backend.Scope{Run: "a", Scope: "root"}); err != nil {
		t.Fatal(err)
	}
	shared, _ = controller.load("dev")
	if shared.PID != first.PID || len(shared.Owners) != 1 || shared.Owners[0] != b {
		t.Fatalf("unrelated use released: %+v", shared)
	}
	if err := independent.Release(ctx, backend.Scope{Run: "b"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(controller.file("dev")); !os.IsNotExist(err) {
		t.Fatal("stopped environment receipt retained", err)
	}
}

type cleanupEventSink struct {
	backend.Messaging
	event backend.MessageEvent
}

func (s *cleanupEventSink) PublishEvent(_ context.Context, e backend.MessageEvent) error {
	s.event = e
	return nil
}
func TestParentCleanupAfterLastEnvironmentRelease(t *testing.T) {
	sink := &cleanupEventSink{}
	bus := &environmentMessaging{base: sink, controller: &localOrchestrator{config: LocalBackends{Directory: t.TempDir()}}}
	request := backend.MessageRequest{ID: "cleanup", Kind: "cleanup", Run: "run", ScopeTarget: &backend.Scope{Run: "run"}}
	if err := bus.PublishRequest(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if sink.event.RequestID != request.ID || sink.event.Kind != "completed" || sink.event.Result == nil || sink.event.Result.Unresolved {
		t.Fatalf("parent cleanup did not confirm no remaining native ownership: %+v", sink.event)
	}
}
