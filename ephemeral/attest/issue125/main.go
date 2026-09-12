// Run with go run ./ephemeral/attest/issue125. Luna plans and performs work in
// a temporary fixture; Haiku independently judges the resulting evidence.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/claude"
	"github.com/tylergannon/gimble/codex"
)

func main() {
	artifacts := flag.String("artifacts", "", "directory for the compact evidence and run log")
	flag.Parse()
	dir, err := os.MkdirTemp("", "gimble-issue125-")
	if err != nil {
		log.Fatal(err)
	}
	if err := makeFixture(dir); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Evidence directory:", dir)
	fmt.Println("Models: planner/worker=gpt-5.6-luna validator=haiku")

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	var evidence []string
	var planChoice gimble.Task
	var independent string
	workerAdapter := codex.New()
	err = gimble.Run(gimble.Project(ctx, dir), "loop-contract", func(ctx context.Context) error {
		if err := gimble.Set(ctx, "prioritized promises", []string{
			"The release check passes for the fixture's primary behavior.",
			"The guide explains what READY means to a user.",
			"After the controlled first probe failure, a later probe passes and establishes stability.",
		}); err != nil {
			return err
		}
		if err := gimble.Set(ctx, "constraint", "Keep the package name calculator and do not change check.sh or probe.sh."); err != nil {
			return err
		}
		initial := runCommand(ctx, dir, "./check.sh")
		if err := gimble.Set(ctx, "initial evidence", initial); err != nil {
			return err
		}

		planner := gimble.NewSession(ctx, "planner", workerAdapter, "gpt-5.6-luna", dir)
		loop := gimble.Loop(ctx, "without-plan", "Satisfy every prioritized promise, including a passing stability probe after its controlled first failure.", planner)
		capped := false
		count := 0
		for taskCtx, task := range loop.Tasks {
			count++
			if count > 4 {
				capped = true
				break
			}
			var result gimble.Text
			var workErr error
			if strings.TrimSpace(task.Validation.Command) == "./probe.sh" {
				result = "This assignment consists of gathering the declared deterministic evidence; the workflow executes it below."
			} else {
				worker := gimble.NewSession(taskCtx, "worker", workerAdapter, "gpt-5.6-luna", dir)
				result, workErr = worker.Generate[gimble.Text](taskCtx,
					"Complete this assignment and gather its evidence.\n\n"+gimble.ScopeText(taskCtx))
			}
			if err := gimble.Set(taskCtx, "worker result", string(result)); err != nil {
				return err
			}
			if workErr != nil {
				if err := gimble.Set(taskCtx, "worker error", workErr.Error()); err != nil {
					return err
				}
			}
			controlledSetup := "unchanged"
			if count == 1 {
				if err := os.Remove(filepath.Join(dir, ".probe-seen")); err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
				controlledSetup = "cleared the transient marker immediately before the first recorded probe"
			}
			if err := gimble.Set(taskCtx, "controlled fixture setup", controlledSetup); err != nil {
				return err
			}
			probe := runCommand(taskCtx, dir, "./probe.sh")
			requested := "not requested"
			if strings.TrimSpace(task.Validation.Command) != "" {
				requested = runCommand(taskCtx, dir, task.Validation.Command)
				if err := gimble.Set(taskCtx, "requested command", requested); err != nil {
					return err
				}
			}
			release := runCommand(taskCtx, dir, "./check.sh")
			if err := gimble.Set(taskCtx, "release check", release); err != nil {
				return err
			}
			if err := gimble.Set(taskCtx, "controlled stability probe", probe); err != nil {
				return err
			}
			evidence = append(evidence, fmt.Sprintf("Task %d: %s\nDescription: %s\nDefinition of done: %s\nControlled setup: %s\nRequested validation: %s\nRelease check: %s\nControlled probe: %s\nWorker: %s",
				count, task.Name, task.Description, task.DefinitionOfDone, controlledSetup, requested, release, probe, result))
		}
		if err := loop.Err(); err != nil {
			return err
		}
		if capped {
			return errors.New("planner did not end dispatch within four tasks")
		}
		if count < 2 {
			return fmt.Errorf("planner dispatched %d task; the controlled failed probe did not produce follow-up work", count)
		}

		if err := gimble.Scope(ctx, "with-plan", func(ctx context.Context) error {
			if err := gimble.Set(ctx, "candidate sprint plan", "Phase 1: polish the optional status copy (87% complete; one nonblocking typo remains).\nPhase 2: implement the missing export, which has no dependency on the typo."); err != nil {
				return err
			}
			if err := gimble.Set(ctx, "priority", "A usable export is the primary promise; status-copy polish is secondary."); err != nil {
				return err
			}
			planPlanner := gimble.NewSession(ctx, "planner", codex.New(), "gpt-5.6-luna", dir)
			planned := gimble.Loop(ctx, "adapt-plan", "Choose the work with the greatest concrete gain toward a usable export and polished status copy.", planPlanner)
			for rangeCtx, task := range planned.Tasks {
				planChoice = task
				if err := gimble.Set(rangeCtx, "selection observed", "The first dispatch is retained as evidence; no work is performed in this planning probe."); err != nil {
					return err
				}
				break
			}
			return planned.Err()
		}); err != nil {
			return err
		}
		if planChoice.Name == "" {
			return errors.New("planner ended the plan-adaptation probe without selecting work")
		}
		evidence = append(evidence, fmt.Sprintf("Plan adaptation choice: %s\nDescription: %s\nDefinition of done: %s", planChoice.Name, planChoice.Description, planChoice.DefinitionOfDone))

		summary := strings.Join(evidence, "\n\n")
		if err := gimble.Set(ctx, "dispatch evidence", summary); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "evidence.txt"), []byte(summary+"\n"), 0o644); err != nil {
			return err
		}
		validator := gimble.NewSession(ctx, "validator", claude.New(), "haiku", dir)
		assessment, err := validator.Generate[gimble.Text](ctx, `Judge the Loop proof in evidence.txt and the fixture itself. The proof passes only if:

1. The no-plan dispatch received the parent constraint and a real failed check, chose useful follow-up work, and ended with ./check.sh passing.
2. The controlled probe's first failure is disclosed rather than counted as a pass, and a later probe passes.
3. The plan-adaptation assignment advances the higher-priority export even though an earlier phase has a nonblocking defect.
4. The assignments are coherent one-session outcomes, with outcome-focused prose rather than pedantic implementation instructions.

Inspect files and run checks as needed. Reply with PASS followed by a concise reason only when every condition is established; otherwise reply with FAIL and the missing evidence.`)
		if err != nil {
			return err
		}
		fmt.Println("Independent assessment:", assessment)
		independent = string(assessment)
		if !strings.HasPrefix(strings.TrimSpace(string(assessment)), "PASS") {
			return fmt.Errorf("independent validator did not pass the evidence: %s", assessment)
		}
		return nil
	})

	final := fmt.Sprintf("Models: planner/worker=gpt-5.6-luna validator=haiku\nRun error: %v\nParent error: %v\nFinal release check: %s\n\n%s\n\nIndependent assessment:\n%s\n", err, ctx.Err(), runCommand(context.Background(), dir, "./check.sh"), strings.Join(evidence, "\n\n"), independent)
	fmt.Print(final)
	if *artifacts != "" {
		if copyErr := retainArtifacts(dir, *artifacts, final); copyErr != nil {
			log.Fatal(copyErr)
		}
	}
	if err != nil || ctx.Err() != nil || !strings.Contains(runCommand(context.Background(), dir, "./check.sh"), "exit 0") {
		os.Exit(1)
	}
}

