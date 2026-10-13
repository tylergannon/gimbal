package localorchestrator

import (
	"bytes"
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
cat > "$(dirname "$ready")/bootstrap.json"
trap 'exit 0' INT
printf ready > "$ready"
while :; do sleep 1; done
`), 0700)
	if err != nil {
		t.Fatal(err)
	}
	config := Config{Directory: dir, Environments: map[string]EnvironmentCommand{"dev": {Command: []string{script}}}}
	controller, err := Open(config, backend.Selection{Name: "custom-shared", Configuration: json.RawMessage(`{"Endpoint":"private-bootstrap-value"}`)})
	if err != nil {
		t.Fatal(err)
	}
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
	rawBootstrap, err := os.ReadFile(filepath.Join(dir, "bootstrap.json"))
	if err != nil {
		t.Fatal(err)
	}
	var received backend.Selection
	if err := json.Unmarshal(rawBootstrap, &received); err != nil {
		t.Fatal(err)
	}
	if received.Name != "custom-shared" || !bytes.Equal(received.Configuration, json.RawMessage(`{"Endpoint":"private-bootstrap-value"}`)) {
		t.Fatalf("selected bootstrap lost: %s", rawBootstrap)
	}
	log, err := os.ReadFile(controller.file("dev") + ".log")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(log, []byte("private-bootstrap-value")) {
		t.Fatal("bootstrap leaked to daemon log")
	}
	first, _ := controller.load("dev")
	independent := &Orchestrator{config: config}
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
