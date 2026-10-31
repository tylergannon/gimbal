package main

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/backend"
)

func nextServiceState(t *testing.T, states <-chan serviceState, want serviceState) {
	t.Helper()
	select {
	case got := <-states:
		if got != want {
			t.Fatalf("state %v, want %v", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("missing lifecycle transition")
	}
}
func serviceResult(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("service did not exit")
		return nil
	}
}

func TestServiceReadinessControlsAndJoinedStop(t *testing.T) {
	for _, control := range []serviceControl{serviceStop, serviceShutdown} {
		t.Run(map[serviceControl]string{serviceStop: "stop", serviceShutdown: "shutdown"}[control], func(t *testing.T) {
			controls := make(chan serviceControl)
			states := make(chan serviceState, 8)
			ready, release := make(chan struct{}), make(chan struct{})
			done := make(chan error, 1)
			go func() {
				done <- serviceLoop(func(ctx context.Context, reportReady func()) error {
					<-ready
					reportReady()
					<-ctx.Done()
					<-release
					return nil
				}, controls, func(state serviceState) { states <- state })
			}()
			nextServiceState(t, states, serviceStarting)
			// Interrogating before readiness must still report StartPending.
			controls <- serviceInterrogate
			nextServiceState(t, states, serviceStarting)
			close(ready)
			nextServiceState(t, states, serviceRunning)
			controls <- serviceInterrogate
			nextServiceState(t, states, serviceRunning)
			controls <- control
			nextServiceState(t, states, serviceStopping)
			controls <- serviceInterrogate
			nextServiceState(t, states, serviceStopping)
			select {
			case <-done:
				t.Fatal("returned before HTTP host stopped")
			default:
			}
			close(release)
			if err := serviceResult(t, done); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestServiceStopDuringStartupNeverReportsRunning(t *testing.T) {
	controls := make(chan serviceControl)
	states := make(chan serviceState, 8)
	done := make(chan error, 1)
	go func() {
		done <- serviceLoop(func(ctx context.Context, ready func()) error {
			<-ctx.Done()
			ready() // A late readiness callback must not resurrect the service.
			return ctx.Err()
		}, controls, func(state serviceState) { states <- state })
	}()
	nextServiceState(t, states, serviceStarting)
	controls <- serviceStop
	nextServiceState(t, states, serviceStopping)
	if err := serviceResult(t, done); err != nil {
		t.Fatal(err)
	}
	select {
	case state := <-states:
		t.Fatalf("late state %v", state)
	default:
	}
}

func TestServicePropagatesStartupAndRuntimeFailures(t *testing.T) {
	for _, started := range []bool{false, true} {
		failure := errors.New("synthetic worker failure")
		var states []serviceState
		err := serviceLoop(func(_ context.Context, ready func()) error {
			if started {
				ready()
			}
			return failure
		}, make(chan serviceControl), func(state serviceState) { states = append(states, state) })
		if !errors.Is(err, failure) {
			t.Fatal("lost host failure")
		}
		if states[0] != serviceStarting || states[len(states)-1] != serviceStopping {
			t.Fatal(states)
		}
		if !started && len(states) != 2 {
			t.Fatal("startup failure reported Running")
		}
	}
	if err := serviceLoop(func(context.Context, func()) error { return nil }, make(chan serviceControl), func(serviceState) {}); err == nil {
		t.Fatal("unexpected host exit accepted")
	}
}

func TestWorkerStartupFailuresNeverReadyAndReleaseLease(t *testing.T) {
	workspace := t.TempDir()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	for _, reason := range []string{"listener", "state", "environment", "relative service paths"} {
		t.Run(reason, func(t *testing.T) {
			state := t.TempDir()
			config := workerConfig{workdir: workspace, stateDir: state, address: listener.Addr().String()}
			switch reason {
			case "state":
				directory := filepath.Join(state, "taskdaemon", "sessions")
				if err := os.MkdirAll(directory, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, "broken.json"), []byte(`{`), 0600); err != nil {
					t.Fatal(err)
				}
			case "environment":
				config.envFile = filepath.Join(state, "missing")
			case "relative service paths":
				config.service = true
				config.workdir = "."
			}
			called := false
			err := runWorker(t.Context(), config, io.Discard, func() { called = true })
			if err == nil || called {
				t.Fatalf("startup err=%v ready=%v", err, called)
			}
			// A failure after acquiring the bridge lease must not lock out a repair.
			if reason == "listener" {
				config.address = "127.0.0.1:0"
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				if err := runWorker(ctx, config, io.Discard, cancel); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestServiceStopAndRestartPreservesTask(t *testing.T) {
	workspace, state := t.TempDir(), t.TempDir()
	envFile := environmentFile(t, `{}`)
	start := func() (backend.HTTP, func()) {
		t.Helper()
		controls := make(chan serviceControl)
		states := make(chan serviceState, 8)
		ready := make(chan string, 1)
		done := make(chan error, 1)
		go func() {
			done <- serviceLoop(func(ctx context.Context, reportReady func()) error {
				return runWorker(ctx, workerConfig{address: "127.0.0.1:0", workdir: workspace, stateDir: state, envFile: envFile, service: true}, startupWriter{ready}, reportReady)
			}, controls, func(state serviceState) { states <- state })
		}()
		nextServiceState(t, states, serviceStarting)
		nextServiceState(t, states, serviceRunning)
		message := <-ready
		address := strings.Split(strings.TrimPrefix(message, "Task worker listening on "), ";")[0]
		var once sync.Once
		stop := func() {
			once.Do(func() {
				controls <- serviceShutdown
				nextServiceState(t, states, serviceStopping)
				if err := serviceResult(t, done); err != nil {
					t.Error(err)
				}
			})
		}
		t.Cleanup(stop)
		return backend.HTTP{Endpoint: "http://" + address}, stop
	}
	client, stop := start()
	request := childRequest(t, "service-restart", "wait")
	if err := client.Process(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(workspace, "ready")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("independent task did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	stop()
	client, _ = start()
	if result, err := client.Result(t.Context(), backend.Target{Run: request.Run, ID: request.Task}); err != nil || result.Done {
		t.Fatalf("owner not reattached: %+v %v", result, err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if result := waitResult(t, client, request); result.ExitCode != 0 {
		t.Fatal(result)
	}
	if got := string(readOutput(t, client, request, "stdout")); got != "survived" {
		t.Fatal(got)
	}
}
