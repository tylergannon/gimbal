// Package conversation hosts an ongoing human conversation as an ordinary workflow.
package conversation

import (
	"context"
	"github.com/tylergannon/gimbal"
)

//go:generate go run github.com/tylergannon/gimbal/internal/generate/gimbalgen -entry Conversation -name conversation

type Params struct {
	// Purpose describes what the conversation should help the human accomplish.
	Purpose string `json:"purpose"`
	// Environment is the selected logical execution environment.
	Environment string `json:"environment"`
}

// Conversation remains available until the human finishes it.
func Conversation(ctx context.Context, env gimbal.Env, params Params) error {
	ctx = gimbal.WithEnvironment(ctx, params.Environment)
	gimbal.Set(ctx, "purpose", params.Purpose)
	agent := gimbal.NewSession(ctx, gimbal.RoleAgenticDialogues, env.WorkDir)
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
