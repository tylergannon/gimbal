package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
)

func TestWindowsSCMHandler(t *testing.T) {
	for _, control := range []svc.Cmd{svc.Stop, svc.Shutdown} {
		t.Run(map[svc.Cmd]string{svc.Stop: "stop", svc.Shutdown: "shutdown"}[control], func(t *testing.T) {
			requests := make(chan svc.ChangeRequest)
			changes := make(chan svc.Status, 8)
			ready := make(chan struct{})
			handler := &workerService{run: func(ctx context.Context, reportReady func()) error {
				<-ready
				reportReady()
				<-ctx.Done()
				return nil
			}}
			done := make(chan struct{})
			go func() {
				specific, code := handler.Execute(nil, requests, changes)
				if specific || code != 0 {
					t.Errorf("stop exit: specific=%v code=%d", specific, code)
				}
				close(done)
			}()
			next := func(want svc.State) svc.Status {
				t.Helper()
				select {
				case status := <-changes:
					if status.State != want {
						t.Fatalf("state=%v want=%v", status.State, want)
					}
					return status
				case <-time.After(5 * time.Second):
					t.Fatal("SCM handler stalled")
					return svc.Status{}
				}
			}
			if status := next(svc.StartPending); status.Accepts != 0 || status.WaitHint == 0 {
				t.Fatal(status)
			}
			requests <- svc.ChangeRequest{Cmd: svc.Interrogate}
			next(svc.StartPending)
			close(ready)
			if status := next(svc.Running); status.Accepts != svc.AcceptStop|svc.AcceptShutdown {
				t.Fatal(status)
			}
			requests <- svc.ChangeRequest{Cmd: control}
			if status := next(svc.StopPending); status.Accepts != 0 {
				t.Fatal(status)
			}
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("SCM handler failed to return")
			}
		})
	}
}

func TestWindowsSCMStartupFailureExitCode(t *testing.T) {
	handler := &workerService{run: func(context.Context, func()) error { return errors.New("startup failed") }}
	changes := make(chan svc.Status, 8)
	specific, code := handler.Execute(nil, make(chan svc.ChangeRequest), changes)
	if !specific || code != 1 {
		t.Fatalf("failure exit: specific=%v code=%d", specific, code)
	}
	for len(changes) > 0 {
		if status := <-changes; status.State == svc.Running {
			t.Fatal("startup failure reported Running")
		}
	}
}

func TestWindowsUserEnvironmentImport(t *testing.T) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer key.Close()
	name := fmt.Sprintf("GIMBAL_WORKER_TEST_%d", time.Now().UnixNano())
	if err := key.SetStringValue(name, "synthetic-import"); err != nil {
		t.Fatal(err)
	}
	defer key.DeleteValue(name)
	value, err := userEnvironment(name)
	if err != nil || value != "synthetic-import" {
		t.Fatal("user registry import failed")
	}
	// Expanded user values are rejected rather than silently interpreted against
	// an unrelated SCM environment. Supply resolved paths in set instead.
	if err := key.SetExpandStringValue(name, "%USERPROFILE%\\bin"); err != nil {
		t.Fatal(err)
	}
	if _, err := userEnvironment(name); err == nil {
		t.Fatal("REG_EXPAND_SZ imported as literal")
	}
	if err := key.DeleteValue(name); err != nil {
		t.Fatal(err)
	}
	if _, err := userEnvironment(name); err == nil {
		t.Fatal("missing user import accepted")
	}
}
