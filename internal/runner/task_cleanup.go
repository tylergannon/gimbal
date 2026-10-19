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
	"github.com/tylergannon/gimbal/internal/runstate"
	"github.com/tylergannon/gimbal/runstore"
	"golang.org/x/sys/unix"
)

// CleanupExited uses the runner's lifetime lock, never a lost connection, as
// evidence that the workflow can no longer release its remote resources.
func cleanupExitedLegacy(state, id string) (bool, error) {
	dir := filepath.Join(state, "runs", id)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	connection, endpoint, _, err := taskConnection(ctx, state, id)
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
	cleanupErr := effects.Cleanup(ctx, endpoint, id, "", backend.NewExecutionIdentity("control").RequestID)
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
			err := effects.Cleanup(ctx, endpoint, run, "", backend.NewExecutionIdentity("control").RequestID)
			return err != nil, err
		}
		var state effects.State
		err := effects.Call(ctx, endpoint, "state", effects.Scope{Run: run}, &state)
		return err != nil || state.Pending > 0, err
	case "scope":
		return false, effects.Cleanup(ctx, endpoint, run, op.Target, backend.NewExecutionIdentity("control").RequestID)
	default:
		return false, fmt.Errorf("workflow process is unavailable; scope cleanup remains reachable")
	}
}

func taskConnection(ctx context.Context, state, id string) (backend.Connection, string, backend.Route, error) {
	dir := filepath.Join(state, "runs", id)
	raw, err := os.ReadFile(filepath.Join(dir, "task-route"))
	if err == nil {
		var route backend.Route
		if err := json.Unmarshal(raw, &route); err != nil {
			return backend.Connection{}, "", backend.Route{}, err
		}
		connection, err := gimbal.DefaultRegistry.Connect(ctx, route)
		return connection, "", route, err
	}
	if !errors.Is(err, os.ErrNotExist) {
		return backend.Connection{}, "", backend.Route{}, err
	}
	endpoint, err := os.ReadFile(filepath.Join(dir, "task-owner"))
	return backend.Connection{}, string(endpoint), effects.Route(effects.WithEndpoint(ctx, string(endpoint))), err
}

func CleanupExited(state, id string) (bool, error) {
	if !runstate.Recognized(filepath.Join(state, "runs", id)) {
		return cleanupExitedLegacy(state, id)
	}
	return cleanupStopped(state, id, nil, runstate.ForProject(state))
}

// cleanupStopped owns all new-format orphan mutations: writer first, then the
// pre-existing runner liveness lock, then one cleanup dispatch and projections.
func cleanupStopped(state, id string, op *operation, factory runstore.Store) (settled bool, retErr error) {
	if id == "" || filepath.Base(id) != id || id == "." || id == ".." {
		return false, fmt.Errorf("invalid run identity")
	}
	dir := filepath.Join(state, "runs", id)
	if op == nil && observation.ReleasedProjection(dir) {
		return true, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r, err := factory.Open(ctx, runstore.RunID(id))
	if err != nil {
		if op == nil && (errors.Is(err, runstore.ErrBusy) || errors.Is(err, runstore.ErrIncomplete)) {
			return false, nil
		}
		return false, err
	}
	defer func() { retErr = errors.Join(retErr, r.Close()) }()
	lock, err := os.OpenFile(filepath.Join(dir, "runner.lock"), os.O_RDWR, 0)
	if errors.Is(err, os.ErrNotExist) {
		if op != nil {
			return false, fmt.Errorf("no runner for this run")
		}
		return true, nil
	}
	if err != nil {
		return false, err
	}
	defer lock.Close()
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if op != nil {
			return false, runstore.ErrBusy
		}
		return false, nil
	}
	summary, err := runstate.Inspect(ctx, r)
	if err != nil {
		return false, err
	}
	target := ""
	if op != nil {
		if op.Action != "scope" {
			return false, fmt.Errorf("orphan supports only scope cleanup")
		}
		target = op.Target
	}
	parent := summary.Header.Root
	if target != "" {
		n, ok := summary.Nodes[target]
		if !ok || n.Kind != runstore.Scope {
			return false, fmt.Errorf("unknown scope %q", target)
		}
		parent = n.ID
	}
	connection, endpoint, route, err := taskConnection(ctx, state, id)
	defer connection.Close()
	noRoute := errors.Is(err, os.ErrNotExist) || (err == nil && endpoint == "" && connection.Execution == nil && connection.Messages == nil)
	if noRoute {
		if target != "" {
			return false, fmt.Errorf("orphan native resources cannot be reconstructed")
		}
		if err = observation.RecordTerminatedOwned(ctx, r, dir, "workflow process exited; native resource cleanup is unconfirmed"); err != nil {
			return false, err
		}
		if err = observation.RecordCleanupOwned(ctx, r, dir, errors.New("native resource cleanup is unconfirmed")); err != nil {
			return false, err
		}
		if op != nil {
			return false, fmt.Errorf("native resource cleanup is unconfirmed")
		}
		return true, nil
	}
	if err != nil {
		return false, err
	}
	ctx = effects.WithRoute(effects.WithEndpoint(effects.WithConnection(ctx, connection), endpoint), route)
	var action runstore.Node
	if op == nil {
		action, err = r.EnsureNode(ctx, runstore.NodeSpec{Slot: runstore.Slot{Parent: parent, Kind: runstore.Action, Name: "cleanup-episode", Ordinal: 1}, DisplayKey: target + "/cleanup-episode.1"})
	} else {
		action, err = runstate.NextAction(ctx, r, parent, "cleanup", target)
	}
	if err != nil {
		return false, err
	}
	identity := backend.ExecutionIdentity{ScopeID: string(parent), ActionID: string(action.ID), RequestID: string(action.ID)}
	if !summary.Intents[action.ID] {
		intent, marshalErr := json.Marshal(map[string]any{"operation": "cleanup", "target": target, "episode_id": action.ID, "route": effects.Route(ctx), "identity": identity})
		if marshalErr != nil {
			return false, marshalErr
		}
		binding, marshalErr := json.Marshal(identity)
		if marshalErr != nil {
			return false, marshalErr
		}
		if _, err = r.Append(ctx, []runstore.Fact{
			{Version: 1, ID: runstore.NewRecordID(), Node: action.ID, Scope: parent, Kind: "intent", Payload: intent},
			{Version: 1, ID: runstore.NewRecordID(), Node: action.ID, Scope: parent, Kind: "binding", Payload: binding},
		}, runstore.Committed); err != nil {
			return false, err
		}
	}
	if !summary.Outcomes[action.ID] {
		cleanupErr := effects.Cleanup(ctx, endpoint, id, target, identity.RequestID)
		current := runstate.CleanupState{Status: "released"}
		if cleanupErr != nil {
			current.Status = "pending"
			current.Error = cleanupErr.Error()
		}
		_, recordErr := runstate.UpdateCleanup(ctx, r, action.ID, current)
		if op != nil || cleanupErr == nil {
			recordErr = errors.Join(recordErr, runstate.Append(ctx, r, action.ID, parent, "outcome", current))
		}
		if target == "" {
			recordErr = errors.Join(recordErr, observation.RecordTerminatedOwned(ctx, r, dir, "workflow process exited; remote cleanup was requested"), observation.RecordCleanupOwned(ctx, r, dir, cleanupErr))
		}
		return cleanupErr == nil, errors.Join(cleanupErr, recordErr)
	}
	if target == "" {
		return true, observation.RecordCleanupOwned(ctx, r, dir, nil)
	}
	return true, nil
}
