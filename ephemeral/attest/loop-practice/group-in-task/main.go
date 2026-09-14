// Run with go run ./ephemeral/attest/loop-practice/group-in-task. Loop
// practice, shape 4 (#178): a Group inside a task, one attempt killed by
// id. A Codex planner (gpt-5.6-luna) dispatches a task; the task runs three
// Codex attempts (gpt-5.6-luna) in a Group, each writing in its own
// directory. A second goroutine holding only the run id follows run.jsonl,
// sees attempt 2's turn start, and kills that attempt's scope by key
// through runtime.KillScope. The task records every attempt's outcome, the
// kill included, picks the first attempt that finished, copies its file to
// the workspace, and returns nil, so the loop goes on to the planner's next
// decision instead of failing on the killed attempt.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
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
	model    = "gpt-5.6-luna"
	attempts = 3
	maxTasks = 3
	by       = "operator"

	goal = "In this directory, produce limerick.txt: one limerick about the Go programming language, five lines, nothing else. The workflow runs each task as three parallel attempts and keeps one. Plan this as one task; when limerick.txt exists in this directory, end dispatch."

	workPrompt = "Complete the task below in the current directory. First run the shell command `sleep 6` once, as its own tool call; then write the file. Answer with the file's contents and nothing else.\n\n"
)

func main() {
	port := flag.Int("port", 8184, "loopback TCP port for the web application")
	hold := flag.Duration("hold", 3*time.Minute, "how long to hold the server open after the run")
	flag.Parse()

	base, err := filepath.Abs("ephemeral/attest/loop-practice/group-in-task")
	if err != nil {
		log.Fatal(err)
	}
	logs := filepath.Join(base, "logs")
	workspace := filepath.Join(base, "workspace")
	for _, dir := range []string{logs, workspace} {
		if err := os.RemoveAll(dir); err != nil {
			log.Fatal(err)
		}
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runtime, err := web.NewRuntime(ctx, logs, web.WithPort(*port))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Models: planner=codex/%s attempts=codex/%s\n", model, model)

	// The operator: it learns the run id from the run directory, follows the
	// run log as the page does, and the first time it sees a turn start on
	// attempt 2 it kills that attempt's scope by key, a few seconds in.
	ready := make(chan string, 1)
	var killOnce sync.Once
	go func() {
		id := waitRunID(ctx, logs)
		fmt.Printf("Run: %s\nPage: http://127.0.0.1:%d/runs/%s\n", id, *port, id)
		ready <- id
		_ = runlog.Read[gimble.LifecycleRecord](ctx, filepath.Join(logs, "runs", id), func(r gimble.LifecycleRecord) error {
			switch e := r.Event.(type) {
			case gimble.TurnStarted:
				if strings.Contains(r.Scope, "/attempt.2") {
					killOnce.Do(func() {
						scope := r.Scope
						go func() {
							time.Sleep(3 * time.Second)
							err := runtime.KillScope(id, scope, by, "the operator killed attempt 2 by id from a second goroutine")
							fmt.Printf("[operator] KillScope(%s, %q): %v\n", id, scope, orOK(err))
							time.Sleep(time.Second)
							save(fmt.Sprintf("http://127.0.0.1:%d/api/runs/%s", *port, id), filepath.Join(base, "snapshot-midrun.json"))
						}()
					})
				}
			case gimble.Killed:
				fmt.Printf("[log] killed on scope=%s session=%s turn=%s by=%s target=%s reason=%q\n", r.Scope, r.Session.Value, r.Turn.Value, e.By, e.Target, e.Reason)
			case gimble.ScopeEnded:
				if strings.Contains(r.Scope, "/attempt") || strings.Contains(r.Scope, "/task.") {
					fmt.Printf("[log] scope_ended %s error=%q\n", r.Scope, e.Error)
				}
			case gimble.PlannerDecision:
				if e.Task.Present {
					fmt.Printf("[log] planner_decision on %s: task %q\n", r.Scope, e.Task.Value.Name)
				} else {
					fmt.Printf("[log] planner_decision on %s: dispatch ended\n", r.Scope)
				}
			}
			return nil
		})
	}()

	runCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	start := time.Now()
	tasks := 0
	runErr := runtime.Run(runCtx, "group-in-task", func(ctx context.Context) error {
		planner := gimble.NewSession(ctx, "planner", codex.New(), model, workspace)
		loop := gimble.Loop(ctx, "practice", goal, planner)
		for ctx, task := range loop.Tasks {
			if tasks++; tasks > maxTasks {
				fmt.Printf("[lap %d] cap of %d tasks hit; leaving the loop\n", tasks, maxTasks)
				break
			}
			fmt.Printf("[lap %d after %s] task %q: %s\n", tasks, time.Since(start).Round(time.Second), task.Name, oneLine(task.Description))
			prompt := workPrompt + gimble.ScopeText(ctx)

			var results [attempts]string
			var errs [attempts]error
			group := gimble.Group(ctx, "attempts")
			for i := range attempts {
				dir := filepath.Join(workspace, fmt.Sprintf("attempt-%d", i+1))
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return err
				}
				group.Go("attempt", func(ctx context.Context) error {
					writer := gimble.NewSession(ctx, "writer", codex.New(), model, dir)
					text, err := writer.Generate[gimble.Text](ctx, prompt)
					results[i], errs[i] = string(text), err
					fmt.Printf("[lap %d] attempt %d after %s: err=%v\n", tasks, i+1, time.Since(start).Round(time.Second), orOK(err))
					return err
				})
			}
			err := group.Wait()
			var killed gimble.Killed
			if err != nil && !errors.As(err, &killed) {
				return err
			}
			winner := 0
			for i := range attempts {
				outcome := "finished: " + oneLine(results[i])
				if errs[i] != nil {
					outcome = "failed: " + errs[i].Error()
					var k gimble.Killed
					if errors.As(errs[i], &k) {
						outcome = fmt.Sprintf("killed by %s: %s", k.By, k.Reason)
					}
				} else if winner == 0 {
					winner = i + 1
				}
				gimble.Set(ctx, fmt.Sprintf("attempt %d", i+1), outcome)
			}
			if winner == 0 {
				gimble.Set(ctx, "winner", "none: every attempt failed")
				continue
			}
			src := filepath.Join(workspace, fmt.Sprintf("attempt-%d", winner), "limerick.txt")
			raw, err := os.ReadFile(src)
			if err != nil {
				gimble.Set(ctx, "winner", fmt.Sprintf("attempt %d finished but left no limerick.txt: %v", winner, err))
				continue
			}
			if err := os.WriteFile(filepath.Join(workspace, "limerick.txt"), raw, 0o644); err != nil {
				return err
			}
			gimble.Set(ctx, "winner", fmt.Sprintf("attempt %d; its limerick.txt was copied to the workspace", winner))
			fmt.Printf("[lap %d] winner: attempt %d\n", tasks, winner)
		}
		return loop.Err()
	})
	fmt.Printf("Run finished in %s after %d tasks: err=%v\n", time.Since(start).Round(time.Millisecond), tasks, orOK(runErr))
	if raw, err := os.ReadFile(filepath.Join(workspace, "limerick.txt")); err == nil {
		fmt.Printf("limerick.txt:\n%s\n", raw)
	} else {
		fmt.Printf("limerick.txt: %v\n", err)
	}

	id := <-ready
	save(fmt.Sprintf("http://127.0.0.1:%d/api/runs/%s", *port, id), filepath.Join(base, "snapshot.json"))
	save(fmt.Sprintf("http://127.0.0.1:%d/runs/%s", *port, id), filepath.Join(base, "page.html"))

	fmt.Printf("Holding the server for %s; press Enter to stop early.\n", *hold)
	waitEnter(ctx, *hold)
	if runErr != nil {
		os.Exit(1)
	}
}

