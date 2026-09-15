// Run with go run ./ephemeral/attest/issue-216, then read the finished run
// again from a new process with go run ./ephemeral/attest/issue-216 -read.
//
// The live proof for #216 on the cheap tier. The workflow runs a command
// that succeeds, a check that exits nonzero and passes when it is run
// again, a command that cannot start, a command its ctx cancels, and a
// command in each of two parallel children; then a real agent (Codex
// gpt-5.6-luna) reads what the commands printed. Before the agent's turn
// the program asks the page's observation endpoint for the live run and
// prints the commands it serves, and it prints them again once the run has
// ended. -read serves the same project from a new process and prints the
// finished run's commands from the same endpoint, beside commands.json.
package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"maps"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/codex"
	"github.com/tylergannon/gimble/internal/observation"
	"github.com/tylergannon/gimble/web"
)

const model = "gpt-5.6-luna"

func main() {
	port := flag.Int("port", 8216, "loopback TCP port for the web application")
	read := flag.Bool("read", false, "read the finished run from a new process instead of running it")
	flag.Parse()

	base, err := filepath.Abs("ephemeral/attest/issue-216")
	if err != nil {
		log.Fatal(err)
	}
	project := filepath.Join(base, ".gimble")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *read {
		if _, err := web.NewRuntime(ctx, project, web.WithPort(*port)); err != nil {
			log.Fatal(err)
		}
		id := onlyRun(project)
		served := printCommands("after restart, from a new process", *port, id)
		var saved []observation.CommandRow
		raw, err := os.ReadFile(filepath.Join(project, "runs", id, "commands.json"))
		if err == nil {
			err = json.Unmarshal(raw, &saved)
		}
		if err != nil {
			log.Fatal(err)
		}
		same := len(saved) == len(served)
		for _, row := range saved {
			same = same && reflect.DeepEqual(served[row.ID], row)
		}
		fmt.Printf("\nThe endpoint serves the %d rows the run wrote to commands.json: %v\n", len(saved), same)
		return
	}

	if err := os.RemoveAll(project); err != nil {
		log.Fatal(err)
	}
	workspace, err := os.MkdirTemp("", "issue-216-")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(workspace) }()
	runtime, err := web.NewRuntime(ctx, project, web.WithPort(*port))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Model: codex %s. Workspace: %s\n", model, workspace)

	start := time.Now()
	err = runtime.Run(ctx, "issue-216", func(ctx context.Context) error {
		// A command that succeeds.
		_, version, _, err := gimble.RunCommand(ctx, "version", workspace, "go", "version")
		if err != nil {
			return err
		}

		// A check that fails, then passes when it is run again: two records.
		var tries []string
		if err := gimble.Scope(ctx, "retry", func(ctx context.Context) error {
			for try := 1; try <= 2; try++ {
				code, stdout, stderr, err := gimble.RunCommand(ctx, "check", workspace, "sh", "-c",
					`test -f ready || { echo "not ready: $PWD has no file named ready" >&2; touch ready; exit 3; }; echo ready`)
				if err != nil {
					return err
				}
				tries = append(tries, fmt.Sprintf("try %d exited %d; stdout: %q; stderr: %q", try, code, stdout, stderr))
				if code == 0 {
					return nil
				}
			}
			return errors.New("the check never passed")
		}); err != nil {
			return err
		}

		// A command that cannot start.
		_, _, _, missing := gimble.RunCommand(ctx, "missing", workspace, "gimble-no-such-tool", "--help")

		// A command its ctx cancels.
		short, cancel := context.WithTimeout(ctx, time.Second)
		_, _, _, cancelled := gimble.RunCommand(short, "sleep", workspace, "sleep", "60")
		cancel()

		// One command in each of two parallel children. The one that exits
		// nonzero does not fail its sibling or the group.
		group := gimble.Group(ctx, "attempts")
		for _, script := range []string{"echo first attempt; exit 1", "echo second attempt"} {
			group.Go("attempt", func(ctx context.Context) error {
				_, _, _, err := gimble.RunCommand(ctx, "check", workspace, "sh", "-c", script)
				return err
			})
		}
		if err := group.Wait(); err != nil {
			return err
		}

		printCommands("live, before the agent's turn", *port, onlyRun(project))

		// An agent reads what the commands printed.
		reader := gimble.NewSession(ctx, "reader", codex.New(), model, workspace)
		answer, err := reader.Generate[gimble.Text](ctx, fmt.Sprintf(readerPrompt, workspace, version, strings.Join(tries, "\n"), missing, cancelled))
		if err != nil {
			return err
		}
		fmt.Printf("\nThe agent's answer:\n%s\n", answer)
		return nil
	})
	fmt.Printf("\nRun finished in %s: err=%v\n", time.Since(start).Round(time.Millisecond), err)
	if err != nil {
		os.Exit(1)
	}
	printCommands("after the run, from the same process", *port, onlyRun(project))
}

const readerPrompt = `A workflow ran some commands in %s. This is what they printed. Use no tools.

go version: %s
A check, run twice:
%s
gimble-no-such-tool --help: %v
sleep 60 with a one-second deadline: %v

Answer in four short lines: which Go version is installed; what the check said was wrong the first time, and whether it passed the second time; why the missing tool did not run; and why the sleep did not finish.`

// onlyRun is the id of the one run under the project.
func onlyRun(project string) string {
	entries, err := os.ReadDir(filepath.Join(project, "runs"))
	if err != nil || len(entries) != 1 {
		log.Fatalf("runs under %s: %v, %v", project, entries, err)
	}
	return entries[0].Name()
}

// printCommands asks the page's observation endpoint for the run, as the
// page does, and prints the commands it serves in the order they started.
func printCommands(when string, port int, id string) map[string]observation.CommandRow {
	response, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/runs/%s", port, id))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	var snapshot observation.RunSnapshot
	if err := json.NewDecoder(response.Body).Decode(&snapshot); err != nil {
		log.Fatal(err)
	}
	rows := slices.Collect(maps.Values(snapshot.Commands))
	slices.SortFunc(rows, func(a, b observation.CommandRow) int {
		return cmp.Or(cmp.Compare(a.Started, b.Started), cmp.Compare(a.ID, b.ID))
	})
	fmt.Printf("\nGET /api/runs/%s (%s, run %s): %d commands\n", id, when, snapshot.Run.Status, len(rows))
	for _, row := range rows {
		ended := "running"
		switch {
		case row.Ended == 0:
		case row.Interrupted:
			ended = "cancelled: " + row.Error
		case row.Error != "":
			ended = "did not start: " + row.Error
		default:
			ended = fmt.Sprintf("exit %d", row.ExitCode)
		}
		fmt.Printf("- %-30s scope %-22q %-40s %s after %d ms; stdout %q stderr %q\n",
			row.ID, row.Scope, strings.Join(append([]string{row.Command}, row.Args...), " "), ended, row.Duration,
			strings.TrimSpace(row.Stdout), strings.TrimSpace(row.Stderr))
	}
	return snapshot.Commands
}
