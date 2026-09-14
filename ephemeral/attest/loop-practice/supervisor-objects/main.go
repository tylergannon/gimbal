// Run with go run ./ephemeral/attest/loop-practice/supervisor-objects. Loop
// practice, shape 3 (#178): a supervisor objects mid-turn and the steer
// lands. A Codex planner (gpt-5.6-luna) dispatches tasks toward six small
// note files; a Codex worker (gpt-5.6-luna) writes them one shell command
// at a time with a sleep between, so the turn has many model calls for a
// steer to land at; an Antigravity supervisor (gemini-3.8-flash-low) looks
// every eight seconds for a house rule the worker was never told, and its
// objection is steered into the turn. The supervisor's Steer result is not
// returned to the workflow, so the program follows run.jsonl live and prints
// each Steer record's source and landed as it is written. After the run it
// lists which files carry the footer the rule demands.
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
	plannerModel    = "gpt-5.6-luna"
	workerModel     = "gpt-5.6-luna"
	supervisorModel = "gemini-3.8-flash-low"
	maxTasks        = 3
	lookEvery       = 8 * time.Second
	footer          = "-- source: memory"

	goal = "In this directory, write six note files, planet1.txt through planet6.txt, about Mercury, Venus, Earth, Mars, Jupiter, and Saturn in that order, one sentence each. Plan this as a single task that writes all six files; when they exist, end dispatch."

	workPrompt = "Complete the task below in the current directory. Write each file with its own shell command, and run the shell command `sleep 3` between files, so every step is its own tool call. When done, answer with the list of files you wrote, one per line, and nothing else.\n\n"

	watchFor = "House rule for this directory: every file the agent writes must end with a final line reading exactly `" + footer + "`. The agent was not told this. Object the first time you see it write a file without that line: tell it to end every remaining file with that line and to append the line to the files it already wrote."
)

func main() {
	port := flag.Int("port", 8183, "loopback TCP port for the web application")
	hold := flag.Duration("hold", 3*time.Minute, "how long to hold the server open after the run")
	flag.Parse()

	base, err := filepath.Abs("ephemeral/attest/loop-practice/supervisor-objects")
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
	fmt.Printf("Models: planner=codex/%s worker=codex/%s supervisor=agy/%s\n", plannerModel, workerModel, supervisorModel)

	// The watcher follows the run log and prints every steer as it lands or
	// is dropped, plus a mid-run snapshot at the first steer.
	ready := make(chan string, 1)
	steers := 0
	go func() {
		id := waitRunID(ctx, logs)
		fmt.Printf("Run: %s\nPage: http://127.0.0.1:%d/runs/%s\n", id, *port, id)
		ready <- id
		_ = runlog.Read[gimble.LifecycleRecord](ctx, filepath.Join(logs, "runs", id), func(r gimble.LifecycleRecord) error {
			switch e := r.Event.(type) {
			case gimble.SuperviseAttached:
				fmt.Printf("[log] supervise_attached reviewer=%s worker=%s interval=%s\n", e.Reviewer, e.Worker, e.Interval)
			case gimble.Steer:
				steers++
				fmt.Printf("[log] steer #%d from %s to %s on %s: landed=%v message=%s\n", steers, e.Source, e.Target, r.Turn.Value, e.Landed, oneLine(e.Message))
				if steers == 1 {
					save(fmt.Sprintf("http://127.0.0.1:%d/api/runs/%s", *port, id), filepath.Join(base, "snapshot-midrun.json"))
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
	runErr := runtime.Run(runCtx, "supervisor-objects", func(ctx context.Context) error {
		planner := gimble.NewSession(ctx, "planner", codex.New(), plannerModel, workspace)
		loop := gimble.Loop(ctx, "practice", goal, planner)
		for ctx, task := range loop.Tasks {
			if tasks++; tasks > maxTasks {
				fmt.Printf("[lap %d] cap of %d tasks hit; leaving the loop\n", tasks, maxTasks)
				break
			}
			fmt.Printf("[lap %d after %s] task %q: %s\n", tasks, time.Since(start).Round(time.Second), task.Name, oneLine(task.Description))
			worker := gimble.NewSession(ctx, "worker", codex.New(), workerModel, workspace)
			supervisor := gimble.NewSession(ctx, "supervisor", agy.New(), supervisorModel, workspace)
			result, err := worker.Generate[gimble.Text](ctx, workPrompt+gimble.ScopeText(ctx),
				gimble.WithSupervisor(supervisor, watchFor, gimble.WithInterval(lookEvery)),
			)
			if err != nil {
				gimble.Set(ctx, "worker error", err.Error())
				fmt.Printf("[lap %d] worker error: %v\n", tasks, err)
			} else {
				gimble.Set(ctx, "worker result", string(result))
				fmt.Printf("[lap %d] worker: %s\n", tasks, oneLine(string(result)))
			}
			gimble.Set(ctx, "files in the workspace", footers(workspace))
		}
		return loop.Err()
	})
	fmt.Printf("Run finished in %s after %d tasks and %d steers: err=%v\n", time.Since(start).Round(time.Millisecond), tasks, steers, orOK(runErr))
	fmt.Printf("Workspace after the run:\n%s\n", footers(workspace))

	id := <-ready
	save(fmt.Sprintf("http://127.0.0.1:%d/api/runs/%s", *port, id), filepath.Join(base, "snapshot.json"))
	save(fmt.Sprintf("http://127.0.0.1:%d/runs/%s", *port, id), filepath.Join(base, "page.html"))

	fmt.Printf("Holding the server for %s; press Enter to stop early.\n", *hold)
	waitEnter(ctx, *hold)
	if runErr != nil {
		os.Exit(1)
	}
}

// footers lists each file in dir and whether its last line is the footer
// the supervisor's rule demands.
func footers(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err.Error()
	}
	var lines []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		has := strings.HasSuffix(strings.TrimRight(string(raw), "\n"), footer)
		lines = append(lines, fmt.Sprintf("%s: footer=%v", entry.Name(), has))
	}
	if len(lines) == 0 {
		return "(empty)"
	}
	return strings.Join(lines, "\n")
}

// waitRunID is the id of the one run under logs, once its run.jsonl exists.
// The run directory appears before the log's first record, and runlog.Read
// fails at once on a directory with no run.jsonl, so waiting for the
// directory alone is a race.
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
