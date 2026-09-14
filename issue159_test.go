package gimble

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimble/internal/runlog"
)

// caught runs f and returns the value it panicked with, or nil.
func caught(f func()) (v any) {
	defer func() { v = recover() }()
	f()
	return nil
}

// terminal reads dir's run.jsonl and returns its last two lifecycle
// records, which a complete run ends with, exactly once each: run_ended,
// then complete.
func terminal(t *testing.T, dir string) (RunEnded, Complete) {
	t.Helper()
	var kinds []string
	var ended RunEnded
	var complete Complete
	if err := runlog.Read[LifecycleRecord](t.Context(), dir, func(e LifecycleRecord) error {
		kinds = append(kinds, lifecycleKind(e.Event))
		switch event := e.Event.(type) {
		case RunEnded:
			ended = event
		case Complete:
			complete = event
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	n := len(kinds)
	if n < 2 || kinds[n-2] != "run_ended" || kinds[n-1] != "complete" {
		t.Fatalf("run.jsonl ends %q, want run_ended then complete", kinds)
	}
	if strings.Count(strings.Join(kinds, " "), "run_ended") != 1 || strings.Count(strings.Join(kinds, " "), "complete") != 1 {
		t.Fatalf("run.jsonl has %q, want one run_ended and one complete", kinds)
	}
	return ended, complete
}

func TestSetMisusePanics(t *testing.T) {
	if got := caught(func() { Set(t.Context(), "goal", "x") }); got != `gimble: set "goal": no scope in the ctx; it must come from gimble.Run` {
		t.Errorf("Set with no scope panicked with %v", got)
	}
	err := runTest(t, func(ctx context.Context) error {
		var ended context.Context
		err := Scope(ctx, "delivery", func(ctx context.Context) error {
			ended = ctx
			Set(ctx, "task", "first")
			if got := caught(func() { Set(ctx, "task", "second") }); got != `gimble: "task" is already set in scope "delivery.1"` {
				t.Errorf("a second Set of a key panicked with %v", got)
			}
			if got := caught(func() { SetJSON(ctx, "task", review{}) }); got != `gimble: "task" is already set in scope "delivery.1"` {
				t.Errorf("SetJSON of a set key panicked with %v", got)
			}
			return nil
		})
		if err != nil {
			return err
		}
		if got := caught(func() { Set(ended, "late", "x") }); got != `gimble: set "late" in scope "delivery.1" after it ended` {
			t.Errorf("Set on an ended scope panicked with %v", got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPanicInScopeBodyIsRecordedThenEscapes(t *testing.T) {
	var dir string
	got := caught(func() {
		_ = runTest(t, func(ctx context.Context) error {
			dir = runDir(ctx)
			NewSession(ctx, "worker", &fake{}, "m", t.TempDir())
			return Scope(ctx, "delivery", func(ctx context.Context) error {
				Set(ctx, "task", "first")
				Set(ctx, "task", "second")
				return nil
			})
		})
	})
	want := `gimble: "task" is already set in scope "delivery.1"`
	if got != want {
		t.Fatalf("Run panicked with %v, want %q", got, want)
	}
	ended, complete := terminal(t, dir)
	if ended.Error != "gimble: panic: "+want {
		t.Errorf("RunEnded.Error = %q", ended.Error)
	}
	if complete.RecordingError != "" {
		t.Errorf("Complete.RecordingError = %q", complete.RecordingError)
	}
}

func TestPanicInGroupChildCancelsSiblingsAndEscapesRun(t *testing.T) {
	var dir string
	var waited error
	got := caught(func() {
		_ = runTest(t, func(ctx context.Context) error {
			dir = runDir(ctx)
			group := Group(ctx, "candidates")
			group.Go("candidate", func(ctx context.Context) error {
				Set(ctx, "task", "first")
				Set(ctx, "task", "second")
				return nil
			})
			group.Go("candidate", func(ctx context.Context) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(5 * time.Second):
					return errors.New("the sibling was not cancelled")
				}
			})
			waited = group.Wait()
			return fmt.Errorf("wrapped: %w", waited)
		})
	})
	want := `gimble: "task" is already set in scope "candidates.1/candidate.1"`
	if waited == nil || !strings.Contains(waited.Error(), want) || !strings.Contains(waited.Error(), "goroutine ") {
		t.Fatalf("Wait returned %v, want the panic text and its stack", waited)
	}
	if got == nil || !strings.Contains(fmt.Sprint(got), want) {
		t.Fatalf("Run panicked with %v, want %q", got, want)
	}
	ended, complete := terminal(t, dir)
	if !strings.Contains(ended.Error, want) {
		t.Errorf("RunEnded.Error = %q", ended.Error)
	}
	if complete.RecordingError != "" {
		t.Errorf("Complete.RecordingError = %q", complete.RecordingError)
	}
}
