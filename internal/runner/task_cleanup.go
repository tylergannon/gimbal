package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"

	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/observation"
	"golang.org/x/sys/unix"
)

// CleanupExited uses the runner's lifetime lock, never a lost connection, as
// evidence that the workflow can no longer release its remote resources.
func CleanupExited(state, id string) (bool, error) {
	dir := filepath.Join(state, "runs", id)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	connection, endpoint, err := taskConnection(ctx, state, id)
	defer func() { _ = connection.Close() }()
	if errors.Is(err, os.ErrNotExist) || (err == nil && endpoint == "" && connection.Execution == nil && connection.Messages == nil) {
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
	ctx = effects.WithConnection(ctx, connection)
	var owned effects.State
	if connection.Messages == nil {
		if e := effects.Call(ctx, endpoint, "state", effects.Scope{Run: id}, &owned); e == nil && owned.Pending == 0 {
			if err := observation.RecordTerminated(dir, "workflow process exited; remote resources are released"); err != nil {
				return false, err
			}
			return true, observation.RecordCleanup(dir, nil)
		}
	}
	cleanupErr := effects.Cleanup(ctx, endpoint, id, "")
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
		if effects.Messages(ctx) != nil {
			err := effects.Cleanup(ctx, endpoint, run, "")
			return err != nil, err
		}
		var state effects.State
		err := effects.Call(ctx, endpoint, "state", effects.Scope{Run: run}, &state)
		return err != nil || state.Pending > 0, err
	case "scope":
		return false, effects.Cleanup(ctx, endpoint, run, op.Target)
	default:
		return false, fmt.Errorf("workflow process is unavailable; scope cleanup remains reachable")
	}
}

func taskConnection(ctx context.Context, state, id string) (backend.Connection, string, error) {
	dir := filepath.Join(state, "runs", id)
	raw, err := os.ReadFile(filepath.Join(dir, "task-route"))
	if err == nil {
		var route backend.Route
		if err := json.Unmarshal(raw, &route); err != nil {
			return backend.Connection{}, "", err
		}
		connection, err := gimbal.DefaultRegistry.Connect(ctx, route)
		return connection, "", err
	}
	if !errors.Is(err, os.ErrNotExist) {
		return backend.Connection{}, "", err
	}
	endpoint, err := os.ReadFile(filepath.Join(dir, "task-owner"))
	return backend.Connection{}, string(endpoint), err
}
