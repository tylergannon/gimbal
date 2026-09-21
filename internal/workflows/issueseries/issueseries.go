// Package issueseries resolves an ordered list of local issue files in one
// working tree. Each issue owns a bounded promise loop: a planner selects a
// coherent assignment, a fresh coding agent implements it, and a fresh
// validator decides whether the issue is demonstrated or needs another lap.
// A passing issue is committed and pushed before the next issue starts.
//
// The issue file and research index are absolute local paths in every issue's
// context. Agents retrieve only the relevant parts of the research rather than
// receiving the entire corpus in every prompt. The workflow stops on an
// unresolved issue or execution failure and leaves the working tree intact.
//
// Example:
//
//	gimble run implement-series \
//	  --issue-list-file ./ephemeral/research/svelte-idioms/series.txt \
//	  --research-index-file ./ephemeral/research/svelte-idioms/README.md \
//	  --max-tasks-per-issue 3 \
//	  --sprint-planning gpt-5.6-sol:high \
//	  --coding gpt-5.6-terra:high \
//	  --architectural-critique claude-sonnet-5:high \
//	  --qa-orchestration claude-sonnet-5:high
package issueseries

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimble"
)

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimble/internal/generate/gimblegen -entry ImplementSeries -name implement-series -mermaid ../../../docs-site/src/lib/generated/workflows/implement-series.mmd

const roleCoding gimble.WorkflowRole = "coding"

// Params identifies an ordered issue list, its research index, and the maximum
// planner assignments allowed for each issue.
type Params struct {
	// IssueListFile is a text file containing one local issue-file path per line, in delivery order.
	IssueListFile string
	// ResearchIndexFile is the local index that routes agents to the audit, raw findings, experiments, and owner decisions.
	ResearchIndexFile string
	// MaxTasksPerIssue bounds the implementation promise loop for each issue.
	MaxTasksPerIssue int
}

// Assessment is an independent validator's bounded completion judgment.
type Assessment struct {
	// ValidationPassed is true when the issue holds at 90–95% completion with no substantial gap.
	ValidationPassed bool `json:"validation_passed"`
	// Observed states what the validator personally inspected or ran.
	Observed string `json:"observed"`
	// SubstantialGaps lists unmet requirements that justify another implementation lap.
	SubstantialGaps []string `json:"substantial_gaps"`
	// SmallGaps lists optional finishing work that must not block exit or trigger another lap.
	SmallGaps []string `json:"small_gaps"`
}

// ImplementSeries resolves local issue files in order in one working tree.
func ImplementSeries(ctx context.Context, env gimble.Env, params Params) error {
	if params.MaxTasksPerIssue < 1 {
		return fmt.Errorf("max-tasks-per-issue must be at least 1")
	}
	issueListFile, err := absoluteRegularFile(env.WorkDir, params.IssueListFile)
	if err != nil {
		return fmt.Errorf("issue-list-file: %w", err)
	}
	researchIndexFile, err := absoluteRegularFile(env.WorkDir, params.ResearchIndexFile)
	if err != nil {
		return fmt.Errorf("research-index-file: %w", err)
	}
	issueFiles, err := readIssueList(env.WorkDir, issueListFile)
	if err != nil {
		return err
	}

	gimble.Set(ctx, "issue list file", issueListFile)
	gimble.Set(ctx, "research index file", researchIndexFile)
	gimble.Set(ctx, "issue count", len(issueFiles))
	gimble.Set(ctx, "series rule", seriesRule)

	for issueCtx, issueFile := range gimble.Iterate(ctx, "issue", issueFiles) {
		gimble.Set(issueCtx, "issue file", issueFile)
		gimble.Set(issueCtx, "research index file", researchIndexFile)
		gimble.Set(issueCtx, "promise", "Resolve and directly demonstrate the issue saved in "+issueFile)
		gimble.Set(issueCtx, "maximum tasks", params.MaxTasksPerIssue)
		gimble.Set(issueCtx, "completion rule", completionRule)

		planner := gimble.NewSession(issueCtx, gimble.RoleSprintPlanning, env.WorkDir)
		plannerCoach := gimble.NewSession(issueCtx, gimble.RoleArchitecturalCritique, env.WorkDir)
		loop := gimble.PromiseLoop(issueCtx, "implementation", "Resolve and directly demonstrate the issue saved in "+issueFile, planner,
			gimble.WithSupervisor(plannerCoach, planningScopePrompt))

		completed := false
		tasksRun := 0
		for taskCtx, task := range loop.Tasks {
			tasksRun++
			worker := gimble.NewSession(taskCtx, roleCoding, env.WorkDir)
			workerCoach := gimble.NewSession(taskCtx, gimble.RoleArchitecturalCritique, env.WorkDir)
			workerReport, err := worker.Generate[gimble.Text](taskCtx, implementationPrompt,
				gimble.WithSupervisor(workerCoach, implementationScopePrompt))
			if err != nil {
				return fmt.Errorf("issue %s: %w", issueFile, err)
			}
			gimble.Set(taskCtx, "worker report", string(workerReport))

			if command := strings.TrimSpace(task.Validation.Command); command != "" {
				if err := gimble.Check(taskCtx, "task check", env.WorkDir, "sh", "-lc", command); err != nil {
					return fmt.Errorf("issue %s: %w", issueFile, err)
				}
			}

			validator := gimble.NewSession(taskCtx, gimble.RoleQAOrchestration, env.WorkDir)
			assessment, err := validator.Generate[Assessment](taskCtx, validationPrompt)
			if err != nil {
				return fmt.Errorf("issue %s: %w", issueFile, err)
			}
			gimble.SetJSON(taskCtx, "independent assessment", assessment)
			if assessment.ValidationPassed {
				completed = true
				break
			}
			if tasksRun >= params.MaxTasksPerIssue {
				return fmt.Errorf("issue %s incomplete after the maximum %d tasks", issueFile, params.MaxTasksPerIssue)
			}
		}
		if err := loop.Err(); err != nil {
			return fmt.Errorf("issue %s: %w", issueFile, err)
		}
		if !completed {
			return fmt.Errorf("issue %s incomplete: planner stopped before it was demonstrated", issueFile)
		}

		checkpointer := gimble.NewSession(issueCtx, roleCoding, env.WorkDir)
		report, err := checkpointer.Generate[gimble.Text](issueCtx, checkpointPrompt)
		if err != nil {
			return fmt.Errorf("checkpoint issue %s: %w", issueFile, err)
		}
		gimble.Set(issueCtx, "checkpoint report", string(report))
	}
	return nil
}

