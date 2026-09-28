package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/runlog"
)

func TestStoppedTurnKeepsTheAdapterFailure(t *testing.T) {
	for _, tc := range []struct {
		name    string
		adapter func(ctx context.Context) error
		want    string
	}{
		{"detail", func(context.Context) error { return errors.New("codex: interrupt not confirmed") }, "context canceled\ncodex: interrupt not confirmed"},
		{"bare", func(ctx context.Context) error { return ctx.Err() }, "context canceled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var cancel context.CancelFunc
			f := &fake{answer: func(ctx context.Context, _, _ string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
				cancel()
				<-ctx.Done()
				return "", tc.adapter(ctx)
			}}
			var dir string
			var turnErr error
			err := runTest(t, bind(f, "m", "worker"), func(ctx context.Context) error {
				dir = runDir(ctx)
				worker := NewSession(ctx, "worker", "/w")
				var turnCtx context.Context
				turnCtx, cancel = context.WithCancel(ctx)
				_, turnErr = worker.Generate[Text](turnCtx, "work")
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if turnErr == nil || turnErr.Error() != tc.want || !errors.Is(turnErr, context.Canceled) {
				t.Errorf("turn error = %q, want %q and context.Canceled", turnErr, tc.want)
			}
			var ended []TurnEnded
			if err := runlog.Read[LifecycleRecord](t.Context(), dir, func(record LifecycleRecord) error {
				if turn, ok := record.Event.(TurnEnded); ok {
					ended = append(ended, turn)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if len(ended) != 1 || ended[0].Error != tc.want || !ended[0].Interrupted {
				t.Errorf("TurnEnded = %+v, want interrupted with %q", ended, strings.ReplaceAll(tc.want, "\n", `\n`))
			}
		})
	}
}
