package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/observation"
	"golang.org/x/sys/unix"
)

// CleanupExited uses the runner's lifetime lock, never a lost connection, as
// evidence that the workflow can no longer release its remote resources.
func CleanupExited(state, id string) (bool, error) {
	dir := filepath.Join(state, "runs", id)
	endpoint, err := os.ReadFile(filepath.Join(dir, "task-owner"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	lock, err := os.OpenFile(filepath.Join(dir, "runner.lock"), os.O_RDWR, 0)
	if err != nil {
		return false, err
	}
	defer func() { _ = lock.Close() }()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 11*time.Second)
	defer cancel()
	var owned effects.State
	if e := effects.Call(ctx, string(endpoint), "state", effects.Scope{Run: id}, &owned); e == nil && owned.Pending == 0 {
		if err := observation.RecordTerminated(dir, "workflow process exited; remote resources are released"); err != nil {
			return false, err
		}
		return true, observation.RecordCleanup(dir, nil)
	}
	cleanupErr := effects.Cleanup(ctx, string(endpoint), id, "")
	if err := observation.RecordTerminated(dir, "workflow process exited; remote scope cleanup was requested"); err != nil {
		return false, err
	}
	if err := observation.RecordCleanup(dir, cleanupErr); err != nil {
		return false, err
	}
	return cleanupErr == nil, cleanupErr
}

func orphanCall(ctx context.Context, endpoint, run string, op operation) (bool, error) {
	switch op.Action {
	case "cleanup":
		var state effects.State
		err := effects.Call(ctx, endpoint, "state", effects.Scope{Run: run}, &state)
		return err != nil || state.Pending > 0, err
	case "scope":
		return false, effects.Cleanup(ctx, endpoint, run, op.Target)
	default:
		return false, fmt.Errorf("workflow process is unavailable; scope cleanup remains reachable")
	}
}
