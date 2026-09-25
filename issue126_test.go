package gimbal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/runlog"
)

func issue126RunDir(t *testing.T, project string) string {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		entries, _ := os.ReadDir(filepath.Join(project, "runs"))
		if len(entries) == 1 {
			dir := filepath.Join(project, "runs", entries[0].Name())
			if _, err := os.Stat(filepath.Join(dir, "run.jsonl")); err == nil {
				return dir
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("run log was not created")
		}
		time.Sleep(time.Millisecond)
	}
}

// TestIssue126CompletionContract makes external ownership explicit: every
// started Run is joined, but reading its record remains independent from its
// verdict. The reader is deliberately outside the body because Complete is
// written only after that body returns.
func TestIssue126CompletionContract(t *testing.T) {
	t.Run("reader follows a successful live run and both finish", func(t *testing.T) {
		project := t.TempDir()
		release := make(chan struct{})
		var runErr error
		var runWG sync.WaitGroup
		runWG.Go(func() {
			runErr = Run(Project(t.Context(), project), "observed", nil, func(context.Context) error {
				<-release
				return nil
			})
		})

		var complete bool
		readErr := runlog.Read[LifecycleRecord](t.Context(), issue126RunDir(t, project), func(record LifecycleRecord) error {
			switch record.Event.(type) {
			case RunStarted:
				close(release)
			case Complete:
				complete = true
			}
			return nil
		})
		runWG.Wait()
		if readErr != nil || runErr != nil || !complete {
			t.Fatalf("Read = %v, Run = %v, complete = %v", readErr, runErr, complete)
		}
	})

	t.Run("reader failure does not replace worker failure", func(t *testing.T) {
		project := t.TempDir()
		release := make(chan struct{})
		workerErr := errors.New("worker failed")
		readerErr := errors.New("reader failed")
		var runErr error
		var runWG sync.WaitGroup
		runWG.Go(func() {
			runErr = Run(Project(t.Context(), project), "worker-failure", nil, func(context.Context) error {
				<-release
				return workerErr
			})
		})

		readErr := runlog.Read[LifecycleRecord](t.Context(), issue126RunDir(t, project), func(LifecycleRecord) error {
			return readerErr
		})
		close(release)
		runWG.Wait()
		if !errors.Is(readErr, readerErr) || !errors.Is(runErr, workerErr) {
			t.Fatalf("Read = %v, Run = %v; want independent reader and worker failures", readErr, runErr)
		}
	})

	t.Run("ending observation does not cancel execution", func(t *testing.T) {
		project := t.TempDir()
		release := make(chan struct{})
		var runErr error
		var runWG sync.WaitGroup
		runWG.Go(func() {
			runErr = Run(Project(t.Context(), project), "observer-exit", nil, func(context.Context) error {
				<-release
				return nil
			})
		})

		readCtx, stopReading := context.WithCancel(t.Context())
		readErr := runlog.Read[LifecycleRecord](readCtx, issue126RunDir(t, project), func(LifecycleRecord) error {
			stopReading()
			return nil
		})
		close(release)
		runWG.Wait()
		if !errors.Is(readErr, context.Canceled) || runErr != nil {
			t.Fatalf("Read = %v, Run = %v; reader cancellation must not change the run verdict", readErr, runErr)
		}
	})

	t.Run("cancelling execution returns only after its run joins", func(t *testing.T) {
		project := t.TempDir()
		runCtx, cancel := context.WithCancel(t.Context())
		var runErr error
		var runWG sync.WaitGroup
		runWG.Go(func() {
			runErr = Run(Project(runCtx, project), "cancelled", nil, func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			})
		})
		_ = issue126RunDir(t, project)
		cancel()
		runWG.Wait()
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("Run = %v, want context.Canceled after joining", runErr)
		}
	})
}
