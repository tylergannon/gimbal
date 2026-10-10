package gimbal

import (
	"context"
	"errors"
	"testing"
	"time"
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
