package gimble

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/polytype"
)

// waiting is a fake whose "wait" prompts block until their ctx ends and
// whose other prompts answer at once.
func waiting() *fake {
	return &fake{answer: func(ctx context.Context, session, prompt string, schema json.RawMessage, emit func(AgentEvent) error) (string, error) {
		if prompt == "wait" {
			<-ctx.Done()
			return "", ctx.Err()
		}
		return "ok", nil
	}}
}

// runningTurns waits until n turns are inside the fake's RunTurn.
func runningTurns(t *testing.T, f *fake, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		f.mu.Lock()
		running := len(f.running)
		f.mu.Unlock()
		if running == n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d turns running, want %d", running, n)
		}
		time.Sleep(time.Millisecond)
	}
}

func runRecords(t *testing.T, project string) []LifecycleRecord {
	t.Helper()
	runs, err := filepath.Glob(filepath.Join(project, "runs", "*", "run.jsonl"))
	if err != nil || len(runs) != 1 {
		t.Fatalf("run logs = %v, %v", runs, err)
	}
	return readRecords[LifecycleRecord](t, runs[0])
}

// TestKillScopeClosesEverySessionAndReportsTheCause: a scope killed by key
// from another goroutine ends with an error whose cause is the Killed, every
// ctx under it reports that cause, every session inside it is closed, and
// the log records who killed what.
func TestKillScopeClosesEverySessionAndReportsTheCause(t *testing.T) {
	f := waiting()
	project := t.TempDir()
	kill := Killed{Target: "lap.1", By: "tyler", Reason: "off the rails"}
	var scopeErr, turnOne, turnTwo, seen error
	var killErr error
	var killWG sync.WaitGroup
	err := Run(Project(t.Context(), project), "test", bind(f, "m", "one", "two"), func(ctx context.Context) error {
		r, _ := current(ctx)
		killWG.Go(func() {
			runningTurns(t, f, 2)
			killErr = r.run.CancelScope("lap.1", kill)
		})
		scopeErr = Scope(ctx, "lap", func(ctx context.Context) error {
			one := NewSession(ctx, "one", "/w")
			two := NewSession(ctx, "two", "/w")
			var wg sync.WaitGroup
			wg.Go(func() { _, turnOne = one.Generate[Text](ctx, "wait") })
			wg.Go(func() { _, turnTwo = two.Generate[Text](ctx, "wait") })
			wg.Wait()
			seen = context.Cause(ctx)
			return errors.Join(turnOne, turnTwo)
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	killWG.Wait()
	if killErr != nil {
		t.Fatalf("CancelScope = %v", killErr)
	}
	for name, err := range map[string]error{"Scope": scopeErr, "turn one": turnOne, "turn two": turnTwo, "context.Cause": seen} {
		var killed Killed
		if !errors.As(err, &killed) || killed != kill {
			t.Errorf("%s = %v, want the Killed cause %v", name, err, kill)
		}
	}
	f.mu.Lock()
	closed := append([]string(nil), f.closed...)
	f.mu.Unlock()
	if len(closed) != 2 {
		t.Fatalf("closed sessions = %v, want both", closed)
	}
	var recorded []Killed
	for _, record := range runRecords(t, project) {
		if killed, ok := record.Event.(Killed); ok {
			if record.Scope != "lap.1" {
				t.Errorf("Killed recorded on scope %q, want lap.1", record.Scope)
			}
			recorded = append(recorded, killed)
		}
	}
	if len(recorded) != 1 || recorded[0] != kill {
		t.Fatalf("Killed records = %v, want one equal to %v", recorded, kill)
	}
}

// TestKillTurnEndsOnlyThatTurn: killing one turn by id returns an error
// from that Generate whose cause is the Killed, and the same session then
// completes another turn in the same scope. An unknown id is an error.
func TestKillTurnEndsOnlyThatTurn(t *testing.T) {
	f := waiting()
	project := t.TempDir()
	kill := Killed{Target: "lap.1/coder.1/turn.1", By: "tyler", Reason: "wrong file"}
	var unknownErr error
	var killErr error
	var killWG sync.WaitGroup
	err := Run(Project(t.Context(), project), "test", bind(f, "m", "coder"), func(ctx context.Context) error {
		return Scope(ctx, "lap", func(ctx context.Context) error {
			r, _ := current(ctx)
			unknownErr = r.run.CancelTurn("lap.1/coder.1/turn.9", kill)
			s := NewSession(ctx, "coder", "/w")
			killWG.Go(func() {
				runningTurns(t, f, 1)
				killErr = r.run.CancelTurn("lap.1/coder.1/turn.1", kill)
			})
			_, err := s.Generate[Text](ctx, "wait")
			var killed Killed
			if !errors.As(err, &killed) || killed != kill {
				t.Errorf("killed Generate = %v, want the Killed cause %v", err, kill)
			}
			if ctx.Err() != nil {
				t.Errorf("the scope's ctx ended with the turn: %v", ctx.Err())
			}
			text, err := s.Generate[Text](ctx, "again")
			if err != nil || text != "ok" {
				t.Errorf("second turn on the same session = %q, %v", text, err)
			}
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	killWG.Wait()
	if killErr != nil {
		t.Fatalf("CancelTurn = %v", killErr)
	}
	if unknownErr == nil {
		t.Fatal("CancelTurn of an unknown turn id returned nil")
	}
	var ended []TurnEnded
	var recorded []LifecycleRecord
	for _, record := range runRecords(t, project) {
		switch event := record.Event.(type) {
		case TurnEnded:
			ended = append(ended, event)
		case Killed:
			recorded = append(recorded, record)
		}
	}
	if len(ended) != 2 || !ended[0].Interrupted || !strings.Contains(ended[0].Error, "wrong file") || ended[1].Interrupted || ended[1].Error != "" {
		t.Fatalf("TurnEnded records = %+v, want the first interrupted with the reason and the second clean", ended)
	}
	if len(recorded) != 1 || recorded[0].Scope != "lap.1" || recorded[0].Session.Value != "lap.1/coder.1" || recorded[0].Turn.Value != kill.Target || recorded[0].Event != kill {
		t.Fatalf("Killed records = %+v, want one placed on the turn", recorded)
	}
}

// TestGroupKilledChildLeavesItsSiblingsRunning: in a Group of three, a
// killed child does not cancel the other two; they finish, and Wait returns
// the killed child's error.
func TestGroupKilledChildLeavesItsSiblingsRunning(t *testing.T) {
	release := make(chan struct{})
	f := &fake{answer: func(ctx context.Context, session, prompt string, schema json.RawMessage, emit func(AgentEvent) error) (string, error) {
		select {
		case <-release:
			return "ok", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}}
	kill := Killed{Target: "bakeoff.1/attempt.1", By: "tyler", Reason: "duplicate"}
	var killErr error
	results := make([]Text, 3)
	errs := make([]error, 3)
	err := runTest(t, bind(f, "m", "candidate"), func(ctx context.Context) error {
		r, _ := current(ctx)
		g := Group(ctx, "bakeoff")
		for i := range 3 {
			g.Go("attempt", func(ctx context.Context) error {
				results[i], errs[i] = NewSession(ctx, "candidate", "/w").Generate[Text](ctx, "go")
				return errs[i]
			})
		}
		runningTurns(t, f, 3)
		killErr = r.run.CancelScope("bakeoff.1/attempt.1", kill)
		runningTurns(t, f, 2) // the killed turn left; the other two are still inside RunTurn
		close(release)
		return g.Wait()
	})
	if killErr != nil {
		t.Fatalf("CancelScope = %v", killErr)
	}
	var killed Killed
	if !errors.As(err, &killed) || killed != kill {
		t.Fatalf("Wait = %v, want the Killed cause %v", err, kill)
	}
	if !errors.As(errs[0], &killed) {
		t.Errorf("attempt 1 = %v, want killed", errs[0])
	}
	for i := 1; i < 3; i++ {
		if errs[i] != nil || results[i] != "ok" {
			t.Errorf("attempt %d = %q, %v, want it to finish", i+1, results[i], errs[i])
		}
	}
}

// TestLoopKilledTaskIsAFailedTaskNotABrokenLoop: a task whose scope is
// killed is recorded failed, the planner sees the reason on its next lap,
// and the loop runs that lap instead of ending with the kill.
func TestLoopKilledTaskIsAFailedTaskNotABrokenLoop(t *testing.T) {
	task := Task{Name: "Refactor", Description: "Split the package.", DefinitionOfDone: "It builds."}
	var mu sync.Mutex
	var plans []string
	f := &fake{answer: func(ctx context.Context, session, prompt string, schema json.RawMessage, emit func(AgentEvent) error) (string, error) {
		if strings.HasPrefix(prompt, "wait") {
			<-ctx.Done()
			return "", ctx.Err()
		}
		if strings.HasPrefix(prompt, "work") {
			return "ok", nil
		}
		mu.Lock()
		plans = append(plans, prompt)
		n := len(plans)
		mu.Unlock()
		p := plan{Tasks: []Task{task}}
		if n <= 2 {
			p.Next = polytype.Nullable[int]{Present: true, Value: 0}
		}
		raw, err := json.Marshal(p)
		return string(raw), err
	}}
	project := t.TempDir()
	kill := Killed{Target: "sprint.1/task.1", By: "tyler", Reason: "wrong package"}
	var killErr error
	var killWG sync.WaitGroup
	var laps []error
	err := Run(Project(t.Context(), project), "test", bind(f, "m", "planner", "worker"), func(ctx context.Context) error {
		r, _ := current(ctx)
		planner := NewSession(ctx, "planner", t.TempDir())
		loop := Loop(ctx, "sprint")
		for ctx := range loop.Tasks("ship", planner) {
			worker := NewSession(ctx, "worker", t.TempDir())
			if len(laps) == 0 {
				killWG.Go(func() {
					runningTurns(t, f, 1)
					killErr = r.run.CancelScope("sprint.1/task.1", kill)
				})
				_, err := worker.Generate[Text](ctx, "wait")
				laps = append(laps, err)
				continue
			}
			_, err := worker.Generate[Text](ctx, "work")
			laps = append(laps, err)
		}
		return loop.Err()
	})
	if err != nil {
		t.Fatalf("Loop ended with %v, want a second lap after the kill", err)
	}
	killWG.Wait()
	if killErr != nil {
		t.Fatalf("CancelScope = %v", killErr)
	}
	var killed Killed
	if len(laps) != 2 || !errors.As(laps[0], &killed) || laps[1] != nil {
		t.Fatalf("laps = %v, want the first killed and the second clean", laps)
	}
	if len(plans) != 3 || !strings.Contains(plans[1], "task failed") || !strings.Contains(plans[1], "wrong package") {
		t.Fatalf("the planner did not see the failed task on its next lap:\n%s", strings.Join(plans, "\n---\n"))
	}
	var endedTasks []string
	for _, record := range runRecords(t, project) {
		if ended, ok := record.Event.(ScopeEnded); ok && strings.HasPrefix(record.Scope, "sprint.1/task.") {
			endedTasks = append(endedTasks, ended.Error)
		}
	}
	if len(endedTasks) != 2 || !strings.Contains(endedTasks[0], "wrong package") || endedTasks[1] != "" {
		t.Fatalf("task ScopeEnded errors = %q, want the first recorded failed with the reason", endedTasks)
	}
}

// TestCancelledTurnLeavesTheSessionUsable is the missing #165 test: after a
// turn is cancelled through its own ctx, the same Session runs another turn
// successfully in the same scope.
func TestCancelledTurnLeavesTheSessionUsable(t *testing.T) {
	f := waiting()
	project := t.TempDir()
	err := Run(Project(t.Context(), project), "test", bind(f, "m", "coder"), func(ctx context.Context) error {
		s := NewSession(ctx, "coder", "/w")
		turnCtx, cancel := context.WithCancel(ctx)
		go func() {
			runningTurns(t, f, 1)
			cancel()
		}()
		if _, err := s.Generate[Text](turnCtx, "wait"); !errors.Is(err, context.Canceled) {
			t.Errorf("cancelled Generate = %v, want context.Canceled", err)
		}
		text, err := s.Generate[Text](ctx, "again")
		if err != nil || text != "ok" {
			t.Errorf("turn after the cancelled one = %q, %v", text, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var ended []TurnEnded
	for _, record := range runRecords(t, project) {
		if event, ok := record.Event.(TurnEnded); ok {
			ended = append(ended, event)
		}
	}
	if len(ended) != 2 || !ended[0].Interrupted || ended[1].Interrupted || ended[1].Error != "" {
		t.Fatalf("TurnEnded records = %+v, want the first interrupted and the second clean", ended)
	}
}
