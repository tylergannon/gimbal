package gimble

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/sync/errgroup"
)

// TestSteerReportsLandedOrDropped covers the outcomes Session.Steer reports
// and that the Steer lifecycle record carries the same one. A steer during
// a turn lands. A steer with no turn running is dropped, false and no
// error. And a steer the adapter could not deliver because the turn ended
// inside the harness first, which the session cannot see for itself, is
// dropped the same way, on the adapter's word, rather than recorded as
// landed or returned as a failure.
func TestSteerReportsLandedOrDropped(t *testing.T) {
	for _, tc := range []struct {
		name string
		drop bool // the adapter reports every steer dropped: the turn ended first
	}{
		{name: "lands during the turn"},
		{name: "dropped inside the adapter", drop: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project := t.TempDir()
			f := &fake{dropSteers: tc.drop}
			started := make(chan struct{})
			release := make(chan struct{})
			f.answer = func(ctx context.Context, session, prompt string, schema json.RawMessage, emit func(AgentEvent) error) (string, error) {
				close(started)
				select {
				case <-release:
					return "done", nil
				case <-ctx.Done():
					return "", ctx.Err()
				}
			}
			var during, after bool
			var duringErr, afterErr error
			err := Run(Project(t.Context(), project), "steer", func(ctx context.Context) error {
				worker := NewSession(ctx, "worker", f, "model", project)
				group, groupCtx := errgroup.WithContext(ctx)
				group.Go(func() error {
					_, err := worker.Generate[Text](groupCtx, "build")
					return err
				})
				group.Go(func() error {
					select {
					case <-started:
					case <-groupCtx.Done():
						return groupCtx.Err()
					}
					during, duringErr = worker.Steer(groupCtx, "mid-turn")
					close(release)
					return nil
				})
				if err := group.Wait(); err != nil {
					return err
				}
				after, afterErr = worker.Steer(ctx, "after the turn")
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if duringErr != nil || afterErr != nil {
				t.Fatalf("Steer errors = %v, %v; a dropped steer is not an error", duringErr, afterErr)
			}
			if during == tc.drop {
				t.Fatalf("steer during the turn landed = %v, want %v", during, !tc.drop)
			}
			if after {
				t.Fatal("a steer with no turn running landed")
			}

			f.mu.Lock()
			delivered := slices.Clone(f.steers)
			f.mu.Unlock()
			var want []string
			if !tc.drop {
				want = []string{"mid-turn"}
			}
			if !slices.Equal(delivered, want) {
				t.Fatalf("the adapter received %q, want %q", delivered, want)
			}

			runs, err := filepath.Glob(filepath.Join(project, "runs", "*", "run.jsonl"))
			if err != nil || len(runs) != 1 {
				t.Fatalf("run logs = %v, %v", runs, err)
			}
			var steers []Steer
			for _, record := range readRecords[LifecycleRecord](t, runs[0]) {
				if event, ok := record.Event.(Steer); ok {
					steers = append(steers, event)
				}
			}
			if len(steers) != 2 {
				t.Fatalf("Steer records = %+v, want two", steers)
			}
			if steers[0].Message != "mid-turn" || steers[0].Landed == tc.drop {
				t.Fatalf("record for the steer during the turn = %+v, want landed=%v", steers[0], !tc.drop)
			}
			if steers[1].Message != "after the turn" || steers[1].Landed {
				t.Fatalf("record for the steer after the turn = %+v, want landed=false", steers[1])
			}
		})
	}
}