func absoluteRegularFile(workDir, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("path must not be blank")
	}
	if !filepath.IsAbs(name) {
		name = filepath.Join(workDir, name)
	}
	path, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("not a regular file: %s", path)
	}
	return path, nil
}

func readIssueList(workDir, listFile string) ([]string, error) {
	contents, err := os.ReadFile(listFile)
	if err != nil {
		return nil, fmt.Errorf("read issue list: %w", err)
	}

	var issues []string
	seen := map[string]bool{}
	for line := range strings.SplitSeq(string(contents), "\n") {
		name := strings.TrimSpace(line)
		if name == "" || strings.HasPrefix(name, "#") {
			continue
		}
		path, err := absoluteRegularFile(workDir, name)
		if err != nil {
			return nil, fmt.Errorf("issue %q: %w", name, err)
		}
		path = filepath.Clean(path)
		if seen[path] {
			return nil, fmt.Errorf("duplicate issue file: %s", path)
		}
		seen[path] = true
		issues = append(issues, path)
	}
	if len(issues) == 0 {
		return nil, fmt.Errorf("issue list is empty: %s", listFile)
	}
	return issues, nil
}

const seriesRule = `Work through the issue files in the supplied order in this one working tree. Finish and independently validate the current issue before starting the next. Later issues inherit earlier changes. Stop on an unresolved issue or execution failure.`

const completionRule = `Exit an issue when independent validation passes at 90–95% completion with only small gaps remaining. Record small gaps for later; replan only for substantial unmet requirements or invalid evidence.`

const planningScopePrompt = `Keep the plan inside the current issue file. Use the research index to retrieve only relevant owner decisions, catalogue entries, raw findings, experiments, or prior work. Every selected task needs a concrete definition of done that advances the issue. Object to invented features, speculative infrastructure, unrelated repairs, and work on later issues.`

const implementationScopePrompt = `Watch only for concrete work beyond the current issue and selected assignment. Object to invented features, speculative abstractions, edits to issue or research files, unrelated cleanup, and work on later issues.`

const implementationPrompt = `Read the issue file, research index, selected task, and definition of done named in context. Retrieve the relevant local research before editing. Implement only the selected task in this working tree and gather useful evidence. Do not edit the issue files, research packet, or this workflow; do not commit, push, merge, or add proof scripts and run output. Preserve unrelated work. Answer with what changed and what you personally ran or observed.`

const validationPrompt = `Independently validate the selected task and current issue. Read the issue file, research index, task definition of done, and completion rule named in context. Retrieve the relevant owner decisions and Claude findings, inspect the actual repository and recorded evidence, and personally observe behavior that checks do not establish. Make no source changes and do not treat the worker report as proof. ValidationPassed is true when the issue holds at 90–95% with no substantial gap. Put optional finishing work in SmallGaps. Set ValidationPassed false only for a substantial unmet requirement or invalid evidence and list each reason in SubstantialGaps.`

const checkpointPrompt = `The current issue has passed independent validation. Read the issue file and research index named in context, then inspect the working tree. Commit only the source, tests, generated files, and durable documentation that belong to this issue, using a concise commit message, and push the current branch. Preserve unrelated changes and local research or run artifacts. If the issue required no repository change, do not create an empty commit. Report the commit and push result.`
