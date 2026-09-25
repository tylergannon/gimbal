package workflows

import (
	"context"

	"github.com/tylergannon/gimbal/checklist"
	"github.com/tylergannon/gimbal/program"
)

type turnResult struct {
	Notes string `json:"notes"`
}
type reviewResult struct {
	MaterialDefect bool   `json:"material_defect"`
	Notes          string `json:"notes"`
}
type evaluateResult struct {
	Passed bool   `json:"passed"`
	Notes  string `json:"notes"`
}

func turn(ctx context.Context, runtime *program.Runtime, name, model, prompt string) error {
	_, err := program.Codergen[turnResult](ctx, runtime, program.CodergenRequest{Name: name, Prompt: prompt, Model: model, JSONSchema: objectSchema("Report the work completed.", []string{"notes"}, map[string]any{"notes": stringSchema("Brief result.")})})
	return err
}

func review(ctx context.Context, runtime *program.Runtime, run ledgerRun, iteration program.Iteration) (reviewResult, error) {
	prompt := reviewPrompt(run.ReviewPrompt, run.WorkContext)
	return program.Codergen[reviewResult](ctx, runtime, program.CodergenRequest{Name: "review " + iteration.Item.Name, Model: run.ReviewModel, Prompt: itemPrompt(run.Goal, iteration, prompt), JSONSchema: objectSchema("Review routing decision.", []string{"material_defect", "notes"}, map[string]any{"material_defect": map[string]any{"type": "boolean"}, "notes": stringSchema("Specific defect, or why validation should decide.")})})
}

func evaluator(runtime *program.Runtime, model, goal, subject string) func(context.Context, *checklist.Checklist) (program.Validation, error) {
	return func(ctx context.Context, list *checklist.Checklist) (program.Validation, error) {
		result, err := program.Codergen[evaluateResult](ctx, runtime, program.CodergenRequest{Name: "evaluate " + subject, Model: model, Prompt: evaluationPrompt(subject, goal, list.Body), JSONSchema: objectSchema("Checklist completion decision.", []string{"passed", "notes"}, map[string]any{"passed": map[string]any{"type": "boolean"}, "notes": stringSchema("Reason for the decision.")})})
		return program.Validation{Passed: result.Passed, Notes: result.Notes}, err
	}
}

func implement(ctx context.Context, runtime *program.Runtime, run ledgerRun, iteration program.Iteration) error {
	return turn(ctx, runtime, "implement "+iteration.Item.Name, run.ImplementModel, implementationPrompt(run, iteration))
}

func repair(ctx context.Context, runtime *program.Runtime, run ledgerRun, iteration program.Iteration, notes string) error {
	return turn(ctx, runtime, "repair "+iteration.Item.Name, run.ImplementModel, repairPrompt(run, iteration, notes))
}
