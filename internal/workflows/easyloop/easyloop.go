// Package easyloop is the easy loop workflow, the Gimble translation of the
// df-easy-loop-simple skill on Loop. A planner writes the plan for a spec
// document as a checklist, a critic on another harness critiques it, and the
// planner revises it. Then the planner dispatches the plan's work to a coder
// one task at a time, and a reviewer judges each task by running the
// software, ticks the plan's boxes, and ends the loop when the spec is met.
package easyloop

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

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimble/cmd gen -entry EasyLoop -name easyloop

// Input starts the easy loop.
type Input struct {
	// Path of the spec document that says what is being built.
	Spec string
	// Absolute path of the repository the work is done in.
	Repo string
	// The most tasks to run in all; absent means 50.
	Tasks polytype.Optional[int]
}

// review is what the reviewer reports after a task.
type review struct {
	// What the task's work did not show working, one finding each. Empty when everything the task asked for was seen working.
	NotSeenWorking []string `json:"not_seen_working"`
	// Whether every requirement of the spec document is now met, judged by your own testing. True ends the loop.
	SpecMet bool `json:"spec_met"`
}

const goal = "Build what the spec document asks, as updated-plan.md in the plan directory says; both are named in the context."

// EasyLoop builds what in.Spec asks. It names four roles, which the run
// binds: planner, critic, coder, and reviewer.
func EasyLoop(ctx context.Context, in Input) error {
	spec, err := filepath.Abs(in.Spec)
	if err != nil {
		return err
	}
	limit := cmp.Or(in.Tasks.Value, 50)
	plans := filepath.Join(in.Repo, "docs", "plans", strings.TrimSuffix(filepath.Base(spec), filepath.Ext(spec)))
	if err := os.MkdirAll(plans, 0o755); err != nil {
		return err
	}
	gimble.Set(ctx, "spec document", spec)
	gimble.Set(ctx, "plan directory", plans)

	planner := gimble.NewSession(ctx, "planner", in.Repo)
	critic := gimble.NewSession(ctx, "critic", in.Repo)
	if _, err := planner.Generate[gimble.Text](ctx, planPrompt); err != nil {
		return err
	}
	if _, err := critic.Generate[gimble.Text](ctx, critiquePrompt); err != nil {
		return err
	}
	if _, err := planner.Generate[gimble.Text](ctx, updatePrompt); err != nil {
		return err
	}

	// The plan is written. The planner dispatches its work to the coder, and
	// the reviewer judges each task and ends the loop when the spec is met.
	coder := gimble.NewSession(ctx, "coder", in.Repo)
	reviewer := gimble.NewSession(ctx, "reviewer", in.Repo)
	loop := gimble.Loop(ctx, "work", goal, planner)
	tasks, met := 0, false
	for ctx, task := range loop.Tasks {
		if tasks++; tasks > limit {
			break
		}
		report, err := coder.Generate[gimble.Text](ctx, codePrompt)
		if err != nil {
			return err
		}
		gimble.Set(ctx, "coder's report", string(report))
		verdict, err := reviewer.Generate[review](ctx, reviewPrompt)
		if err != nil {
			return err
		}
		gimble.SetJSON(ctx, "review", verdict)
		log.Printf("easyloop: task %q: %d findings; spec met: %t", task.Name, len(verdict.NotSeenWorking), verdict.SpecMet)
		if verdict.SpecMet {
			met = true
			break
		}
	}
	if err := loop.Err(); err != nil {
		return err
	}
	if !met {
		return fmt.Errorf("easyloop: the reviewer had not seen the spec met after %d tasks", tasks)
	}
	return nil
}

const planPrompt = `Read the spec document named below, do focused reconnaissance of the repository, and write the plan to plan.md in the plan directory named below: Markdown checklist items, one "- [ ]" box per task.`

const critiquePrompt = `Read the spec document and plan.md in the plan directory named below, and write plan-critique.md there: where the plan misreads the spec, what it misses, and what it builds that the spec does not ask for.`

const updatePrompt = `Read the spec document, plan.md, and plan-critique.md in the plan directory named below, and write updated-plan.md there: the plan revised for the critique, still Markdown checklist items with only unchecked "- [ ]" boxes, taking only the changes that derisk the work or make it better tested.`

const codePrompt = `Do the task below. Read the spec document and updated-plan.md in the plan directory named below first. Test what you write and run the repository's standard tests. Commit each change with git add of specific paths, never wildcards or -A. Do not edit updated-plan.md or tick any box. Answer with what changed, the evidence you gathered, and the exact text of each plan item you believe is done.`

const reviewPrompt = `Judge the task's work below by your own testing: run the software and the repository's standard tests, and never take the coder's report as evidence. In updated-plan.md in the plan directory named below, tick the box of each item you saw complete; that is the only edit you may make there. Write review.md there saying what you found. Report what you did not see working, and whether every requirement of the spec document is now met. When it is, commit updated-plan.md and review.md with git add of their paths.`
