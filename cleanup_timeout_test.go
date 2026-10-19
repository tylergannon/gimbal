package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/runstore"
)

func TestCleanupTimeoutIsCapturedPerRunAndUsedForRetry(t *testing.T) {
	for _, tc := range []struct {
		name    string
		timeout time.Duration
		want    time.Duration
	}{
		{"default", 0, 10 * time.Second},
		{"short", 250 * time.Millisecond, 250 * time.Millisecond},
		{"other-run", time.Second, time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := Project(t.Context(), t.TempDir())
			if tc.timeout != 0 {
				ctx = WithCleanupTimeout(ctx, tc.timeout)
			}
			remaining := make(chan time.Duration, 2)
			var calls atomic.Int32
			failed := errors.New("cleanup needs retry")
			adapter := &fake{
				answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
					return "ok", nil
				},
				onClose: func(ctx context.Context, _ string) {
					deadline, ok := ctx.Deadline()
					if !ok {
						t.Error("adapter cleanup has no deadline")
					}
					remaining <- time.Until(deadline)
				},
				closeErr: func(string) error {
					if calls.Add(1) == 1 {
						return failed
					}
					return nil
				},
			}
			var owner *run
			err := Run(ctx, string(runstore.NewRunID()), tc.name, bind(adapter, "model", "worker"), func(ctx context.Context) error {
				s, err := current(ctx)
				if err != nil {
					return err
				}
				owner = s.run
				// A descendant context cannot alter the policy captured by Run.
				return Scope(WithCleanupTimeout(ctx, time.Nanosecond), "child", func(ctx context.Context) error {
					_, err := NewSession(ctx, "worker", t.TempDir()).Generate[Text](ctx, "work")
					return err
				})
			})
			if !errors.Is(err, failed) || !owner.CleanupPending() {
				t.Fatalf("failed cleanup = %v; pending = %v", err, owner.CleanupPending())
			}
			if err := owner.retryCleanup(); err != nil || owner.CleanupPending() {
				t.Fatalf("retry = %v; pending = %v", err, owner.CleanupPending())
			}
			for range 2 {
				got := <-remaining
				if got <= tc.want/2 || got > tc.want {
					t.Errorf("cleanup deadline remaining = %v, want approximately %v", got, tc.want)
				}
			}
		})
	}
}

func TestCleanupTimeoutRejectsNonpositiveDuration(t *testing.T) {
	for _, timeout := range []time.Duration{0, -time.Second} {
		t.Run(timeout.String(), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("invalid cleanup timeout did not panic")
				}
			}()
			WithCleanupTimeout(t.Context(), timeout)
		})
	}
}