// waitRunID is the id of the one run under logs, once its run.jsonl exists.
// The run directory appears before the log's first record, and runlog.Read
// fails at once on a directory with no run.jsonl, so waiting for the
// directory alone is a race the watcher lost in the first run of this shape.
func waitRunID(ctx context.Context, logs string) string {
	for ctx.Err() == nil {
		entries, err := os.ReadDir(filepath.Join(logs, "runs"))
		if err == nil && len(entries) == 1 {
			if _, err := os.Stat(filepath.Join(logs, "runs", entries[0].Name(), "run.jsonl")); err == nil {
				return entries[0].Name()
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return ""
}

func save(url, file string) {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("[page] GET %s: %v\n", url, err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if err := os.WriteFile(file, body, 0o644); err != nil {
		fmt.Printf("[page] write %s: %v\n", file, err)
		return
	}
	fmt.Printf("[page] GET %s: %s, %d bytes, saved to %s\n", url, resp.Status, len(body), filepath.Base(file))
}

func waitEnter(ctx context.Context, hold time.Duration) {
	done := make(chan struct{})
	go func() {
		var line [1]byte
		for {
			n, err := os.Stdin.Read(line[:])
			if err != nil {
				return
			}
			if n > 0 && line[0] == '\n' {
				close(done)
				return
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(hold):
	case <-ctx.Done():
	}
}

func orOK(err error) any {
	if err == nil {
		return "ok"
	}
	return err
}

func oneLine(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 160 {
		return text[:160] + "..."
	}
	return text
}
