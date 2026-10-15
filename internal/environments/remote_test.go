package environments

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend/messagefiles"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/taskdaemon"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--test-ssh-tunnel" {
		runTunnelFixture(os.Args[2:])
		os.Exit(0)
	}
	if len(os.Args) == 3 && os.Args[1] == "desktop-environment" {
		var request HelperRequest
		if e := json.Unmarshal([]byte(os.Args[2]), &request); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		result, e := RunHelper(context.Background(), request)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		if e = json.NewEncoder(os.Stdout).Encode(result); e != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func TestRemoteHTTPHelperUsesActualDirectoryAndRetainsMutationIdentity(t *testing.T) {
	ctx := context.Background()
	daemon := taskdaemon.New()
	var processes atomic.Int32
	handler := daemon.Handler()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tasks/process" {
			processes.Add(1)
		}
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	s := testService(t)
	dir := repository(t)
	target := Target{Machine: Machine{ID: "remote", Name: "Test remote", Kind: KindAddress, Address: server.URL, Executable: os.Args[0]}, Environment: Environment{ID: "env", Project: "p", Machine: "remote", Directory: dir}, Directory: dir}
	facts, e := s.Inspect(ctx, target, dir)
	if e != nil || !facts.Git {
		t.Fatal(facts, e)
	}
	if facts.Path != dir {
		t.Fatal("inspected wrong machine directory", facts)
	}
	prepared, e := s.Prepare(ctx, target, Checkout{Branch: "remote-work", Base: "main"}, "remote123")
	if e != nil {
		t.Fatal(e)
	}
	count := processes.Load()
	again, e := s.Prepare(ctx, target, Checkout{Branch: "remote-work", Base: "main"}, "remote123")
	if e != nil || again.Directory != prepared.Directory || processes.Load() != count {
		t.Fatal("same identity reran remote mutation", again, e)
	}
	snap, e := s.Snapshot(ctx, "p")
	if e != nil {
		t.Fatal(e)
	}
	_ = snap
	st := s.machineState(target.Machine)
	if len(st.Harnesses) == 0 || st.Build == "" {
		t.Fatal("remote inspection discarded machine metadata", st)
	}
}
func TestRemoteFixedDaemonDirectoryIsReportedAndCannotPrepareElsewhere(t *testing.T) {
	daemon := taskdaemon.New()
	fixed := repository(t)
	daemon.Workdir = fixed
	server := httptest.NewServer(daemon.Handler())
	defer server.Close()
	s := testService(t)
	target := Target{Machine: Machine{ID: "remote", Name: "Remote", Kind: KindAddress, Address: server.URL, Executable: os.Args[0]}, Directory: t.TempDir()}
	d, e := s.Inspect(context.Background(), target, target.Directory)
	if e != nil || d.FixedWorkdir != fixed || d.ExecutionDirectory != fixed {
		t.Fatalf("daemon override concealed: %+v %v", d, e)
	}
	_, e = s.Prepare(context.Background(), target, Checkout{SharedCheckout: true}, "fixed123")
	if e == nil || !strings.Contains(e.Error(), "fixes execution") {
		t.Fatalf("incompatible preparation accepted: %v", e)
	}
}
func TestDirectHTTPRouteRetainsSteering(t *testing.T) {
	c := capabilities(Machine{Kind: KindRoute, Route: &backend.Route{Name: "http"}}, Directory{})
	if !c.Steer {
		t.Fatal("direct HTTP route misidentified as messaging")
	}
}

type helperMessages struct {
	*messagefiles.Store
	handler        backend.MessageHandler
	ctx            context.Context
	cleanupFailure bool
	processes      atomic.Int32
}

