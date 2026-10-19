package environments

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/backend/messagefiles"
	"github.com/tylergannon/gimbal/internal/taskdaemon"
)

func TestConnectSSHObservesTheTunnelBeforeReportingHealthAndHelperFacts(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\ncase \"$*\" in *'command -v'*) printf '%s\\n' " + shellQuote(os.Args[0]) + "; exit 0;; *'-N -L'*) exec " + shellQuote(os.Args[0]) + " --test-ssh-tunnel \"$@\";; esac\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	daemon := taskdaemon.New(t.TempDir())
	handler := daemon.Handler()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(200)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	address, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(address.Port())
	s := testService(t)
	doc, _ := s.Config(context.Background())
	progress, err := s.ConnectMachine(context.Background(), MachineEdit{Revision: doc.Revision, Machine: Machine{ID: "ssh", Name: "SSH box", Kind: KindSSH, SSH: &SSH{Host: "test-box", DaemonPort: port}, Executable: os.Args[0]}})
	if err != nil || !progress.Complete || !progress.CanConnect {
		t.Fatalf("real TCP tunnel and helper did not complete: %+v %v", progress, err)
	}
	ids := []string{}
	for _, check := range progress.Checks {
		ids = append(ids, check.ID)
		if check.State == "pending" || check.Fatal {
			t.Fatalf("completed connection retained an unobserved check: %+v", check)
		}
	}
	if !slices.Equal(ids, []string{"sign-in", "executable", "daemon", "health", "inspection", "harnesses", "build"}) {
		t.Fatalf("SSH checks appeared in the wrong order: %v", ids)
	}
	if progress.Checks[2].State != "completed" || progress.Checks[3].State != "completed" {
		t.Fatalf("tunnel/health were not observed: %+v", progress)
	}
}

func TestConnectSSHStopsAtTheActualFatalCheckWithoutSaving(t *testing.T) {
	for _, fail := range []string{"sign-in", "executable"} {
		t.Run(fail, func(t *testing.T) {
			dir := t.TempDir()
			log := filepath.Join(dir, "commands")
			script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + shellQuote(log) + "\n"
			if fail == "sign-in" {
				script += "printf 'Permission denied' >&2\nexit 255\n"
			} else {
				script += "case \"$*\" in *'command -v'*) printf 'not installed' >&2; exit 1;; esac\nexit 0\n"
			}
			if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			s := testService(t)
			doc, _ := s.Config(context.Background())
			machine := Machine{ID: "build", Name: "Build box", Kind: KindSSH, SSH: &SSH{Host: "build-box"}}
			progress, err := s.ConnectMachine(context.Background(), MachineEdit{Machine: machine, Revision: doc.Revision})
			if err == nil || !progress.Complete || progress.CanConnect || progress.Saved {
				t.Fatalf("fatal failure allowed persistence: %+v %v", progress, err)
			}
			last := progress.Checks[len(progress.Checks)-1]
			if last.ID != fail || !last.Fatal || last.State != "failed" || last.Fix == "" || last.Consequence == "" {
				t.Fatalf("failure was not explained at its observed line: %+v", progress)
			}
			commands, _ := os.ReadFile(log)
			if strings.Contains(string(commands), "nohup") || fail == "sign-in" && strings.Contains(string(commands), "command -v") {
				t.Fatalf("checks continued after fatal failure: %s", commands)
			}
			if _, err := s.ConnectMachine(context.Background(), MachineEdit{Machine: machine, Revision: doc.Revision, ConnectAnyway: true, Attempt: progress.Attempt}); err == nil {
				t.Fatal("fatal check could be bypassed by Connect anyway")
			}
			after, _ := s.Config(context.Background())
			if after.Revision != doc.Revision || after.Text != doc.Text {
				t.Fatal("fatal connection changed configuration")
			}
		})
	}
}

