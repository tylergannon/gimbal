package continuity

import (
	"context"
	"fmt"

	"github.com/tylergannon/gimbal"
)

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimbal/internal/generate/gimbalgen -entry Continuity -name continuity

const RememberPrompt = "Remember amber-17 as our conversation token. Return exactly this object: {\"summary\":\"edit\",\"file\":\"continuity.txt\",\"receipt\":\"amber-17\"}. Do not edit files yet."
const EditPrompt = "Recall our conversation token as receipt. Write that token to continuity.txt. Return summary as the single word in the current layer context value, without explanation, and file continuity.txt."
const ForkPrompt = "Recall the conversation token we had before this turn as receipt. Then remember violet-29 as the new token in this conversation only. Do not edit any files. Return summary violet-29 and file continuity.txt, without extra explanation."
const ResumePrompt = "Recall our conversation token as receipt. Read continuity.txt and report its filename. Return summary as the single word in the current layer context value, without extra explanation."

// Continuity keeps a parent conversation while scoped work edits its checkout.
func Continuity(ctx context.Context, env gimbal.Env) error {
	gimbal.Set(ctx, "layer", "parent")
	parent := gimbal.NewSession(ctx, "coder", env.WorkDir)
	decision, err := parent.Generate[Report](ctx, RememberPrompt)
	if err != nil {
		return err
	}
	if decision.Summary == "edit" {
		err = gimbal.Scope(ctx, "child", func(ctx context.Context) error {
			gimbal.Set(ctx, "layer", "child")
			child, err := parent.Generate[Report](ctx, EditPrompt)
			if err != nil {
				return err
			}
			gimbal.SetJSON(ctx, "result", child)
			fork, err := parent.Fork(ctx, "fork")
			if err != nil {
				return err
			}
			_, err = fork.Generate[Report](ctx, ForkPrompt)
			return err
		})
		if err != nil {
			return err
		}
	}
	code, stdout, stderr, err := gimbal.RunCommand(ctx, "diagnostic", env.WorkDir, "sh", "-c", "printf observed; printf diagnostic >&2; exit 7")
	if err != nil {
		return err
	}
	gimbal.SetJSON(ctx, "diagnostic", Checks{code, stdout, stderr})
	if code == 7 {
		if _, _, _, err = gimbal.RunCommand(ctx, "recovery", env.WorkDir, "sh", "-c", "printf recovered > recovery.txt"); err != nil {
			return err
		}
	}
	final, err := parent.Generate[Report](ctx, ResumePrompt)
	if err != nil {
		return err
	}
	if final.Receipt != "amber-17" || final.Summary != "parent" {
		return fmt.Errorf("continuation mismatch: %+v", final)
	}
	return nil
}

type Report struct {
	Summary string `json:"summary"`
	File    string `json:"file"`
	Receipt string `json:"receipt"`
}
type Checks struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}
