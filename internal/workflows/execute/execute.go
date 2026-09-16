// Package execute is the execute workflow, the Gimble translation of the
// df-sprint-execute skill: one worker builds one planned sprint from its
// document, the repository's tests run, and the worker reports what it did
// and did not do.
package execute

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/polytype"
)

//go:generate go run github.com/tylergannon/gimble/cmd/gimble gen -entry Execute -name execute

// Input starts the execute workflow.
type Input struct {
	// The sprint to execute: NNN of docs/sprints/SPRINT-NNN.md.
	Sprint int
	// Absolute path of the working directory the sprint is built in.
	WorkDir string
	// The repository's test command, run with sh -c after the worker finishes; absent means go test ./...
	Test polytype.Optional[string]
}

// The roles Execute names, and the model each runs on unless the run's flag
// says otherwise.
var roles = map[string]string{"worker": "gpt-5.6-luna"}

// Execute builds sprint in.Sprint. It names one role, worker, which the run
// binds.
func Execute(ctx context.Context, in Input) error {
	doc := filepath.Join(in.WorkDir, "docs", "sprints", fmt.Sprintf("SPRINT-%03d.md", in.Sprint))
	if _, err := os.Stat(doc); err != nil {
		return fmt.Errorf("execute: no sprint document: %w", err)
	}
	gimble.Set(ctx, "sprint document", doc)
	gimble.Set(ctx, "blockers file", filepath.Join(in.WorkDir, "docs", "sprints", "drafts", fmt.Sprintf("SPRINT-%03d-BLOCKERS.md", in.Sprint)))

	worker := gimble.NewSession(ctx, "worker", in.WorkDir)
	if _, err := worker.Generate[gimble.Text](ctx, executePrompt); err != nil {
		return err
	}

	test := cmp.Or(in.Test.Value, "go test ./...")
	code, stdout, stderr, err := gimble.RunCommand(ctx, "tests", in.WorkDir, "sh", "-c", test)
	if err != nil {
		return err
	}
	output := stdout + stderr
	if len(output) > 3000 {
		output = "[...]" + strings.ToValidUTF8(output[len(output)-3000:], "")
	}
	gimble.Set(ctx, "tests", fmt.Sprintf("$ %s\nexit %d\n%s", test, code, output))
	report, err := worker.Generate[gimble.Text](ctx, reportPrompt)
	if err != nil {
		return err
	}
	log.Printf("execute: sprint %d:\n%s", in.Sprint, report)
	if code != 0 {
		return fmt.Errorf("execute: the tests exited %d", code)
	}
	return nil
}

const executePrompt = `You are executing a sprint plan. Read the sprint document named below and work through every task in its Implementation Plan, phase by phase, in order: implement the change, verify it works with the tests and checks the repository has, and move on to the next task. Follow the project's conventions in AGENTS.md, CLAUDE.md, or equivalent. Do not skip tasks, and do not reorder phases unless a dependency requires it. If you hit a blocker you cannot resolve, write what is blocked and why to the blockers file named below and continue with the next unblocked task. If the sprint is linked to a chapter, read the chapter document too and keep the work aligned with its vector and non-goals, preserving the link in the sprint document. If the sprint document has a Pyramid Index, keep it accurate. Answer with what you completed and what you did not.`

const reportPrompt = `The repository's tests ran as recorded below. Read the blockers file named below if it exists. Report the tasks completed, the blockers, and the test results.`
