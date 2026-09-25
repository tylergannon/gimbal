package gimbal

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

// TestIssue118ConcurrentRunsShareProjectLog runs two Runs of one project at
// once and checks that project.jsonl, written by each Run's own eventWriter,
// stays well-formed under the chosen fix: Seq is dropped (always 0) for
// project-level records, and Time is what orders them. A WaitGroup barrier
// forces both runs' bodies to be live at the same time, so their
// RunStarted/RunEnded writes to the shared file genuinely interleave rather
// than merely racing to finish first.
func TestIssue118ConcurrentRunsShareProjectLog(t *testing.T) {
	project := t.TempDir()

	var barrier sync.WaitGroup
	barrier.Add(2)

	var g errgroup.Group
	for _, name := range []string{"alpha", "beta"} {
		g.Go(func() error {
			return Run(Project(t.Context(), project), name, nil, func(ctx context.Context) error {
				// Neither run's body returns (and so neither writes
				// RunEnded) until both have started, guaranteeing both
				// runs are live in the project at once.
				barrier.Done()
				barrier.Wait()
				return nil
			})
		})
	}
	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}

	records := readRecords[LifecycleRecord](t, filepath.Join(project, "project.jsonl"))
	if len(records) != 4 {
		t.Fatalf("project.jsonl records = %d, want 4 (started+ended for two runs): %+v", len(records), records)
	}

	type span struct{ started, ended bool }
	byName := map[string]*span{"alpha": {}, "beta": {}}
	startedAt := map[string]time.Time{}
	endedAt := map[string]time.Time{}
	for _, r := range records {
		if r.Seq != 0 {
			t.Fatalf("project.jsonl record carries a nonzero seq; the chosen fix drops Seq for project-level records: %+v", r)
		}
		if r.Time.IsZero() {
			t.Fatalf("project.jsonl record lacks a time to order by: %+v", r)
		}
		switch e := r.Event.(type) {
		case RunStarted:
			s, ok := byName[e.Name]
			if !ok {
				t.Fatalf("run_started for unexpected run %q", e.Name)
			}
			s.started = true
			startedAt[e.Name] = r.Time
		case RunEnded:
			s, ok := byName[e.Name]
			if !ok {
				t.Fatalf("run_ended for unexpected run %q", e.Name)
			}
			s.ended = true
			endedAt[e.Name] = r.Time
		default:
			t.Fatalf("project.jsonl carries an unexpected event: %+v", r)
		}
	}
	for name, s := range byName {
		if !s.started || !s.ended {
			t.Fatalf("project.jsonl missing run_started/run_ended for %q: %+v", name, s)
		}
	}
	// Each run's own start precedes the other run's end: proof the two runs
	// were live in the project at the same time, which is exactly the
	// scenario that used to collide on Seq.
	if !startedAt["alpha"].Before(endedAt["beta"]) || !startedAt["beta"].Before(endedAt["alpha"]) {
		t.Fatalf("run spans do not overlap: alpha started %v beta started %v alpha ended %v beta ended %v",
			startedAt["alpha"], startedAt["beta"], endedAt["alpha"], endedAt["beta"])
	}
}
