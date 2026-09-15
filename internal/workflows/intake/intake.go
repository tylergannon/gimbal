// Package intake helps a person choose how much planning their work needs.
package intake

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/codex"
)

//go:generate go tool polytype --validate

type Input struct {
	Repo, Goal, Plan, Acceptance, Constraints, Model string
	ContextFiles                                     []string
}

// Advice preserves the user's clarifications separately from their original goal.
type Advice struct {
	Workflow, Reason, Clarifications string
}

type decision struct {
	// Choose lfg for a small clear change, plan for substantial or uncertain work,
	// or sprint for a sufficiently detailed existing plan. Use exactly one of these names.
	Workflow string `json:"workflow"`
	// Briefly explain how the size and uncertainty of this particular task justify the choice.
	Reason string `json:"reason"`
	// Ask one short question only when its answer materially changes the goal or choice.
	// Otherwise return an empty string. Never require a planning ritual for a small clear task.
	Question string `json:"question"`
}

// Guide runs an advisory conversation inside the caller's run, without editing the project.
// A nil reader means the caller requested a recommendation without an interview.
func Guide(ctx context.Context, in Input, reader *bufio.Reader, out io.Writer) (Advice, error) {
	return guide(ctx, in, reader, out, codex.New())
}

func guide(ctx context.Context, in Input, reader *bufio.Reader, out io.Writer, adapter gimble.HarnessAdapter) (Advice, error) {
	if strings.TrimSpace(in.Goal) == "" && in.Plan == "" {
		return Advice{}, errors.New("describe the work or supply a plan")
	}
	if in.Model == "" {
		in.Model = "gpt-5.6-luna"
	}
	gimble.Set(ctx, "original request", in.Goal)
	gimble.Set(ctx, "plan file", in.Plan)
	gimble.Set(ctx, "acceptance criteria", in.Acceptance)
	gimble.Set(ctx, "constraints", in.Constraints)
	gimble.Set(ctx, "context files", in.ContextFiles)
	guide := gimble.NewSession(ctx, "guide", adapter, in.Model, in.Repo)
	var advice Advice
	for visit := range 4 {
		var next decision
		err := gimble.Scope(ctx, "clarify", func(ctx context.Context) error {
			gimble.Set(ctx, "user clarifications", advice.Clarifications)
			prompt := "Read the request and relevant local files in " + in.Repo + ". Change no files and run no mutating commands. Recommend the smallest useful workflow: lfg is one supervised implementation session; plan drafts and critiques a plan before sprint execution; sprint executes an existing sufficiently clear plan with a planner, supervised workers, and independent validation. A design or a claim is a valid starting point. Preserve the person's intent. Ask only if an answer would materially change the work, not to fill a template.\n\n" + gimble.ScopeText(ctx)
			if reader == nil || visit == 3 {
				prompt += "\nDo not ask another question. Recommend based on the supplied information, stating material assumptions in your reason. Prefer planning when important design decisions remain."
			}
			var err error
			next, err = guide.Generate[decision](ctx, prompt)
			if err != nil {
				return err
			}
			gimble.SetJSON(ctx, "recommendation", next)
			return nil
		})
		if err != nil {
			return Advice{}, err
		}
		switch next.Workflow {
		case "lfg", "plan", "sprint":
		default:
			return Advice{}, fmt.Errorf("guide returned unknown workflow %q", next.Workflow)
		}
		if strings.TrimSpace(next.Question) == "" {
			advice.Workflow, advice.Reason = next.Workflow, next.Reason
			return advice, nil
		}
		if reader == nil || visit == 3 {
			return Advice{}, fmt.Errorf("guide still needs an answer: %s", next.Question)
		}
		if _, err := fmt.Fprintln(out, next.Question); err != nil {
			return Advice{}, err
		}
		answer, err := readAnswer(ctx, reader)
		if err != nil {
			return Advice{}, fmt.Errorf("answer needed before choosing a workflow: %w", err)
		}
		advice.Clarifications += "\nQuestion: " + next.Question + "\nAnswer: " + answer + "\n"
	}
	return Advice{}, errors.New("guide did not settle on a workflow")
}

func readAnswer(ctx context.Context, reader *bufio.Reader) (string, error) {
	// Reading is synchronous. The caller owns closing a blocked reader when it
	// needs cancellation to unblock it; this helper does not leak a goroutine.
	if err := ctx.Err(); err != nil {
		return "", err
	}
	text, err := reader.ReadString('\n')
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", ctxErr
	}
	if err != nil && (!errors.Is(err, io.EOF) || text == "") {
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		return "", errors.New("empty answer")
	}
	return strings.TrimRight(text, "\r\n"), nil
}