func makeFixture(dir string) error {
	files := map[string]string{
		"go.mod":     "module example.com/calculator\n\ngo 1.27.1\n",
		"status.txt": "BROKEN\n",
		"guide.txt":  "Calculator guide\n",
		"check.sh": `#!/bin/sh
set -eu
test "$(cat status.txt)" = READY
grep -q 'READY means usable' guide.txt
echo release-ready
`,
		"probe.sh": `#!/bin/sh
set -eu
if [ ! -f .probe-seen ]; then
  touch .probe-seen
  echo 'controlled first observation: re-run to establish stability'
  exit 7
fi
./check.sh
`,
	}
	for name, text := range files {
		mode := os.FileMode(0o644)
		if strings.HasSuffix(name, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), mode); err != nil {
			return err
		}
	}
	return nil
}

func runCommand(ctx context.Context, dir, command string) string {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		} else {
			return fmt.Sprintf("$ %s\nerror: %v\n%s", command, err, tail(string(out)))
		}
	}
	return fmt.Sprintf("$ %s\nexit %d\n%s", command, code, tail(string(out)))
}

func tail(text string) string {
	const limit = 3000
	if len(text) <= limit {
		return text
	}
	return "[...]" + strings.ToValidUTF8(text[len(text)-limit:], "")
}

func retainArtifacts(project, artifacts, summary string) error {
	if err := os.MkdirAll(artifacts, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(artifacts, "result.txt"), []byte(summary), 0o644); err != nil {
		return err
	}
	runs, err := filepath.Glob(filepath.Join(project, "runs", "*", "run.jsonl"))
	if err != nil || len(runs) != 1 {
		return fmt.Errorf("find run log: %v (%d matches)", err, len(runs))
	}
	in, err := os.Open(runs[0])
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(filepath.Join(artifacts, "run.jsonl"))
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
