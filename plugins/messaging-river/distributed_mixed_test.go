package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/web"
)

// The application and both separately started workers must already load the
// matching binaries and fixture plugin. This test never starts or stops them.
func TestDistributedMixedOwnedParticipants(t *testing.T) {
	if os.Getenv("GIMBAL_DISTRIBUTED_MIXED_TEST") != "1" {
		t.Skip("set GIMBAL_DISTRIBUTED_MIXED_TEST=1 with owned application and workers")
	}
	required := func(name string) string {
		t.Helper()
		v := os.Getenv(name)
		if v == "" {
			t.Fatalf("external setup required: %s", name)
		}
		if !filepath.IsAbs(v) {
			t.Fatalf("external setup: %s must be absolute", name)
		}
		return v
	}
	instance := required("GIMBAL_DISTRIBUTED_INSTANCE")
	project := required("GIMBAL_DISTRIBUTED_PROJECT")
	routePath := required("GIMBAL_DISTRIBUTED_ROUTE")
	remoteDir := required("GIMBAL_DISTRIBUTED_REMOTE_DIR")
	raw, err := os.ReadFile(routePath)
	if err != nil {
		t.Fatalf("external setup: read route: %v", err)
	}
	var route backend.Route
	if err := json.Unmarshal(raw, &route); err != nil {
		t.Fatalf("external setup: route must contain one route object: %v", err)
	}
	if route.Name != "river" || route.Environment != "" {
		t.Fatal("external setup: route must select river without a physical environment")
	}
	cfg, err := parse(route.Configuration)
	if err != nil {
		t.Fatalf("external setup: invalid River configuration: %v", err)
	}
	if cfg.Environments["workflow-local"] != "worker-a" || cfg.Environments["task-remote"] != "worker-b" {
		t.Fatal("external setup: route must bind workflow-local=worker-a and task-remote=worker-b")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
	defer cancel()
	bus, err := open(ctx, cfg)
	if err != nil {
		t.Fatalf("external setup: River unavailable: %v", err)
	}
	defer bus.Close()
	input, _ := json.Marshal(map[string]string{"remote_dir": remoteDir})
	accepted, err := web.Start(ctx, instance, invocation.Request{Workflow: "distributed-mixed-test", Input: input, Project: project, WorkDir: project, TaskRoute: route})
	if err != nil {
		t.Fatalf("external setup: application must be running with matching registered fixture and River plugins: %v", err)
	}
	t.Logf("run=%s model=luna", accepted.RunID)
	selection := backend.MessageSelection{Run: accepted.RunID}
	stopObserved := errors.New("observer checkpoint")
	// Disconnect an independent subscriber after local completion, then replay
	// from the beginning while the remote session continues.
	err = bus.ListenEvents(ctx, selection, time.Time{}, func(event backend.MessageEvent) error {
		if event.WorkerID == "worker-a" && event.Kind == "completed" && event.Result != nil && event.Result.Process != nil {
			return stopObserved
		}
		return nil
	})
	if !errors.Is(err, stopObserved) {
		t.Fatalf("local command observation: %v", err)
	}
	followed := make(chan error, 1)
	go func() {
		row, err := web.Follow(ctx, instance, project, accepted.RunID)
		if err == nil && row.Status != "completed" {
			err = fmt.Errorf("run %s: %s", row.Status, row.Error)
		}
		followed <- err
	}()
	seen := map[string]backend.MessageEvent{}
	observeCtx, stop := context.WithCancel(ctx)
	observed := make(chan error, 1)
	go func() {
		observed <- bus.ListenEvents(observeCtx, selection, time.Time{}, func(event backend.MessageEvent) error { seen[event.ID] = event; return nil })
	}()
	err = <-followed
	stop()
	listenErr := <-observed
	if err != nil {
		t.Fatalf("workflow: %v", err)
	}
	if listenErr != nil && !errors.Is(listenErr, context.Canceled) {
		t.Fatalf("observer: %v", listenErr)
	}
	// Replay once more after completion, asserting the independent subscriber
	// retained its original identities and that both workers admitted real work.
	local, remote, turns := false, false, map[string]bool{}
	replayed := 0
	sessions := map[string]bool{}
	replayEnd := errors.New("replayed")
	replayCtx, replayCancel := context.WithTimeout(ctx, 3*time.Second)
	defer replayCancel()
	err = bus.ListenEvents(replayCtx, selection, time.Time{}, func(e backend.MessageEvent) error {
		if previous, ok := seen[e.ID]; ok {
			if previous.RequestID != e.RequestID || previous.Source != e.Source || previous.Sequence != e.Sequence {
				return fmt.Errorf("event identity changed: %s", e.ID)
			}
			replayed++
		}
		if e.Kind == "accepted" && e.WorkerID == "worker-a" && e.Environment == "workflow-local" {
			local = true
		}
		if e.Kind == "accepted" && e.WorkerID == "worker-b" && e.Environment == "task-remote" {
			remote = true
		}
		if e.Kind == "completed" && e.WorkerID == "worker-b" && e.Result != nil && len(e.Result.Output) > 0 {
			turns[e.RequestID] = true
			if e.SessionID != "" {
				sessions[e.SessionID] = true
			}
		}
		if local && remote && len(turns) >= 2 && replayed > 0 {
			return replayEnd
		}
		return nil
	})
	if !errors.Is(err, replayEnd) {
		t.Fatalf("replay missing admitted workers or two remote outputs (local=%t remote=%t turns=%d): %v", local, remote, len(turns), err)
	}
	if len(sessions) != 1 {
		t.Fatalf("remote turn session identities: %v", sessions)
	}
	t.Logf("replayed %d observed event identities; two remote turn outputs arrived through River", replayed)
}
