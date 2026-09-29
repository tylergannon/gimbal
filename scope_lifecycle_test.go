package gimbal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/compiledscope"
)

func TestLocalScopeEarlyReturnCancelsOpenChild(t *testing.T) {
	bodyErr := errors.New("parent returned early")
	childDone := make(chan error, 1)
	var root *scope
	err := runTest(t, nil, func(ctx context.Context) error {
		root, _ = current(ctx)
		parentErr := Scope(ctx, "parent", func(ctx context.Context) error {
			started := make(chan struct{})
			go func() {
				childDone <- Scope(ctx, "child", func(ctx context.Context) error {
					close(started)
					<-ctx.Done()
					return ctx.Err()
				})
			}()
			<-started
			return bodyErr
		})
		if !errors.Is(parentErr, bodyErr) || parentErr.Error() != bodyErr.Error() {
			t.Errorf("Scope returned %v, want original body error %v", parentErr, bodyErr)
		}
		select {
		case err := <-childDone:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("child returned %v, want cancellation", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("child remained active after parent body returned")
			// Release the child even when the regression is present.
			root.cancel(nil)
			<-childDone
		}
		return nil
	})
	if err != nil {
		t.Errorf("Run returned %v", err)
	}
	root.mu.Lock()
	ended := root.ended
	root.mu.Unlock()
	root.run.mu.Lock()
	remaining := len(root.run.scopes)
	root.run.mu.Unlock()
	if !ended || root.ctx.Err() == nil || remaining != 0 {
		t.Errorf("run leaked scopes: ended=%v cancellation=%v remaining=%d", ended, root.ctx.Err(), remaining)
	}
}

func TestCompiledLoopAndTaskFinishRequireClosedChildren(t *testing.T) {
	err := runTest(t, nil, func(ctx context.Context) error {
		if err := compiledscope.InitializeContext(ctx, compiledscope.Store{Root: t.TempDir()}, t.TempDir(), ""); err != nil {
			return err
		}
		loop, finishLoop, err := compiledscope.OpenLoop(ctx, "loop")
		if err != nil {
			return err
		}
		raw := []byte(`{"name":"work","description":"do work","definition_of_done":"done","validation":{"command":"","query":""}}`)
		task, _, finishTask, err := compiledscope.OpenTask(loop, "task", raw, "")
		if err != nil {
			return err
		}
		child, finishChild, err := compiledscope.OpenScope(task, "child")
		if err != nil {
			return err
		}
		for _, finish := range []func(error) error{finishTask, finishLoop} {
			if err := finish(nil); err == nil {
				t.Error("compiled finish accepted an open child")
			}
		}
		if task.Err() != nil || loop.Err() != nil || child.Err() != nil {
			t.Error("refused finish cancelled active work")
		}
		loopScope, _ := current(loop)
		if !loopScope.queueMessage("still dispatching") {
			t.Error("refused loop finish ended dispatch")
		}
		for _, finish := range []func(error) error{finishChild, finishTask, finishLoop} {
			if err := finish(nil); err != nil {
				return err
			}
			if err := finish(nil); err == nil {
				t.Error("duplicate compiled finish accepted")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
