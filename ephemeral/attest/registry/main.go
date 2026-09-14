// Run with go run ./ephemeral/attest/registry. The live check for #176 on
// the cheap tier: one real Codex session (gpt-5.6-luna, the machine's
// shared app-server daemon) takes four turns in a scope under
// web.NewRuntime, while a second goroutine holding only the run id, the
// session id, the turn ids, and the scope key reaches it through the
// runtime: it steers turn 1, kills turn 2 by id, lets turn 3 complete on
// the same session, and kills the scope by key during turn 4. The run log
// under ephemeral/attest/registry/logs is the record: one Steer with
// Source person, two Killed records, and the four TurnEnded records.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/codex"
	"github.com/tylergannon/gimble/internal/runlog"
	"github.com/tylergannon/gimble/web"
)

const (
	model     = "gpt-5.6-luna"
	scopeKey  = "lap.1"
	sessionID = "lap.1/worker.1"
	turnTwo   = "lap.1/worker.1/turn.2"
	by        = "attest"

	// Turn 1 makes several model calls (one per tool call), so a steer sent
	// between them has a model call to land at.
	slowPrompt  = "Using the shell, run `sleep 4` four times, one command per tool call, and after each one say how many runs remain. When all four are done, reply with exactly the word FINISHED."
	steerText   = "Operator: stop after the current command, run nothing else, and reply with exactly the word STEERED."
	longPrompt  = "Count from 1 to 400, one number per line, in your final answer. Do not use any tools and do not summarize; write out every number."
	shortPrompt = "Answer in one sentence: what is a context in Go? Use no tools."
)

