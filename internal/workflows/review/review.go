// Package review is a small read-only code review workflow.
package review

import (
	"context"

	"github.com/tylergannon/gimbal"
)

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimbal/internal/generate/gimbalgen -entry Review -name review

// ReviewParams describes what to review.
type ReviewParams struct {
	// Goal says what the review should assess.
	Goal string
}

// Result is the reviewer's list of concrete correctness findings.
type Result struct {
	// Findings lists concrete correctness issues found in the code, one per entry.
	Findings []string `json:"findings"`
}

// Review asks one reviewer to inspect the repository read-only and record its findings.
func Review(ctx context.Context, env gimbal.Env, params ReviewParams) error {
	gimbal.Set(ctx, "goal", params.Goal)
	reviewer := gimbal.NewSession(ctx, gimbal.RoleCodeReview, env.WorkDir)
	result, err := reviewer.Generate[Result](ctx, reviewPrompt)
	if err != nil {
		return err
	}
	gimbal.SetJSON(ctx, "result", result)
	return nil
}

const reviewPrompt = "Read the code in your working directory. Assess the goal given below. Report concrete correctness issues; return an empty findings list when there are none. Make no changes."
