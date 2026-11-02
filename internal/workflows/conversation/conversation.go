// Package conversation hosts an ongoing human conversation as an ordinary workflow.
package conversation

import (
	"context"
	"fmt"
	"strings"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/polytype"
)

//go:generate go run github.com/tylergannon/gimbal/internal/generate/gimbalgen -entry Conversation -name conversation

type Params struct {
	// Purpose describes what the conversation should help the human accomplish.
	Purpose string `json:"purpose"`
	// Environment is the selected logical execution environment.
	Environment string `json:"environment"`
	// PrepareCheckout creates a separate checkout on the selected worker.
	PrepareCheckout polytype.Optional[bool] `json:"prepare_checkout"`
	// CheckoutID identifies the checkout across workflow recovery.
	CheckoutID polytype.Optional[string] `json:"checkout_id"`
	// Branch and Base select the new branch and its starting commit.
	Branch polytype.Optional[string] `json:"branch"`
	Base   polytype.Optional[string] `json:"base"`
}

// Conversation remains available until the human finishes it.
func Conversation(ctx context.Context, env gimbal.Env, params Params) error {
	ctx = gimbal.WithEnvironment(ctx, params.Environment)
	gimbal.Set(ctx, "purpose", params.Purpose)
	directory := env.WorkDir
	if params.PrepareCheckout.Value {
		if params.CheckoutID.Value == "" {
			return fmt.Errorf("conversation: checkout identity is required")
		}
		code, stdout, stderr, err := gimbal.RunCommand(ctx, "prepare-checkout", env.WorkDir, "sh", "-c", `set -eu
id=$1; branch=$2; base=$3
case "$id" in *[!a-zA-Z0-9_-]*|'') echo 'invalid checkout identity' >&2; exit 1;; esac
if ! root=$(git rev-parse --show-toplevel 2>/dev/null) || ! git rev-parse --verify 'HEAD^{commit}' >/dev/null 2>&1; then
  if [ -n "$branch$base" ]; then echo 'directory has no committed Git history' >&2; exit 1; fi
  pwd; exit 0
fi
if [ -z "$base" ]; then base=$(git symbolic-ref --quiet --short refs/remotes/origin/HEAD 2>/dev/null || printf HEAD); fi
commit=$(git rev-parse --verify --end-of-options "$base^{commit}")
directory=${TMPDIR:-/tmp}/gimbal-conversations/$id
mkdir -p "$(dirname "$directory")"
if [ -z "$branch" ]; then branch="gimbal/conversation-$(printf '%.12s' "$id")"; fi
git check-ref-format "refs/heads/$branch"
git -C "$root" worktree add -b "$branch" "$directory" "$commit" >&2
printf '%s\n%s\n' "$directory" "$branch"`, "prepare-checkout", params.CheckoutID.Value, params.Branch.Value, params.Base.Value)
		if err != nil {
			return err
		}
		if code != 0 {
			return fmt.Errorf("conversation: checkout preparation failed: %s", strings.TrimSpace(stderr))
		}
		path, branch, _ := strings.Cut(stdout, "\n")
		directory = strings.TrimSpace(path)
		if directory == "" {
			return fmt.Errorf("conversation: checkout preparation returned no directory")
		}
		if branch = strings.TrimSpace(branch); branch != "" {
			gimbal.Set(ctx, "conversation_checkout_branch", branch)
		}
	}
	agent := gimbal.NewSession(ctx, gimbal.RoleAgenticDialogues, directory)
	for {
		transcript, err := gimbal.Interactive(ctx, "conversation", agent, conversationPurpose)
		if err != nil {
			return err
		}
		if transcript.EndedBy == "human" {
			return nil
		}
	}
}

const conversationPurpose = "Help the human with the stated purpose. Reply with chat when there is more to discuss, or end when this part of the conversation is complete."
