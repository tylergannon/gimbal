package gimble_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/codex"
	"github.com/tylergannon/gimble/internal/runlog"
	"github.com/tylergannon/gimble/web"
)

// TestLiveKillTurnByID is the live check for #175 on the cheap tier: one
// real Codex session (gpt-5.6-luna, the machine's shared app-server daemon)
// starts a long first turn in a scope; a second goroutine, holding only the
// run id and the turn id, kills that turn through the web runtime a few
// seconds in. The turn ends with the Killed cause, the scope keeps running,
// and the same session completes a second turn. It only runs when
// explicitly requested:
//
//	GIMBLE_LIVE=1 go test . -run TestLiveKillTurnByID -v
func TestLiveKillTurnByID(t *testing.T) {
	if os.Getenv("GIMBLE_LIVE") != "1" {
		t.Skip("set GIMBLE_LIVE=1 to run against the live codex app-server daemon")
	}
	logs := t.TempDir()
	workspace := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	instance, err := web.NewInstance(ctx, filepath.Join(logs, ".gimble"), []string{logs}, web.WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := instance.Owner.Project(logs)
	if err != nil {
		t.Fatal(err)
	}
	const turnID = "lap.1/worker.1/turn.1"
	kill := gimble.Killed{Target: turnID, By: "attest", Reason: "the operator killed this turn by id"}
	var killErr error
	var killWG sync.WaitGroup
	var first, second error
	dir := filepath.Join(logs, ".gimble", "runs")
	start := time.Now()
	err = runtime.Run(ctx, "cancel-by-id", map[gimble.WorkflowRole]gimble.ModelBinding{"worker": {Adapter: codex.New(), Model: "gpt-5.6-luna"}}, func(ctx context.Context) error {
		return gimble.Scope(ctx, "lap", func(ctx context.Context) error {
			session := gimble.NewSession(ctx, "worker", workspace)
			killWG.Go(func() {
				time.Sleep(4 * time.Second)
				killErr = runtime.KillTurn(liveRunID(t, dir), turnID, kill.By, kill.Reason)
			})
			_, first = session.Generate[gimble.Text](ctx, "Count from 1 to 400, one number per line, in your final answer. Do not use any tools and do not summarize; write out every number.")
			t.Logf("turn 1 ended after %s: %v", time.Since(start).Round(time.Millisecond), first)
			killWG.Wait()
			if killErr != nil {
				return fmt.Errorf("kill %s: %w", turnID, killErr)
			}
			var cause gimble.Killed
			if !errors.As(first, &cause) || cause != kill {
				return fmt.Errorf("turn 1 ended with %v, want the Killed cause %v", first, kill)
			}
			if ctx.Err() != nil {
				return fmt.Errorf("the scope's ctx ended with the turn: %w", ctx.Err())
			}
			text, err := session.Generate[gimble.Text](ctx, "Answer in one sentence: what is a context in Go? Use no tools.")
			second = err
			t.Logf("turn 2 after %s: %s", time.Since(start).Round(time.Millisecond), text)
			if err == nil && strings.TrimSpace(string(text)) == "" {
				return errors.New("turn 2 returned no text")
			}
			return err
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if second != nil {
		t.Fatal(second)
	}

	runDir := filepath.Join(dir, liveRunID(t, dir))
	t.Logf("run: %s", filepath.Base(runDir))
	var killedRecords []gimble.LifecycleRecord
	var ended []gimble.TurnEnded
	if err := runlog.Read[gimble.LifecycleRecord](ctx, runDir, func(record gimble.LifecycleRecord) error {
		switch event := record.Event.(type) {
		case gimble.Killed:
			killedRecords = append(killedRecords, record)
		case gimble.TurnEnded:
			ended = append(ended, event)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(killedRecords) != 1 || killedRecords[0].Turn.Value != turnID || killedRecords[0].Event != kill {
		t.Fatalf("Killed records = %+v, want one placed on %s", killedRecords, turnID)
	}
	if len(ended) != 2 || !ended[0].Interrupted || !strings.Contains(ended[0].Error, kill.Reason) || ended[1].Interrupted || ended[1].Error != "" {
		t.Fatalf("TurnEnded records = %+v, want the first interrupted with the reason and the second clean", ended)
	}
}

// liveRunID is the id of the one run under the project's runs directory,
// read the way an operator would: from the directory Run made for it.
func liveRunID(t *testing.T, runs string) string {
	t.Helper()
	entries, err := os.ReadDir(runs)
	if err != nil || len(entries) != 1 {
		t.Fatalf("runs = %v, %v", entries, err)
	}
	return entries[0].Name()
}