func (m *helperMessages) PublishRequest(ctx context.Context, r backend.MessageRequest) error {
	if r.Kind == "process" {
		m.processes.Add(1)
	}
	if r.Kind == "cleanup" && m.cleanupFailure {
		return m.PublishEvent(m.ctx, backend.MessageEvent{ID: ulid.Make().String(), RequestID: r.ID, Created: time.Now().UTC(), Owner: r.Owner, Kind: "completed", Result: &backend.MessageResult{Unresolved: true, Error: "cleanup unavailable"}})
	}
	return m.handler(m.ctx, r)
}
func TestMessagingHelpersReadRetainedStdoutAndKeepCleanupResponsibility(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	store, e := messagefiles.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	bus := &helperMessages{Store: store, ctx: ctx}
	daemon := taskdaemon.New()
	bus.handler = daemon.Messages(bus)
	registry := &gimbal.Registry{}
	if e = registry.RegisterMessaging(gimbal.MessagingDefinition{Name: "test-messages", ValidateConfiguration: func(json.RawMessage) error { return nil }, Connect: func(context.Context, json.RawMessage, backend.Dependencies) (backend.Messaging, error) {
		return bus, nil
	}, Worker: func(context.Context, json.RawMessage, backend.Dependencies, backend.MessageHandler) error { return nil }}); e != nil {
		t.Fatal(e)
	}
	s, e := New(ctx, Options{InstanceDir: t.TempDir(), Registry: registry})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	route := backend.Route{Name: "test-messages"}
	dir := repository(t)
	target := Target{Machine: Machine{ID: "queue", Name: "Queue", Kind: KindRoute, Route: &route, Executable: os.Args[0]}, Directory: dir}
	facts, e := s.Inspect(ctx, target, dir)
	if e != nil || !facts.Git {
		t.Fatal(facts, e)
	}
	p, e := s.Prepare(ctx, target, Checkout{Branch: "message-work", Base: "main"}, "messaging123")
	if e != nil {
		t.Fatal(e)
	}
	count := bus.processes.Load()
	again, e := s.Prepare(ctx, target, Checkout{Branch: "message-work", Base: "main"}, "messaging123")
	if e != nil || p.Directory != again.Directory || count != bus.processes.Load() {
		t.Fatal("messaging retry repeated preparation", again, e)
	}
	bus.cleanupFailure = true
	if _, e = s.Inspect(ctx, target, dir); e == nil || !strings.Contains(e.Error(), "cleanup unavailable") {
		t.Fatalf("cleanup failure discarded: %v", e)
	}
	if len(s.pendingIssues()) == 0 {
		t.Fatal("failed probe cleanup was not retained")
	}
	bus.cleanupFailure = false
	s.retryProbeCleanup()
	if len(s.pendingIssues()) != 0 {
		t.Fatal("same owned cleanup did not recover")
	}
}

type failingProvisioner struct{ released atomic.Int32 }

func (*failingProvisioner) AcquireEnvironment(context.Context, backend.Owner, string) (backend.Environment, error) {
	return backend.Environment{}, fmt.Errorf("readiness failed after allocation")
}
func (f *failingProvisioner) Release(context.Context, backend.Scope) error {
	f.released.Add(1)
	return nil
}
func TestFailedReadinessReleasesOnlyItsRetainedProbeOwnership(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store, e := messagefiles.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	bus := &helperMessages{Store: store, ctx: ctx}
	bus.handler = taskdaemon.New().Messages(bus)
	provisioner := &failingProvisioner{}
	registry := &gimbal.Registry{}
	if e = registry.RegisterBackend(gimbal.BackendDefinition{Name: "failed-ready", ConfigurationSchema: json.RawMessage(`{}`), ValidateConfiguration: func(json.RawMessage) error { return nil }, Connect: func(context.Context, json.RawMessage) (backend.Connection, error) {
		return backend.Connection{Messages: bus, Orchestrator: provisioner}, nil
	}}); e != nil {
		t.Fatal(e)
	}
	s, e := New(ctx, Options{InstanceDir: t.TempDir(), Registry: registry})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	route := backend.Route{Name: "failed-ready"}
	_, e = s.Inspect(ctx, Target{Machine: Machine{ID: "q", Name: "Q", Kind: KindRoute, Route: &route}}, "/")
	if e == nil || provisioner.released.Load() != 1 {
		t.Fatalf("failed-readiness ownership lost: %v, released=%d", e, provisioner.released.Load())
	}
}
