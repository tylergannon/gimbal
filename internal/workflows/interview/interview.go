// Package interview demonstrates a person answering questions in the web application.
package interview

import (
	"context"

	"github.com/tylergannon/gimble"
)

//go:generate go run github.com/tylergannon/gimble/internal/generate/gimblegen -entry Interview -name interview

const interviewerRole gimble.WorkflowRole = "interviewer"

// InterviewParams are the inputs to an interview about one topic.
type InterviewParams struct {
	// Topic is the subject whose preferences the interview discovers.
	Topic string
}

// Interview discovers the person's preferences related to a topic.
func Interview(ctx context.Context, env gimble.Env, params InterviewParams) error {
	gimble.Set(ctx, "topic", params.Topic)
	interviewer := gimble.NewSession(ctx, interviewerRole, env.WorkDir)
	transcript, err := gimble.Interview(ctx, "preferences", interviewer, interviewPurpose)
	if err != nil {
		return err
	}
	gimble.SetJSON(ctx, "preferences", transcript)
	return nil
}

const interviewPurpose = "Discover the person's preferences related to the topic."