func main() {
	port := flag.Int("port", 8099, "loopback TCP port for the web application")
	flag.Parse()

	base, err := filepath.Abs("ephemeral/attest/registry")
	if err != nil {
		log.Fatal(err)
	}
	logs := filepath.Join(base, "logs")
	if err := os.RemoveAll(logs); err != nil {
		log.Fatal(err)
	}
	workspace, err := os.MkdirTemp("", "gimble-registry-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(workspace)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 6*time.Minute)
	defer cancel()
	runtime, err := web.NewRuntime(ctx, logs, web.WithPort(*port))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Model: codex=%s\n", model)

	// The operator: it learns the run id the way the page does, from the
	// run directory, and acts by id a few seconds into each turn the
	// workflow announces on turnStarted.
	turnStarted := make(chan int)
	var operatorErr error
	var operatorWG sync.WaitGroup
	operatorWG.Go(func() {
		id := waitRunID(ctx, logs)
		fmt.Printf("Run: %s\nPage: http://127.0.0.1:%d/runs/%s\n", id, *port, id)
		var errs []error
		act := func(what string, err error) {
			fmt.Printf("[operator] %s: %v\n", what, orOK(err))
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", what, err))
			}
		}
		for turn := range turnStarted {
			switch turn {
			case 1:
				time.Sleep(6 * time.Second)
				act("steer "+sessionID, steered(runtime.Steer(ctx, id, sessionID, steerText)))
			case 2:
				time.Sleep(4 * time.Second)
				act("kill turn "+turnTwo, runtime.KillTurn(id, turnTwo, by, "the operator killed this turn by id"))
			case 4:
				time.Sleep(4 * time.Second)
				act("kill scope "+scopeKey, runtime.KillScope(id, scopeKey, by, "the operator killed this scope by key"))
			}
		}
		act("steer after the run", expectError(dropLanded(runtime.Steer(ctx, id, sessionID, "too late"))))
		operatorErr = errors.Join(errs...)
	})

	start := time.Now()
	var texts [4]string
	var errs [4]error
	runErr := runtime.Run(ctx, "registry", func(ctx context.Context) error {
		return gimble.Scope(ctx, "lap", func(ctx context.Context) error {
			worker := gimble.NewSession(ctx, "worker", codex.New(), model, workspace)
			for i, prompt := range []string{slowPrompt, longPrompt, shortPrompt, longPrompt} {
				turnStarted <- i + 1
				text, err := worker.Generate[gimble.Text](ctx, prompt)
				texts[i], errs[i] = string(text), err
				fmt.Printf("[turn %d after %s] err=%v text=%s\n", i+1, time.Since(start).Round(time.Millisecond), orOK(err), oneLine(string(text)))
			}
			return nil
		})
	})
	close(turnStarted)
	fmt.Printf("Run finished in %s: err=%v\n", time.Since(start).Round(time.Millisecond), orOK(runErr))
	operatorWG.Wait()

	id := waitRunID(ctx, logs)
	var steers []gimble.LifecycleRecord
	var kills []gimble.LifecycleRecord
	var ended []gimble.TurnEnded
	readErr := runlog.Read[gimble.LifecycleRecord](ctx, filepath.Join(logs, "runs", id), func(record gimble.LifecycleRecord) error {
		switch event := record.Event.(type) {
		case gimble.Steer:
			steers = append(steers, record)
		case gimble.Killed:
			kills = append(kills, record)
		case gimble.TurnEnded:
			ended = append(ended, event)
		}
		return nil
	})

	var problems []error
	check := func(ok bool, format string, args ...any) {
		if !ok {
			problems = append(problems, fmt.Errorf(format, args...))
		}
	}
	check(runErr == nil, "Run = %v", runErr)
	check(operatorErr == nil, "operator: %v", operatorErr)
	check(readErr == nil, "read the run log: %v", readErr)
	check(errs[0] == nil, "turn 1 = %v, want it to complete", errs[0])
	var killed gimble.Killed
	check(errors.As(errs[1], &killed) && killed.Target == turnTwo && killed.By == by, "turn 2 = %v, want the Killed cause on %s", errs[1], turnTwo)
	check(errs[2] == nil && strings.TrimSpace(texts[2]) != "", "turn 3 = %q, %v, want text on the same session after the kill", texts[2], errs[2])
	check(errors.As(errs[3], &killed) && killed.Target == scopeKey && killed.By == by, "turn 4 = %v, want the Killed cause on %s", errs[3], scopeKey)
	check(len(steers) == 1, "Steer records = %d, want one", len(steers))
	if len(steers) == 1 {
		steer := steers[0].Event.(gimble.Steer)
		check(steer.Source == "person" && steer.Target == sessionID && steer.Landed, "Steer record = %+v, want Source person on %s, landed", steer, sessionID)
		fmt.Printf("Steer record: source=%s target=%s landed=%v turn=%s\n", steer.Source, steer.Target, steer.Landed, steers[0].Turn.Value)
	}
	check(len(kills) == 2, "Killed records = %d, want two", len(kills))
	if len(kills) == 2 {
		check(kills[0].Turn.Value == turnTwo && kills[0].Event.(gimble.Killed).By == by, "first Killed record = %+v, want on %s by %s", kills[0], turnTwo, by)
		check(kills[1].Scope == scopeKey && kills[1].Session.Value == "" && kills[1].Event.(gimble.Killed).By == by, "second Killed record = %+v, want on %s by %s", kills[1], scopeKey, by)
		for _, kill := range kills {
			fmt.Printf("Killed record: scope=%s session=%s turn=%s event=%+v\n", kill.Scope, kill.Session.Value, kill.Turn.Value, kill.Event)
		}
	}
	check(len(ended) == 4, "TurnEnded records = %d, want four", len(ended))
	if len(ended) == 4 {
		check(!ended[0].Interrupted && ended[0].Error == "", "turn 1 ended %+v, want clean", ended[0])
		check(ended[1].Interrupted && strings.Contains(ended[1].Error, "killed this turn"), "turn 2 ended %+v, want interrupted by the kill", ended[1])
		check(!ended[2].Interrupted && ended[2].Error == "", "turn 3 ended %+v, want clean", ended[2])
		check(ended[3].Interrupted && strings.Contains(ended[3].Error, "killed this scope"), "turn 4 ended %+v, want interrupted by the scope kill", ended[3])
	}
	fmt.Printf("Turn 1 text mentions STEERED: %v (whether the steer changed the answer is the model's choice; the record above is what the runtime attests)\n", strings.Contains(texts[0], "STEERED"))
	if len(problems) > 0 {
		fmt.Println("FAILED:")
		for _, problem := range problems {
			fmt.Printf("  - %v\n", problem)
		}
		os.Exit(1)
	}
	fmt.Println("OK: steer by id recorded as person, turn killed by id, session reused, scope killed by key")
}

// waitRunID is the id of the one run under logs, read from the directory
// Run made for it, once it exists.
func waitRunID(ctx context.Context, logs string) string {
	for ctx.Err() == nil {
		entries, err := os.ReadDir(filepath.Join(logs, "runs"))
		if err == nil && len(entries) == 1 {
			return entries[0].Name()
		}
		time.Sleep(50 * time.Millisecond)
	}
	return ""
}

// steered turns a Steer result into an error when the message did not land.
func steered(landed bool, err error) error {
	if err == nil && !landed {
		return errors.New("the steer was dropped")
	}
	return err
}

func dropLanded(_ bool, err error) error { return err }

func expectError(err error) error {
	if err == nil {
		return errors.New("returned nil for a finished run")
	}
	fmt.Printf("[operator] finished run refused as expected: %v\n", err)
	return nil
}

func orOK(err error) any {
	if err == nil {
		return "ok"
	}
	return err
}

func oneLine(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 120 {
		return text[:120] + "..."
	}
	return text
}
