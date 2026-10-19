package environments

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/backend/messagefiles"
	"github.com/tylergannon/gimbal/internal/taskdaemon"
)

type readinessOrchestrator struct {
	mu     sync.Mutex
	owners map[backend.Owner]bool
}

func (o *readinessOrchestrator) AcquireEnvironment(_ context.Context, owner backend.Owner, _ string) (backend.Environment, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.owners[owner] = true
	return backend.Environment{ID: "actual-worker-id"}, nil
}
func (o *readinessOrchestrator) Release(_ context.Context, scope backend.Scope) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	for owner := range o.owners {
		if owner.Run == scope.Run && backend.WithinScope(scope.Scope, owner.Scope) {
			delete(o.owners, owner)
		}
	}
	return nil
}
func (o *readinessOrchestrator) ActiveEnvironments(context.Context) ([]backend.EnvironmentDestination, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.owners) == 0 {
		return nil, nil
	}
	return []backend.EnvironmentDestination{{ID: "actual-worker-id"}}, nil
}
func (o *readinessOrchestrator) Destinations(_ context.Context, r backend.MessageRequest) ([]backend.EnvironmentDestination, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.owners[r.Owner] {
		return []backend.EnvironmentDestination{{ID: "actual-worker-id"}}, nil
	}
	return nil, nil
}
func TestProvisionedProbeBecomesIdleWithoutHidingAnotherOwner(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	store, e := messagefiles.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	bus := &helperMessages{Store: store, ctx: ctx}
	bus.handler = taskdaemon.New(t.TempDir()).Messages(bus)
	orchestrator := &readinessOrchestrator{owners: map[backend.Owner]bool{}}
	registry := &gimbal.Registry{}
	if e = registry.RegisterBackend(gimbal.BackendDefinition{Name: "provisioned-test", ConfigurationSchema: json.RawMessage(`{}`), ValidateConfiguration: func(json.RawMessage) error { return nil }, Connect: func(context.Context, json.RawMessage) (backend.Connection, error) {
		return backend.Connection{Messages: bus, Orchestrator: orchestrator}, nil
	}}); e != nil {
		t.Fatal(e)
	}
	s, e := New(ctx, Options{InstanceDir: t.TempDir(), Registry: registry})
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = s.Close() }()
	route := backend.Route{Name: "provisioned-test", Environment: "configured-placement"}
	machine := Machine{ID: "worker", Name: "Worker", Kind: KindRoute, Route: &route, Executable: os.Args[0]}
	target := Target{Machine: machine, Route: route, Directory: repository(t)}
	cfg, _ := json.Marshal(Config{Version: 1, Machines: []Machine{machine}, Environments: []Environment{}})
	if _, e = s.SaveConfig(ctx, ConfigEdit{Text: string(cfg)}); e != nil {
		t.Fatal(e)
	}
	state := func() MachineState {
		snapshot, e := s.Snapshot(ctx, "")
		if e != nil {
			t.Fatal(e)
		}
		for _, m := range snapshot.Machines {
			if m.ID == machine.ID {
				return m
			}
		}
		t.Fatal("machine disappeared")
		return MachineState{}
	}
	if _, e = s.Inspect(ctx, target, target.Directory); e != nil {
		t.Fatal(e)
	}
	if got := state(); got.State != StateNotStarted || len(got.Harnesses) == 0 {
		t.Fatalf("released probe advertised a ready worker or lost observed facts: %+v", got)
	}
	parent := backend.Owner{Run: "conversation", Scope: "conversation", Task: "session"}
	binding, e := s.Acquire(ctx, target, parent)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Inspect(ctx, target, target.Directory); e != nil {
		t.Fatal(e)
	}
	if got := state(); got.State != StateConnected {
		t.Fatalf("probe cleanup hid a worker retained by another owner: %+v", got)
	}
	if e = s.Release(ctx, binding); e != nil {
		t.Fatal(e)
	}
	if got := state(); got.State != StateNotStarted {
		t.Fatalf("last owner's release retained connected status: %+v", got)
	}
}
