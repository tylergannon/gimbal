// Package review is a small read-only code review workflow.
//
// Use it to assess a bounded correctness question about an existing repository
// before making changes or accepting an implementation. The goal tells the
// reviewer which behavior or files to assess; the working directory supplies
// the repository to inspect. It does not edit code or create issues or pull requests.
//
// Start a Gimbal instance and authenticate the configured agent harness before
// invoking the workflow. The Run view lets you choose the review model explicitly.
//
// The reviewer returns a list of concrete findings, saved as result in the
// workflow context. An empty list means the reviewer found no correctness issues
// within the stated goal. A completed run means the review was recorded, not that
// its findings were resolved. A failed agent turn fails the workflow.
//
// Example:
//
//	gimbal run review --project /abs/project --goal "Review the current changes"
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