func TestConnectAddressReportsObservedChecksAndRequiresMatchingWarningConfirmation(t *testing.T) {
	ctx := context.Background()
	daemon := taskdaemon.New(t.TempDir())
	handler := daemon.Handler()
	var inspections atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(200)
			return
		}
		if r.URL.Path == "/tasks/identified-process" {
			inspections.Add(1)
		}
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	s := testService(t)
	doc, _ := s.Config(ctx)
	machine := Machine{ID: "box", Name: "Box", Kind: KindAddress, Address: server.URL, Executable: os.Args[0]}
	progress, err := s.ConnectMachine(ctx, MachineEdit{Machine: machine, Revision: doc.Revision})
	if err != nil || progress.Saved || !progress.Complete || !progress.CanConnect {
		t.Fatalf("warnings did not pause before save: %+v %v", progress, err)
	}
	ids := []string{}
	for _, check := range progress.Checks {
		ids = append(ids, check.ID)
		if check.State == "pending" || check.Fatal {
			t.Fatalf("successful inspection retained incomplete/fatal check: %+v", check)
		}
	}
	if !slices.Equal(ids, []string{"health", "inspection", "harnesses", "build"}) {
		t.Fatalf("address invented SSH/start observations or lost check order: %v", ids)
	}
	if inspections.Load() != 1 {
		t.Fatalf("expected one actual helper inspection, got %d", inspections.Load())
	}
	changed := machine
	changed.Address += "/other"
	if _, err := s.ConnectMachine(ctx, MachineEdit{Machine: changed, Revision: doc.Revision, ConnectAnyway: true, Attempt: progress.Attempt}); err == nil {
		t.Fatal("warning confirmation accepted a different machine configuration")
	}
	afterChecks, _ := s.Config(ctx)
	if afterChecks.Revision != doc.Revision {
		t.Fatal("checks saved configuration before confirmation")
	}
	progress, err = s.ConnectMachine(ctx, MachineEdit{Machine: machine, Revision: doc.Revision, ConnectAnyway: true, Attempt: progress.Attempt})
	if err != nil || !progress.Saved || inspections.Load() != 1 {
		t.Fatalf("confirmation did not save the already checked machine: %+v %v; inspections=%d", progress, err, inspections.Load())
	}
	after, _ := s.Config(ctx)
	if !strings.Contains(after.Text, server.URL) {
		t.Fatal("confirmed machine was not saved")
	}
	progress.Checks[0].State = "tampered"
	if s.Connection(machine.ID).Checks[0].State == "tampered" {
		t.Fatal("a returned progress slice can mutate the authoritative checks")
	}
}

func TestConnectionProgressExposesOnlyChecksActuallyStarted(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(503)
	}))
	defer server.Close()
	s := testService(t)
	doc, _ := s.Config(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = s.ConnectMachine(context.Background(), MachineEdit{Machine: Machine{ID: "box", Name: "Box", Kind: KindAddress, Address: server.URL}, Revision: doc.Revision})
	}()
	<-entered
	progress := s.Connection("box")
	if progress.Complete || len(progress.Checks) != 1 || progress.Checks[0].ID != "health" || progress.Checks[0].State != "pending" {
		t.Fatalf("progress invented later checks: %+v", progress)
	}
	close(release)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("failed health check did not stop connection")
	}
	progress = s.Connection("box")
	if !progress.Complete || len(progress.Checks) != 1 || !progress.Checks[0].Fatal || progress.Checks[0].State != "failed" {
		t.Fatalf("health failure did not stop at its actual check: %+v", progress)
	}
}

func TestRouteConnectionReportsWorkerFailureWithoutSSHOrDaemonHealth(t *testing.T) {
	s := testService(t)
	machine := Machine{ID: "queue", Name: "Queue", Kind: KindRoute, Route: &backend.Route{Name: "missing-worker", Configuration: json.RawMessage(`{}`)}}
	if err := s.checkConnection(context.Background(), machine); err == nil {
		t.Fatal("missing route worker reported connected")
	}
	progress := s.Connection(machine.ID)
	if len(progress.Checks) != 1 || progress.Checks[0].ID != "inspection" || !progress.Checks[0].Fatal || !strings.Contains(progress.Checks[0].Fix, "environments.json") {
		t.Fatalf("route failure invented daemon/SSH checks or lost its fix: %+v", progress)
	}
}

func TestRouteConnectionUsesActualMessagingHelperFacts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	store, err := messagefiles.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bus := &helperMessages{Store: store, ctx: ctx}
	bus.handler = taskdaemon.New(t.TempDir()).Messages(bus)
	registry := &gimbal.Registry{}
	if err := registry.RegisterMessaging(gimbal.MessagingDefinition{ExecutionIdentity: true, Name: "connect-messages", ValidateConfiguration: func(json.RawMessage) error { return nil }, Connect: func(context.Context, json.RawMessage, backend.Dependencies) (backend.Messaging, error) {
		return bus, nil
	}, Worker: func(context.Context, json.RawMessage, backend.Dependencies, backend.MessageHandler) error { return nil }}); err != nil {
		t.Fatal(err)
	}
	s, err := New(ctx, Options{InstanceDir: t.TempDir(), Registry: registry})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	machine := Machine{ID: "queue", Name: "Queue", Kind: KindRoute, Route: &backend.Route{Name: "connect-messages"}, Executable: os.Args[0]}
	if err := s.checkConnection(ctx, machine); err != nil {
		t.Fatal(err)
	}
	progress := s.Connection(machine.ID)
	ids := []string{}
	for _, check := range progress.Checks {
		ids = append(ids, check.ID)
	}
	if !progress.Complete || !slices.Equal(ids, []string{"inspection", "harnesses", "build"}) || bus.processes.Load() != 1 {
		t.Fatalf("route did not report its one real helper operation: %+v, processes=%d", progress, bus.processes.Load())
	}
	if !strings.Contains(progress.Checks[0].Cause, "helper returned") || len(s.machineState(machine).Harnesses) == 0 {
		t.Fatalf("route inspection discarded actual worker facts: %+v", progress)
	}
}

