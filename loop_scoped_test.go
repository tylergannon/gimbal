package gimble

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
)

func TestLoopIterationsLifecycleAndOuterSession(t *testing.T) {
	adapter := &fake{answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
		return "outer remains usable", nil
	}}
	var dir string
	err := runTest(t, bind(adapter, "model", "outer", "inner"), func(ctx context.Context) error {
		dir = runDir(ctx)
		outer := NewSession(ctx, "outer", ".")
		if _, err := outer.Generate[Text](ctx, "prime outer"); err != nil {
			return err
		}
		loop := Loop(ctx, "work")
		count := 0
		var child *Session
		for iterationCtx := range loop.Iterations {
			count++
			child = NewSession(iterationCtx, "inner", ".")
			if _, err := child.Generate[Text](iterationCtx, "prime child"); err != nil {
				return err
			}
			if count == 2 && len(adapter.closed) != 1 {
				t.Fatalf("closed sessions before second body = %v, want first child", adapter.closed)
			}
			if _, err := current(iterationCtx); err != nil {
				return err
			}
			if count == 1 {
				continue
			}
			break
		}
		if count != 2 {
			t.Fatalf("iterations = %d, want two", count)
		}
		if err := loop.Err(); err != nil {
			return err
		}
		if _, err := child.Generate[Text](ctx, "closed child"); err == nil {
			t.Fatal("saved child session remained usable after break")
		}
		if len(adapter.closed) != 2 {
			t.Fatalf("closed sessions before outer resumes = %v, want two children", adapter.closed)
		}
		_, err := outer.Generate[Text](ctx, "still usable")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if adapter.made != 3 {
		t.Fatalf("sessions created = %d, want outer plus two children", adapter.made)
	}
	if len(adapter.closed) != 3 {
		t.Fatalf("closed sessions after run cleanup = %v, want outer plus two children", adapter.closed)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, "scopes", "work.1", "backlog.md")); err != nil {
		t.Fatal(err)
	} else if len(matches) != 0 {
		t.Fatalf("raw iterations wrote planner backlog %v", matches)
	}
}

func TestLoopIterationsPanicClosesChild(t *testing.T) {
	adapter := &fake{answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
		return "done", nil
	}}
	err := runTest(t, bind(adapter, "model", "worker"), func(ctx context.Context) error {
		loop := Loop(ctx, "work")
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("iteration panic was not raised")
				}
			}()
			for iterationCtx := range loop.Iterations {
				worker := NewSession(iterationCtx, "worker", ".")
				if _, err := worker.Generate[Text](iterationCtx, "prime"); err != nil {
					t.Fatal(err)
				}
				panic("body failed")
			}
		}()
		if len(adapter.closed) != 1 {
			t.Fatalf("closed sessions after panic = %v, want one child", adapter.closed)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoopIterationsReturnClosesChild(t *testing.T) {
	adapter := &fake{answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
		return "done", nil
	}}
	err := runTest(t, bind(adapter, "model", "worker"), func(ctx context.Context) error {
		stop := errors.New("stop this workflow step")
		step := func() error {
			loop := Loop(ctx, "work")
			for iterationCtx := range loop.Iterations {
				worker := NewSession(iterationCtx, "worker", ".")
				if _, err := worker.Generate[Text](iterationCtx, "prime"); err != nil {
					return err
				}
				return stop
			}
			return loop.Err()
		}
		if err := step(); !errors.Is(err, stop) {
			t.Fatalf("step returned %v, want %v", err, stop)
		}
		if len(adapter.closed) != 1 {
			t.Fatalf("closed sessions when step returned = %v, want one child", adapter.closed)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoopIterationsCancellationBeforeStart(t *testing.T) {
	adapter := &fake{}
	err := runTest(t, bind(adapter, "model", "worker"), func(ctx context.Context) error {
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		loop := Loop(cancelled, "work")
		for range loop.Iterations {
			t.Fatal("cancelled loop yielded an iteration")
		}
		if !errors.Is(loop.Err(), context.Canceled) {
			t.Fatalf("Err = %v, want context.Canceled", loop.Err())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoopIterationsCancellationAfterBody(t *testing.T) {
	started := make(chan struct{})
	adapter := &fake{answer: func(ctx context.Context, _ string, _ string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	}}
	err := runTest(t, bind(adapter, "model", "worker"), func(ctx context.Context) error {
		iterationCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		loop := Loop(iterationCtx, "work")
		for iterCtx := range loop.Iterations {
			worker := NewSession(iterCtx, "worker", ".")
			go func() {
				<-started
				cancel()
			}()
			if _, err := worker.Generate[Text](iterCtx, "blocked"); err == nil {
				t.Fatal("blocked generation unexpectedly succeeded")
			}
			cancel()
		}
		if !errors.Is(loop.Err(), context.Canceled) {
			t.Fatalf("Err = %v, want context.Canceled", loop.Err())
		}
		if len(adapter.closed) != 1 {
			t.Fatalf("closed sessions after cancellation = %v, want one child", adapter.closed)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoopIterationsCancellationDuringCloseWinsOverBreak(t *testing.T) {
	var cancel context.CancelFunc
	adapter := &fake{
		answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
			return "done", nil
		},
		onClose: func(_ context.Context, _ string) {
			cancel()
		},
	}
	err := runTest(t, bind(adapter, "model", "worker"), func(ctx context.Context) error {
		iterationCtx, stop := context.WithCancel(ctx)
		cancel = stop
		loop := Loop(iterationCtx, "work")
		for iterCtx := range loop.Iterations {
			worker := NewSession(iterCtx, "worker", ".")
			if _, err := worker.Generate[Text](iterCtx, "prime"); err != nil {
				return err
			}
			break
		}
		if !errors.Is(loop.Err(), context.Canceled) {
			t.Fatalf("Err = %v, want context.Canceled from Close", loop.Err())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
