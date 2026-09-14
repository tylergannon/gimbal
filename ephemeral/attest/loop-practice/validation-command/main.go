// Run with go run ./ephemeral/attest/loop-practice/validation-command. Loop
// practice, shape 2 (#178): a validation command decides. A Codex planner
// (gpt-5.6-luna) dispatches tasks, each of which must carry a
// Validation.Command; an Antigravity worker (gemini-3.8-flash-low) does the
// task, and then the workflow itself runs the command with sh in the
// workspace and records its exit code and output. The exit code alone
// decides whether the task passed; the worker's own claim is recorded
// beside it, never in its place. A task with no command is recorded as
// rejected and no worker runs. The run is served by web.NewRuntime on
// -port; the page's snapshot JSON and HTML are saved after the run.
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
	"os/exec"
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

	goal = "In this directory, build a tiny shell project: data.txt holding exactly twelve lines, each a different fruit name; and count.sh, executable, which prints the number of lines in data.txt and nothing else. Every task must carry a validation command in validation.command, to be run with sh from this directory; the workflow runs it after the worker finishes and its exit code alone decides whether the task passed. A task without a command is rejected without running. Plan the data file and the script as separate tasks, the data file first. End dispatch when the recorded commands prove the whole project."

	workPrompt = "Complete the task below in the current directory. Answer with one sentence saying what you did and whether you believe the task's definition of done is met.\n\n"
)

func main() {
	port := flag.Int("port", 8182, "loopback TCP port for the web application")
	hold := flag.Duration("hold", 3*time.Minute, "how long to hold the server open after the run")
	flag.Parse()

	base, err := filepath.Abs("ephemeral/attest/loop-practice/validation-command")
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

	ready := make(chan string, 1)
	go func() {
		id := waitRunID(ctx, logs)
		fmt.Printf("Run: %s\nPage: http://127.0.0.1:%d/runs/%s\n", id, *port, id)
		ready <- id
		_ = runlog.Read[gimble.LifecycleRecord](ctx, filepath.Join(logs, "runs", id), func(r gimble.LifecycleRecord) error {
			switch e := r.Event.(type) {
			case gimble.PlannerDecision:
				if e.Task.Present {
					fmt.Printf("[log] planner_decision on %s: task %q command=%q\n", r.Scope, e.Task.Value.Name, e.Task.Value.Validation.Command)
				} else {
					fmt.Printf("[log] planner_decision on %s: dispatch ended\n", r.Scope)
				}
			case gimble.ValueSet:
				if e.Key == "passed" || e.Key == "validation command" {
					fmt.Printf("[log] value_set %s %s=%s\n", r.Scope, e.Key, oneLine(string(e.Value)))
				}
			}
			return nil
		})
	}()

	runCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	start := time.Now()
	tasks, passed, failed, rejected := 0, 0, 0, 0
	var runID string
	runErr := runtime.Run(runCtx, "validation-command", func(ctx context.Context) error {
		planner := gimble.NewSession(ctx, "planner", codex.New(), plannerModel, workspace)
		loop := gimble.Loop(ctx, "practice", goal, planner)
		for ctx, task := range loop.Tasks {
			if tasks++; tasks > maxTasks {
				fmt.Printf("[lap %d] cap of %d tasks hit; leaving the loop\n", tasks, maxTasks)
				break
			}
			fmt.Printf("[lap %d after %s] task %q command=%q\n", tasks, time.Since(start).Round(time.Second), task.Name, task.Validation.Command)
			if tasks == 2 && runID == "" {
				runID = <-ready
				save(fmt.Sprintf("http://127.0.0.1:%d/api/runs/%s", *port, runID), filepath.Join(base, "snapshot-midrun.json"))
			}
			if strings.TrimSpace(task.Validation.Command) == "" {
				rejected++
				gimble.Set(ctx, "validation command", "rejected: the task carries no validation command, so no worker ran and nothing was checked")
				gimble.Set(ctx, "passed", false)
				fmt.Printf("[lap %d] rejected: no validation command\n", tasks)
				continue
			}
			worker := gimble.NewSession(ctx, "worker", agy.New(), workerModel, workspace)
			result, err := worker.Generate[gimble.Text](ctx, workPrompt+gimble.ScopeText(ctx))
			if err != nil {
				gimble.Set(ctx, "worker error", err.Error())
				fmt.Printf("[lap %d] worker error: %v\n", tasks, err)
			} else {
				gimble.Set(ctx, "worker claim", string(result))
				fmt.Printf("[lap %d] worker claims: %s\n", tasks, oneLine(string(result)))
			}
			code, output, err := command(ctx, workspace, task.Validation.Command)
			if err != nil {
				return err
			}
			gimble.Set(ctx, "validation command", fmt.Sprintf("$ %s\nexit %d\n%s", task.Validation.Command, code, output))
			gimble.Set(ctx, "passed", code == 0)
			if code == 0 {
				passed++
			} else {
				failed++
			}
			fmt.Printf("[lap %d] validation exit %d: %s\n", tasks, code, oneLine(output))
		}
		return loop.Err()
	})
	fmt.Printf("Run finished in %s: tasks=%d passed=%d failed=%d rejected=%d err=%v\n", time.Since(start).Round(time.Millisecond), tasks, passed, failed, rejected, orOK(runErr))
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

// command runs text with sh in dir, bounded to a minute, and returns its
// exit code and combined output. Only a failure to run it at all is an error.
func command(ctx context.Context, dir, text string) (int, string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", text)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	output := string(out)
	if len(output) > 3000 {
		output = "[...]" + strings.ToValidUTF8(output[len(output)-3000:], "")
	}
	if err == nil {
		return 0, output, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), output, nil
	}
	return 0, "", fmt.Errorf("validation command %q: %w", text, err)
}

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
		lines = append(lines, fmt.Sprintf("%s (%d bytes, %s)", entry.Name(), info.Size(), info.Mode().Perm()))
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
