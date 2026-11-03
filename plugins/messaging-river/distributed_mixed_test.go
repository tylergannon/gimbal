package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	defer func() {
		if err := bus.Close(); err != nil {
			t.Errorf("close River: %v", err)
		}
	}()
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
	local, remote := false, false
	turns := map[string]string{}
	comparedTurns := map[string]bool{}
	var localOutput []byte
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
			if previous.Result != nil && (e.Result == nil || !bytes.Equal(previous.Result.Output, e.Result.Output) || !bytes.Equal(previous.Result.Stdout, e.Result.Stdout)) {
				return fmt.Errorf("replayed result bytes changed: %s", e.ID)
			}
			if previous.WorkerID == "worker-b" && previous.Kind == "completed" && previous.Result != nil && len(previous.Result.Output) > 0 {
				comparedTurns[e.RequestID] = true
			}
			replayed++
		}
		if e.Kind == "accepted" && e.WorkerID == "worker-a" && e.Environment == "workflow-local" {
			local = true
		}
		if e.Kind == "accepted" && e.WorkerID == "worker-b" && e.Environment == "task-remote" {
			remote = true
		}
		if e.Kind == "completed" && e.WorkerID == "worker-a" && e.Result != nil && e.Result.Process != nil {
			localOutput = append([]byte(nil), e.Result.Stdout...)
		}
		if e.Kind == "completed" && e.WorkerID == "worker-b" && e.Result != nil && len(e.Result.Output) > 0 {
			var output string
			if err := json.Unmarshal(e.Result.Output, &output); err != nil {
				return fmt.Errorf("invalid turn output %s: %w", e.RequestID, err)
			}
			turns[e.RequestID] = strings.TrimSpace(output)
			if e.SessionID != "" {
				sessions[e.SessionID] = true
			}
		}
		if local && remote && len(localOutput) > 0 && len(turns) >= 2 && len(comparedTurns) >= 2 {
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
	parts := strings.Split(strings.TrimSpace(string(localOutput)), "\n")
	if len(parts) != 3 {
		t.Fatalf("invalid replayed local output: %q", localOutput)
	}
	first, followup := false, false
	for _, output := range turns {
		lines := strings.Split(output, "\n")
		if len(lines) == 5 && lines[0] != "" && lines[0] != parts[0] && lines[1] == parts[1] && lines[2] == "buried-"+parts[2] && lines[3] == "file-"+parts[1] && lines[4] == "child-value" {
			first = true
		}
		if output == "file-"+parts[1] {
			followup = true
		}
	}
	if !first || !followup {
		t.Fatalf("replayed turn content does not match local values: first=%t followup=%t", first, followup)
	}
	t.Logf("replayed %d observed event identities; two remote turn outputs arrived through River", replayed)
}
