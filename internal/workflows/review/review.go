// Package review is a small read-only code review workflow.
package review

import (
	"context"

	"github.com/tylergannon/gimble"
)

//go:generate go run github.com/tylergannon/gimble/internal/generate/gimblegen -entry Review -name review

// Input starts a review of the repository at WorkDir.
type Input struct {
	// WorkDir is the absolute path of the repository to review.
	WorkDir string
	// Goal says what the review should assess.
	Goal string
}

// Result is the reviewer's list of concrete correctness findings.
type Result struct {
	// Findings lists concrete correctness issues found in the code, one per entry.
	Findings []string `json:"findings"`
}

// Review asks one reviewer to inspect the repository read-only and record its findings.
func Review(ctx context.Context, in Input) error {
	gimble.Set(ctx, "goal", in.Goal)
	reviewer := gimble.NewSession(ctx, "reviewer", in.WorkDir)
	result, err := reviewer.Generate[Result](ctx, reviewPrompt)
	if err != nil {
		return err
	}
	gimble.SetJSON(ctx, "result", result)
	return nil
}

const reviewPrompt = "Read the code in your working directory. Assess the goal given below. Report concrete correctness issues; return an empty findings list when there are none. Make no changes."
