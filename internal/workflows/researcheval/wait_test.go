package researcheval

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/live"
	"github.com/tylergannon/gimbal/internal/observation"

	"github.com/tylergannon/gimbal/runstore"
)

func TestWaitResearchFollowsChildAndCancelsByIdentity(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		name := "completion"
		if timeout {
			name = "timeout"
		}
		t.Run(name, func(t *testing.T) {
			state := t.TempDir()
			started := make(chan string, 1)
			finish := make(chan struct{})
			done := make(chan error, 1)
			var controller live.Controller
			ctx := live.WithHook(gimbal.Project(t.Context(), state), func(id string, c live.Controller) func() {
				controller = c
				started <- id
				return func() {}
			})
			go func() {
				done <- gimbal.Run(ctx, string(runstore.NewRunID()), "child", nil, func(ctx context.Context) error {
					select {
					case <-finish:
						return nil
					case <-ctx.Done():
						return context.Cause(ctx)
					}
				})
			}()
			id := <-started
			reader := observation.NewRegistry(state)
			initial, err := reader.Snapshot(id)
			if err != nil || initial.Run.Status != observation.StatusRunning {
				t.Fatalf("initial: %+v, %v", initial.Run, err)
			}
			controls := live.NewRuns()
			resolved := false
			controls.Resolve = func(got string) (live.Controller, error) {
				if got != id {
					t.Fatalf("resolved %q instead of %q", got, id)
				}
				resolved = true
				return controller, nil
			}
			waitCtx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			defer cancel()
			if !timeout {
				close(finish)
			}
			snapshot, err := waitResearch(waitCtx, reader, controls, id)
			if timeout {
				if !errors.Is(err, context.DeadlineExceeded) || !resolved || snapshot.Run.Status == observation.StatusRunning {
					t.Fatalf("timeout: %+v, %v, resolved=%v", snapshot.Run, err, resolved)
				}
			} else if err != nil || snapshot.Run.Status != observation.StatusCompleted {
				t.Fatalf("completion: %+v, %v", snapshot.Run, err)
			}
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("child did not settle")
			}
		})
	}
}

func TestWaitResearchReportsUnavailableCancellation(t *testing.T) {
	state := t.TempDir()
	dir := filepath.Join(state, "runs", "child")
	writer, err := observation.Open(nil, "child", "child", dir, func() error { return os.MkdirAll(dir, 0755) })
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = writer.Close() }()
	controls := live.NewRuns()
	unavailable := errors.New("controller unavailable")
	controls.Resolve = func(string) (live.Controller, error) { return nil, unavailable }
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = waitResearch(ctx, observation.NewRegistry(state), controls, "child")
	if !errors.Is(err, unavailable) {
		t.Fatalf("cancellation error: %v", err)
	}
}
