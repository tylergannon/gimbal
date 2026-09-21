package main

import (
	"context"
	_ "embed"
	"encoding/json"

	"github.com/spf13/cobra"
	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/binding"
	"github.com/tylergannon/gimble/internal/conversation"
	"github.com/tylergannon/gimble/internal/workflows/implementation"
	"github.com/tylergannon/gimble/internal/workflows/issueseries"
	"github.com/tylergannon/gimble/internal/workflows/pyramidsummary"
	"github.com/tylergannon/gimble/internal/workflows/researchdocument"
	"github.com/tylergannon/gimble/internal/workflows/review"
	"github.com/tylergannon/gimble/internal/workflows/validateproduct"
	"github.com/tylergannon/gimble/web"
)

//go:embed defaults.json
var workflowDefaultsJSON []byte

func workflowDefaults() map[gimble.WorkflowRole]string {
	var defaults map[gimble.WorkflowRole]string
	if err := json.Unmarshal(workflowDefaultsJSON, &defaults); err != nil {
		panic(err)
	}
	return defaults
}

func conversationWorkflowOption() web.Option {
	defaults := workflowDefaults()
	reviewEntry := func(ctx context.Context, runtime *web.Runtime, worktree string, request conversation.LaunchRequest) error {
		models, err := binding.Roles(map[gimble.WorkflowRole]string{
			gimble.RoleCodeReview: defaults[gimble.RoleCodeReview],
		})
		if err != nil {
			return err
		}
		env := gimble.Env{WorkDir: worktree}
		params := review.ReviewParams{Goal: request.Goal}
		return runtime.Run(ctx, conversation.WorkflowReview, models, func(ctx context.Context) error {
			return review.Review(ctx, env, params)
		})
	}
	implementEntry := func(ctx context.Context, runtime *web.Runtime, worktree string, request conversation.LaunchRequest) error {
		models, err := binding.Roles(map[gimble.WorkflowRole]string{
			gimble.RoleSprintPlanning:        defaults[gimble.RoleSprintPlanning],
			gimble.RoleArchitecturalCritique: defaults[gimble.RoleArchitecturalCritique],
			gimble.WorkflowRole("coding"):    defaults[gimble.WorkflowRole("coding")],
			gimble.RoleQAOrchestration:       defaults[gimble.RoleQAOrchestration],
		})
		if err != nil {
			return err
		}
		env := gimble.Env{WorkDir: worktree}
		params := implementation.Params{
			Promise: request.Goal, DefinitionOfDoneFile: request.DefinitionOfDoneFile, MaxTasks: 3,
		}
		return runtime.Run(ctx, conversation.WorkflowImplement, models, func(ctx context.Context) error {
			return implementation.Implement(ctx, env, params)
		})
	}
	return web.WithConversationWorkflows(reviewEntry, implementEntry)
}

// newRunCommand is gimble run: the workflows built into this binary, each
// the Command its package generated.
func newRunCommand() *cobra.Command {
	run := &cobra.Command{
		Use:   "run",
		Short: "Run a workflow built into this binary; gimble run --help lists them",
	}
	run.AddCommand(review.Command(workflowDefaults()))
	run.AddCommand(validateproduct.Command(workflowDefaults()))
	run.AddCommand(implementation.Command(workflowDefaults()))
	run.AddCommand(issueseries.Command(workflowDefaults()))
	run.AddCommand(researchdocument.Command(workflowDefaults()))
	run.AddCommand(pyramidsummary.Command(workflowDefaults()))
	return run
}
