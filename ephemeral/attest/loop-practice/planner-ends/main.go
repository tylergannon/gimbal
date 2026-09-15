// Run with go run ./ephemeral/attest/loop-practice/planner-ends. Loop
// practice, shape 1 (#178): the planner ends the loop on its own. A Codex
// planner (gpt-5.6-luna) dispatches tasks toward a three-file goal in a
// scratch workspace; each task is one turn of an Antigravity worker
// (gemini-3.8-flash-low). The workflow records the worker's answer and a
// listing of the workspace after each task, and stops only if the planner
// ends dispatch or the task cap is hit. The run is served by web.NewRuntime
// on -port; after the run the page's snapshot JSON and HTML are saved beside
// the logs and the server is held open for -hold.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/agy"
	"github.com/tylergannon/gimble/codex"
	"github.com/tylergannon/gimble/internal/runlog"
	"github.com/tylergannon/gimble/web"
)

const (
	plannerModel = "gpt-5.6-luna"
	workerModel  = "gemini-3.8-flash-low"
	maxTasks     = 6

	goal = "In this directory, make three files: spring.txt, summer.txt, and autumn.txt. Each holds one haiku about that season and nothing else. Plan one file per task. When all three files exist, the goal is met and dispatch should end."

	workPrompt = "Complete the task below in the current directory. Answer with one sentence saying what you did.\n\n"
)

func main() {
	port := flag.Int("port", 8181, "loopback TCP port for the web application")
	hold := flag.Duration("hold", 3*time.Minute, "how long to hold the server open after the run")
	flag.Parse()

	base, err := filepath.Abs("ephemeral/attest/loop-practice/planner-ends")
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
	fmt.Printf("Models: planner=codex/%s worker=agy/%s\n", plannerModel, workerModel)

	// The watcher follows the run log as the page does and prints each
	// planner decision and task end as it is recorded.
	ready := make(chan string, 1)
	go func() {
		id := waitRunID(ctx, logs)
		fmt.Printf("Run: %s\nPage: http://127.0.0.1:%d/runs/%s\n", id, *port, id)
		ready <- id
		_ = runlog.Read[gimble.LifecycleRecord](ctx, filepath.Join(logs, "runs", id), func(r gimble.LifecycleRecord) error {
			switch e := r.Event.(type) {
			case gimble.PlannerDecision:
				if e.Task.Present {
					fmt.Printf("[log] planner_decision on %s: task %q\n", r.Scope, e.Task.Value.Name)
				} else {
					fmt.Printf("[log] planner_decision on %s: dispatch ended\n", r.Scope)
				}
			case gimble.ScopeEnded:
				if strings.Contains(r.Scope, "/task.") {
					fmt.Printf("[log] scope_ended %s error=%q\n", r.Scope, e.Error)
				}
			}
			return nil
		})
	}()

	runCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	start := time.Now()
	tasks := 0
	var runID string
	runErr := runtime.Run(runCtx, "planner-ends", func(ctx context.Context) error {
		planner := gimble.NewSession(ctx, "planner", codex.New(), plannerModel, workspace)
		loop := gimble.Loop(ctx, "practice", goal, planner)
		for ctx, task := range loop.Tasks {
			if tasks++; tasks > maxTasks {
				fmt.Printf("[lap %d] cap of %d tasks hit; leaving the loop\n", tasks, maxTasks)
				break
			}
			fmt.Printf("[lap %d after %s] task %q: %s\n", tasks, time.Since(start).Round(time.Second), task.Name, oneLine(task.Description))
			if tasks == 2 && runID == "" {
				runID = <-ready
				save(fmt.Sprintf("http://127.0.0.1:%d/api/runs/%s", *port, runID), filepath.Join(base, "snapshot-midrun.json"))
			}
			worker := gimble.NewSession(ctx, "worker", agy.New(), workerModel, workspace)
			result, err := worker.Generate[gimble.Text](ctx, workPrompt+gimble.ScopeText(ctx))
			if err != nil {
				gimble.Set(ctx, "worker error", err.Error())
				fmt.Printf("[lap %d] worker error: %v\n", tasks, err)
			} else {
				gimble.Set(ctx, "worker result", string(result))
				fmt.Printf("[lap %d] worker: %s\n", tasks, oneLine(string(result)))
			}
			gimble.Set(ctx, "files in the workspace", listing(workspace))
		}
		return loop.Err()
	})
	fmt.Printf("Run finished in %s after %d tasks: err=%v\n", time.Since(start).Round(time.Millisecond), tasks, orOK(runErr))
	fmt.Printf("Workspace after the run:\n%s\n", listing(workspace))

	if runID == "" {
		runID = <-ready
	}
	save(fmt.Sprintf("http://127.0.0.1:%d/api/runs/%s", *port, runID), filepath.Join(base, "snapshot.json"))
	save(fmt.Sprintf("http://127.0.0.1:%d/runs/%s", *port, runID), filepath.Join(base, "page.html"))

	fmt.Printf("Holding the server for %s; press Enter to stop early.\n", *hold)
	waitEnter(ctx, *hold)
	if runErr != nil {
		os.Exit(1)
	}
}

// listing is the workspace's files with their sizes, deterministic
// evidence for the planner beside the worker's own claim.
func listing(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err.Error()
	}
	var lines []string
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s (%d bytes)", entry.Name(), info.Size()))
	}
	if len(lines) == 0 {
		return "(empty)"
	}
	return strings.Join(lines, "\n")
}

// waitRunID is the id of the one run under logs, once its directory exists.
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

// save writes what the server answers at url to file.
func save(url, file string) {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("[page] GET %s: %v\n", url, err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
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
