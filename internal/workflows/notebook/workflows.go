// Package notebook contains two bounded, informational workflow examples.
// They are learning/evaluation material, independent of release or deployment gates.
package notebook

import (
	"context"
	"fmt"
	"github.com/tylergannon/gimbal"
)

//go:generate go tool polytype --validate

// Route connects a reader's question to evidence in the supplied corpus.
type Route struct {
	Question string   `json:"question"`
	Sources  []string `json:"sources"`
	Answer   string   `json:"answer"`
}

// Index is a compact question-driven map, not a vector database.
type Index struct {
	Routes []Route `json:"routes"`
}

// Task is a bounded delivery outcome with dependencies and acceptance evidence.
type Task struct {
	ID         string   `json:"id"`
	Outcome    string   `json:"outcome"`
	DependsOn  []string `json:"depends_on"`
	Acceptance string   `json:"acceptance"`
}

// Sprint is an ordered plan for the supplied goal and constraints.
type Sprint struct {
	Goal  string `json:"goal"`
	Tasks []Task `json:"tasks"`
}

// Review is an independent assessment against the original fixture.
type Review struct {
	Findings []string `json:"findings"`
}

// SemanticIndex creates a retrieval map and gives it to a fresh reviewer.
func SemanticIndex(ctx context.Context, env gimbal.Env, corpus string) (Index, error) {
	gimbal.Set(ctx, "corpus", corpus)
	author := gimbal.NewSession(ctx, gimbal.RoleBulkClassification, env.WorkDir)
	index, err := author.Generate[Index](ctx, indexPrompt)
	if err != nil {
		return Index{}, err
	}
	if err := ValidateIndex(index); err != nil {
		return Index{}, err
	}
	gimbal.SetJSON(ctx, "candidate", index)
	reviewer := gimbal.NewSession(ctx, gimbal.RoleCodeReview, env.WorkDir)
	review, err := reviewer.Generate[Review](ctx, indexReview)
	if err != nil {
		return Index{}, err
	}
	gimbal.SetJSON(ctx, "review", review)
	if len(review.Findings) != 0 {
		return Index{}, fmt.Errorf("index review: %v", review.Findings)
	}
	return index, nil
}

// SprintPlan creates an actionable plan and gives it to a fresh reviewer.
func SprintPlan(ctx context.Context, env gimbal.Env, brief string) (Sprint, error) {
	gimbal.Set(ctx, "brief", brief)
	planner := gimbal.NewSession(ctx, gimbal.RoleSprintPlanning, env.WorkDir)
	plan, err := planner.Generate[Sprint](ctx, sprintPrompt)
	if err != nil {
		return Sprint{}, err
	}
	if err := ValidateSprint(plan); err != nil {
		return Sprint{}, err
	}
	gimbal.SetJSON(ctx, "candidate", plan)
	reviewer := gimbal.NewSession(ctx, gimbal.RoleCodeReview, env.WorkDir)
	review, err := reviewer.Generate[Review](ctx, sprintReview)
	if err != nil {
		return Sprint{}, err
	}
	gimbal.SetJSON(ctx, "review", review)
	if len(review.Findings) != 0 {
		return Sprint{}, fmt.Errorf("sprint review: %v", review.Findings)
	}
	return plan, nil
}

const indexPrompt = `Build a compact semantic index of the corpus below. Return four question-driven routes: starting a local run, isolation of concurrent app/database stacks, ownership of workflow semantics, and updating a backend while runs are active. Each answer must cite the relevant supplied source IDs, distinguish requirements from demonstrated support, and be useful to a reader choosing what to open next. Do not invent capabilities. Use only the supplied corpus. Make no file changes.`
const indexReview = `Independently inspect the candidate against the original corpus. Check all four requested retrieval questions, exact source attribution, requirements versus proven support, and unsupported promises. Return concrete findings, or an empty findings list when the index is faithful. Make no changes.`
const sprintPrompt = `Turn the supplied brief into a small sprint of three to five tasks. Name bounded outcomes, earlier task dependencies, and observable acceptance evidence. Cover the local baseline, concurrent stack isolation, cancellation and cleanup, and human-legible observation. Respect the budget and exclusions. This is a proposed plan, not completed work. Make no file changes.`
const sprintReview = `Independently inspect the candidate against the original brief. Check complete coverage, feasible ordering, concrete acceptance, budget, and exclusions. Reject claims of completed implementation, universal restartability, or mandatory Docker for basic local workflows. Return concrete findings, or an empty findings list when the plan is actionable. Make no changes.`
