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
	var firstProbeFailed, laterProbePassed bool
	workerAdapter := codex.New()
	err = gimble.Run(gimble.Project(ctx, dir), "loop-contract", func(ctx context.Context) error {
		gimble.Set(ctx, "prioritized promises", []string{
			"The release check passes for the fixture's primary behavior.",
			"The guide explains what READY means to a user.",
			"If the stability probe exposes a regression, the fixture is repaired and a later probe passes.",
		})
		gimble.Set(ctx, "constraint", "Keep the package name calculator and do not change check.sh or probe.sh.")
		initial := runCommand(ctx, dir, "./check.sh")
		gimble.Set(ctx, "initial evidence", initial)

		planner := gimble.NewSession(ctx, "planner", workerAdapter, "gpt-5.6-luna", dir)
		loop := gimble.Loop(ctx, "without-plan", "Satisfy every prioritized promise, including repairing any regression exposed by the stability probe.", planner)
		capped := false
		count := 0
		for taskCtx, task := range loop.Tasks {
			count++
			if count > 4 {
				capped = true
				break
			}
			worker := gimble.NewSession(taskCtx, "worker", workerAdapter, "gpt-5.6-luna", dir)
			result, workErr := worker.Generate[gimble.Text](taskCtx,
				"Complete this assignment. The workflow will gather validation after your turn.\n\n"+gimble.ScopeText(taskCtx))
			gimble.Set(taskCtx, "worker result", string(result))
			if workErr != nil {
				gimble.Set(taskCtx, "worker error", workErr.Error())
			}
			controlledSetup := "unchanged"
			if count == 1 {
				if err := os.WriteFile(filepath.Join(dir, "stability.txt"), []byte("UNSTABLE\n"), 0o644); err != nil {
					return err
				}
				controlledSetup = "introduced the disclosed stability regression immediately before the first recorded probe"
			}
			gimble.Set(taskCtx, "controlled fixture setup", controlledSetup)
			probe := runCommand(taskCtx, dir, "./probe.sh")
			if count == 1 && strings.Contains(probe, "exit 7") {
				firstProbeFailed = true
			}
			if count > 1 && strings.Contains(probe, "exit 0") {
				laterProbePassed = true
			}
			requested := "not requested"
			if strings.TrimSpace(task.Validation.Command) != "" {
				requested = runCommand(taskCtx, dir, task.Validation.Command)
				gimble.Set(taskCtx, "requested command", requested)
			}
			release := runCommand(taskCtx, dir, "./check.sh")
			gimble.Set(taskCtx, "release check", release)
			gimble.Set(taskCtx, "controlled stability probe", probe)
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
			return fmt.Errorf("planner dispatched %d task; the failed stability probe did not produce repair work", count)
		}
		if !firstProbeFailed || !laterProbePassed {
			return fmt.Errorf("stability evidence incomplete: first failure=%t later pass=%t", firstProbeFailed, laterProbePassed)
		}

		if err := gimble.Scope(ctx, "with-plan", func(ctx context.Context) error {
			candidatePlan := "Phase 1: finish polishing the optional status copy before Phase 2 begins. The status display works and is 87% complete; one nonblocking typo remains.\nPhase 2: after Phase 1 is perfect, implement the missing public export required by downstream callers."
			gimble.Set(ctx, "candidate sprint plan", candidatePlan)
			gimble.Set(ctx, "current evidence", "The status display works despite its optional typo. A downstream caller cannot compile because the public export does not exist.")
			planPlanner := gimble.NewSession(ctx, "planner", codex.New(), "gpt-5.6-luna", dir)
			planned := gimble.Loop(ctx, "adapt-plan", "Choose the work with the greatest concrete gain toward a usable export and polished status copy.", planPlanner)
			for rangeCtx, task := range planned.Tasks {
				planChoice = task
				gimble.Set(rangeCtx, "selection observed", "The first dispatch is retained as evidence; no work is performed in this planning probe.")
				break
			}
			return planned.Err()
		}); err != nil {
			return err
		}
		if planChoice.Name == "" {
			return errors.New("planner ended the plan-adaptation probe without selecting work")
		}
		if !strings.Contains(strings.ToLower(planChoice.Name+" "+planChoice.Description), "export") {
			return fmt.Errorf("planner followed the candidate plan's Phase 1 gate instead of adapting toward the missing export: %+v", planChoice)
		}
		evidence = append(evidence, fmt.Sprintf("Candidate plan before dispatch: Phase 1 must be perfect before Phase 2 begins; Phase 1 works but has a nonblocking typo, while Phase 2's public export is absent.\nPlan adaptation selection (planning evidence only; the task is not claimed complete): %s\nDescription: %s\nDefinition of done: %s", planChoice.Name, planChoice.Description, planChoice.DefinitionOfDone))

		summary := strings.Join(evidence, "\n\n")
		gimble.Set(ctx, "dispatch evidence", summary)
		if err := os.WriteFile(filepath.Join(dir, "evidence.txt"), []byte(summary+"\n"), 0o644); err != nil {
			return err
		}
		validator := gimble.NewSession(ctx, "validator", claude.New(), "haiku", dir)
		assessment, err := validator.Generate[gimble.Text](ctx, `Judge the Loop proof in evidence.txt and the fixture itself. The proof passes only if:

1. The no-plan dispatch received the parent constraint and a real failed check, then sent a worker a useful repair assignment and ended with ./check.sh passing.
2. The disclosed stability regression produced a real probe exit 7, and a later worker repaired the fixture so a later probe passed.
3. The plan-adaptation dispatcher departs from the supplied Phase 1 gate by selecting the missing-export assignment even though the earlier phase has a nonblocking defect. This planning probe tests task selection, not task completion; do not require or infer that the export was implemented.
4. The assignments are coherent one-session outcomes, with outcome-focused prose rather than pedantic implementation instructions.

Inspect files and run checks as needed. Reply with PASS followed by a concise reason only when every condition is established; otherwise reply with FAIL and the missing evidence.`)
		if err != nil {
			return err
		}
		fmt.Println("Independent assessment:", assessment)
		independent = string(assessment)
		if !assessmentPassed(string(assessment)) {
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

func assessmentPassed(text string) bool {
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PASS") || strings.HasPrefix(line, "**PASS**") {
			return true
		}
	}
	return false
}

func makeFixture(dir string) error {
	files := map[string]string{
		"go.mod":        "module example.com/calculator\n\ngo 1.27.1\n",
		"status.txt":    "BROKEN\n",
		"guide.txt":     "Calculator guide\n",
		"stability.txt": "STABLE\n",
		"check.sh": `#!/bin/sh
set -eu
test "$(cat status.txt)" = READY
grep -q 'READY means usable' guide.txt
echo release-ready
`,
		"probe.sh": `#!/bin/sh
set -eu
if [ "$(cat stability.txt)" != STABLE ]; then
  echo 'stability regression: expected STABLE'
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
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
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
	defer func() { _ = in.Close() }()
	out, err := os.Create(filepath.Join(artifacts, "run.jsonl"))
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