func TestWarningConfirmationRequiresTheExactAttemptAndRevision(t *testing.T) {
	ctx := t.Context()
	handler := taskdaemon.New(t.TempDir()).Handler()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(200)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	s := testService(t)
	doc, _ := s.Config(ctx)
	machine := Machine{ID: "box", Name: "Box", Kind: KindAddress, Address: server.URL, Executable: os.Args[0]}
	first, err := s.ConnectMachine(ctx, MachineEdit{Machine: machine, Revision: doc.Revision})
	if err != nil || !first.CanConnect {
		t.Fatalf("initial checks: %+v %v", first, err)
	}
	second, err := s.ConnectMachine(ctx, MachineEdit{Machine: machine, Revision: doc.Revision})
	if err != nil || second.Attempt == first.Attempt {
		t.Fatalf("fresh checks reused attempt: %+v %v", second, err)
	}
	for _, attempt := range []string{"", first.Attempt} {
		if _, err := s.ConnectMachine(ctx, MachineEdit{Machine: machine, Revision: doc.Revision, Attempt: attempt, ConnectAnyway: true}); err == nil {
			t.Fatal("accepted missing or replaced attempt")
		}
	}
	// Even an unrelated file edit needs a new check. Supplying the new revision
	// cannot bless the old attempt, and supplying the old one cannot overwrite it.
	updated, err := s.SaveConfig(ctx, ConfigEdit{Revision: doc.Revision, Text: `{"version":1,"machines":[],"environments":[],"plugin":{"opaque":true}}`})
	if err != nil {
		t.Fatal(err)
	}
	for _, revision := range []string{doc.Revision, updated.Revision} {
		if _, err := s.ConnectMachine(ctx, MachineEdit{Machine: machine, Revision: revision, Attempt: second.Attempt, ConnectAnyway: true}); err == nil {
			t.Fatal("accepted a confirmation across a config edit")
		}
	}
	after, _ := s.Config(ctx)
	if after.Text != updated.Text {
		t.Fatal("rejected confirmation changed the configuration")
	}
}

func TestConnectionSerializesConcurrentAttemptsAndConfirmsOnlyOnce(t *testing.T) {
	ctx := t.Context()
	entered, release := make(chan struct{}), make(chan struct{})
	handler := taskdaemon.New(t.TempDir()).Handler()
	var healthCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			if healthCalls.Add(1) == 1 {
				close(entered)
				<-release
			}
			w.WriteHeader(200)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	s := testService(t)
	doc, _ := s.Config(ctx)
	machine := Machine{ID: "box", Name: "Box", Kind: KindAddress, Address: server.URL, Executable: os.Args[0]}
	done := make(chan ConnectionProgress, 1)
	go func() {
		p, _ := s.ConnectMachine(ctx, MachineEdit{Machine: machine, Revision: doc.Revision})
		done <- p
	}()
	<-entered
	if _, err := s.ConnectMachine(ctx, MachineEdit{Machine: machine, Revision: doc.Revision}); err == nil {
		t.Fatal("concurrent Connect was admitted")
	}
	if err := s.checkConnection(ctx, machine); err == nil {
		t.Fatal("concurrent Reconnect was admitted")
	}
	close(release)
	progress := <-done
	if !progress.CanConnect {
		t.Fatalf("check failed: %+v", progress)
	}
	edit := MachineEdit{Machine: machine, Revision: doc.Revision, Attempt: progress.Attempt, ConnectAnyway: true}
	if p, err := s.ConnectMachine(ctx, edit); err != nil || !p.Saved {
		t.Fatalf("confirmation: %+v %v", p, err)
	}
	saved, _ := s.Config(ctx)
	edit.Revision = saved.Revision
	if _, err := s.ConnectMachine(ctx, edit); err == nil {
		t.Fatal("one confirmation saved twice")
	}
	after, _ := s.Config(ctx)
	if after.Revision != saved.Revision {
		t.Fatal("duplicate confirmation rewrote file")
	}
	var config Config
	if err := json.Unmarshal([]byte(after.Text), &config); err != nil || len(config.Machines) != 1 {
		t.Fatalf("not exactly one machine: %s %v", after.Text, err)
	}
}
