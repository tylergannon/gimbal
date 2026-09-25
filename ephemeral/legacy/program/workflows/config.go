// Package workflows contains the small, directly runnable programs that
// complement Gimbal's shipped YAML workflow graphs. The programs deliberately
// express their control flow in Go; they do not interpret a graph at runtime.
package workflows

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimbal/program"
)

// SprintExecuteInput selects the sprint ledger and the agents used to work it.
// The checklist is relative to the runtime workdir unless it is absolute.
type SprintExecuteInput struct {
	Goal           string `json:"goal"`
	Checklist      string `json:"checklist"`
	ImplementModel string `json:"implement_model,omitempty"`
	ReviewModel    string `json:"review_model,omitempty"`
	EvaluateModel  string `json:"evaluate_model,omitempty"`
	MaxIterations  int    `json:"max_iterations,omitempty"`
}

// ChapterLoopInput selects a chapter ledger. Each open chapter item must name
// its sprint checklist through checklist: in its ledger entry.
type ChapterLoopInput struct {
	Goal                 string `json:"goal"`
	Checklist            string `json:"checklist"`
	ImplementModel       string `json:"implement_model,omitempty"`
	ReviewModel          string `json:"review_model,omitempty"`
	EvaluateModel        string `json:"evaluate_model,omitempty"`
	ChapterMaxIterations int    `json:"chapter_max_iterations,omitempty"`
	SprintMaxIterations  int    `json:"sprint_max_iterations,omitempty"`
}

// DeliveryLoopInput turns a written specification into a plan checklist and
// then works it. Plan, critique, coding, and review are separate turns.
type DeliveryLoopInput struct {
	Goal          string `json:"goal"`
	Checklist     string `json:"checklist,omitempty"`
	PlanModel     string `json:"plan_model,omitempty"`
	CritiqueModel string `json:"critique_model,omitempty"`
	CodingModel   string `json:"coding_model,omitempty"`
	ReviewModel   string `json:"review_model,omitempty"`
	EvaluateModel string `json:"evaluate_model,omitempty"`
	MaxIterations int    `json:"max_iterations,omitempty"`
}

const (
	defaultSprintIterations  = 40
	defaultChapterIterations = 12
)

type ledgerRun struct {
	Goal, Checklist, ImplementModel, ReviewModel, EvaluateModel, ImplementPrompt, ReviewPrompt string
	WorkContext                                                                                string
	MaxIterations                                                                              int
}

func prepareSprint(runtime *program.Runtime, input SprintExecuteInput) (ledgerRun, error) {
	if runtime == nil {
		return ledgerRun{}, fmt.Errorf("sprint-execute: runtime is required")
	}
	if strings.TrimSpace(input.Checklist) == "" {
		return ledgerRun{}, fmt.Errorf("sprint-execute: checklist is required")
	}
	return ledgerRun{
		Goal: input.Goal, Checklist: runtimePath(runtime, input.Checklist),
		ImplementModel: modelOr(input.ImplementModel, "gpt-5.6-terra"), ReviewModel: modelOr(input.ReviewModel, "gpt-5.6-sol"),
		EvaluateModel: modelOr(input.EvaluateModel, "gpt-5.6-luna"), MaxIterations: positiveOr(input.MaxIterations, defaultSprintIterations),
		ImplementPrompt: sprintImplementPrompt,
		ReviewPrompt:    sprintReviewPrompt,
	}, nil
}

func prepareChapter(runtime *program.Runtime, input ChapterLoopInput) (string, program.LoopOptions, error) {
	if runtime == nil {
		return "", program.LoopOptions{}, fmt.Errorf("chapter-loop: runtime is required")
	}
	if strings.TrimSpace(input.Checklist) == "" {
		return "", program.LoopOptions{}, fmt.Errorf("chapter-loop: checklist is required")
	}

	return runtimePath(runtime, input.Checklist), program.LoopOptions{
		Validate:      runtime.Validate,
		Evaluate:      evaluator(runtime, modelOr(input.EvaluateModel, "gpt-5.6-luna"), input.Goal, "chapter ledger"),
		MaxIterations: positiveOr(input.ChapterMaxIterations, defaultChapterIterations),
	}, nil
}

func prepareChapterSprint(runtime *program.Runtime, input ChapterLoopInput, chapter program.Iteration) (ledgerRun, error) {
	if strings.TrimSpace(chapter.Item.Checklist) == "" {
		return ledgerRun{}, fmt.Errorf("chapter %q has no nested sprint checklist", chapter.Item.Name)
	}
	return ledgerRun{
		Goal: input.Goal, Checklist: runtimePath(runtime, chapter.Item.Checklist),
		ImplementModel: modelOr(input.ImplementModel, "gpt-5.6-terra"), ReviewModel: modelOr(input.ReviewModel, "gpt-5.6-sol"),
		EvaluateModel: modelOr(input.EvaluateModel, "gpt-5.6-luna"), MaxIterations: positiveOr(input.SprintMaxIterations, defaultSprintIterations),
		ImplementPrompt: chapterImplementPrompt,
		ReviewPrompt:    chapterReviewPrompt,
		WorkContext:     chapterContext(chapter),
	}, nil
}

func prepareDelivery(runtime *program.Runtime, input DeliveryLoopInput) (ledgerRun, error) {
	if runtime == nil {
		return ledgerRun{}, fmt.Errorf("delivery-loop: runtime is required")
	}
	checklistPath := input.Checklist
	if checklistPath == "" {
		checklistPath = "plan.md"
	}
	checklistPath = runtimePath(runtime, checklistPath)
	return ledgerRun{
		Goal: input.Goal, Checklist: checklistPath, ImplementModel: modelOr(input.CodingModel, "gpt-5.6-terra"),
		ReviewModel: modelOr(input.ReviewModel, "gpt-5.6-sol"), EvaluateModel: modelOr(input.EvaluateModel, "gpt-5.6-luna"),
		MaxIterations:   positiveOr(input.MaxIterations, defaultSprintIterations),
		ImplementPrompt: deliveryImplementPrompt,
		ReviewPrompt:    deliveryReviewPrompt,
	}, nil
}

func positiveOr(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
func modelOr(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

// runtimePath resolves a ledger in the runtime workdir. Runtime owns the
// working directory so CLI callers and direct Go callers behave identically.
func runtimePath(runtime *program.Runtime, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(runtime.Workdir, path)
}
