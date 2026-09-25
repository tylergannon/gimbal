//go:build darwin || linux

package gimbal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestServiceLivesForItsDeclaringScopeAcrossIterations(t *testing.T) {
	workdir := t.TempDir()
	pidFile := filepath.Join(workdir, "pid")
	termFile := filepath.Join(workdir, "term")
	var pids []string
	var dir string
	err := Run(Project(t.Context(), t.TempDir()), "service-scope", nil, func(ctx context.Context) error {
		dir = runDir(ctx)
		return Scope(ctx, "preview", func(ctx context.Context) error {
			command := "print $$ > " + shellQuote(pidFile) + "; trap 'print TERM >> " + shellQuote(termFile) + "; exit 0' TERM; while true; do sleep 0.05; done"
			if err := Service(ctx, "server", workdir, command); err != nil {
				return err
			}
			for range Iterate(ctx, "lap", []int{1, 2}) {
				pids = append(pids, waitFile(t, pidFile))
			}
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pids) != 2 || pids[0] == "" || pids[0] != pids[1] {
		t.Fatalf("service pids across iterations = %v, want the same nonempty pid", pids)
	}
	if got := waitFile(t, termFile); !strings.Contains(got, "TERM") {
		t.Fatalf("termination marker = %q, want TERM", got)
	}
	row := find(commandRows(t, dir), "preview.1/server.1")
	if !row.Interrupted || row.ExitCode != 0 || row.Error != "" {
		t.Fatalf("service row = %+v, want intentional clean shutdown", row)
	}
}

func TestIterationLocalServiceEndsBeforeTheNextIteration(t *testing.T) {
	workdir := t.TempDir()
	var markers []string
	err := runTest(t, nil, func(ctx context.Context) error {
		for itemCtx, item := range Iterate(ctx, "lap", []int{1, 2}) {
			if item == 2 {
				markers = append(markers, waitFile(t, filepath.Join(workdir, "term-1")))
			}
			command := "trap 'print TERM > " + shellQuote(filepath.Join(workdir, "term-"+strconv.Itoa(item))) + "; exit 0' TERM; print ready > " + shellQuote(filepath.Join(workdir, "ready-"+strconv.Itoa(item))) + "; while true; do sleep 0.05; done"
			if err := Service(itemCtx, "server", workdir, command); err != nil {
				return err
			}
			waitFile(t, filepath.Join(workdir, "ready-"+strconv.Itoa(item)))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	markers = append(markers, waitFile(t, filepath.Join(workdir, "term-2")))
	if len(markers) != 2 || strings.TrimSpace(markers[0]) != "TERM" || strings.TrimSpace(markers[1]) != "TERM" {
		t.Fatalf("iteration termination markers = %q, want TERM for each iteration", markers)
	}
}

func TestServiceCancellationStopsTheProcessGroupWithSIGTERM(t *testing.T) {
	workdir := t.TempDir()
	readyFile := filepath.Join(workdir, "ready")
	termFile := filepath.Join(workdir, "term")
	runCtx, cancel := context.WithCancel(t.Context())
	err := Run(Project(runCtx, t.TempDir()), "service-cancel", nil, func(ctx context.Context) error {
		command := "trap 'print TERM > " + shellQuote(termFile) + "; exit 0' TERM; print ready > " + shellQuote(readyFile) + "; while true; do sleep 0.05; done"
		if err := Service(ctx, "server", workdir, command); err != nil {
			return err
		}
		waitFile(t, readyFile)
		cancel()
		<-ctx.Done()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := waitFile(t, termFile); strings.TrimSpace(got) != "TERM" {
		t.Fatalf("termination marker = %q, want TERM", got)
	}
}

func TestDerivedCallContextDoesNotOwnTheServiceLifetime(t *testing.T) {
	workdir := t.TempDir()
	pidFile := filepath.Join(workdir, "pid")
	termFile := filepath.Join(workdir, "term")
	err := runTest(t, nil, func(ctx context.Context) error {
		callCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		command := "print $$ > " + shellQuote(pidFile) + "; trap 'print TERM > " + shellQuote(termFile) + "; exit 0' TERM; while true; do sleep 0.05; done"
		if err := Service(callCtx, "server", workdir, command); err != nil {
			return err
		}
		pid, err := strconv.Atoi(strings.TrimSpace(waitFile(t, pidFile)))
		if err != nil {
			return err
		}
		cancel()
		time.Sleep(100 * time.Millisecond)
		if ctx.Err() != nil {
			return fmt.Errorf("owning scope was cancelled with derived call context: %w", ctx.Err())
		}
		if _, err := os.Stat(termFile); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("service stopped with derived call context: %v", err)
		}
		if err := syscall.Kill(pid, 0); err != nil {
			return fmt.Errorf("service did not survive derived call cancellation: %w", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := waitFile(t, termFile); strings.TrimSpace(got) != "TERM" {
		t.Fatalf("scope-close marker = %q, want TERM", got)
	}
}

func TestUnexpectedServiceExitFailsAndCancelsItsScope(t *testing.T) {
	var cancelled error
	var dir string
	err := Run(Project(t.Context(), t.TempDir()), "service-exit", nil, func(ctx context.Context) error {
		dir = runDir(ctx)
		if err := Service(ctx, "server", "", "print gone; exit 0"); err != nil {
			return err
		}
		<-ctx.Done()
		cancelled = context.Cause(ctx)
		return ctx.Err()
	})
	if err == nil || !strings.Contains(err.Error(), "required service server.1 exited unexpectedly") {
		t.Fatalf("run error = %v, want unexpected required-service exit", err)
	}
	if cancelled == nil || !strings.Contains(cancelled.Error(), "required service server.1 exited unexpectedly") {
		t.Fatalf("scope cancellation cause = %v, want service failure", cancelled)
	}
	row := find(commandRows(t, dir), "server.1")
	if row.ExitCode != 0 || row.Stdout != "gone\n" || row.Interrupted || row.Error == "" {
		t.Fatalf("unexpected service row = %+v", row)
	}
}

func TestUnexpectedIterationServiceExitFailsTheEnclosingScope(t *testing.T) {
	err := runTest(t, nil, func(ctx context.Context) error {
		for itemCtx := range Iterate(ctx, "lap", []int{1, 2}) {
			if err := Service(itemCtx, "server", "", "exit 0"); err != nil {
				return err
			}
			<-itemCtx.Done()
		}
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "required service lap.1/server.1 exited unexpectedly") {
		t.Fatalf("run error = %v, want the iteration service failure", err)
	}
}

func TestServiceStartupFailureIsSynchronous(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	var startErr error
	err := runTest(t, nil, func(ctx context.Context) error {
		startErr = Service(ctx, "server", missing, "sleep 30")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if startErr == nil || !strings.Contains(startErr.Error(), "no such file or directory") {
		t.Fatalf("Service startup error = %v, want missing workdir", startErr)
	}
	if err := Service(t.Context(), "server", "", "sleep 30"); err == nil {
		t.Fatal("Service outside a run succeeded")
	}
}

func TestServiceEscalatesToTheWholeProcessGroup(t *testing.T) {
	oldGrace, oldKillWait := serviceShutdownGrace, serviceKillWait
	serviceShutdownGrace, serviceKillWait = 100*time.Millisecond, 2*time.Second
	defer func() { serviceShutdownGrace, serviceKillWait = oldGrace, oldKillWait }()

	workdir := t.TempDir()
	childFile := filepath.Join(workdir, "child")
	err := runTest(t, nil, func(ctx context.Context) error {
		command := "(trap '' TERM; while true; do sleep 1; done) & print $! > " + shellQuote(childFile) + "; trap 'wait' TERM; wait"
		if err := Service(ctx, "server", workdir, command); err != nil {
			return err
		}
		waitFile(t, childFile)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(waitFile(t, childFile)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		err = syscall.Kill(childPID, 0)
		if errors.Is(err, syscall.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("child pid %d still exists after group escalation: %v", childPID, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitFile(t *testing.T, name string) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(name)
		if err == nil && len(raw) > 0 {
			return string(raw)
		}
		time.Sleep(10 * time.Millisecond)
	}
	raw, err := os.ReadFile(name)
	t.Fatalf("read %s before deadline: %q, %v", name, raw, err)
	return ""
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
