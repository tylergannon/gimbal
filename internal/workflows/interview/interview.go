// Package interview demonstrates a person answering questions in the web application.
package interview

import (
	"context"

	"github.com/tylergannon/gimbal"
)

//go:generate go run github.com/tylergannon/gimbal/internal/generate/gimbalgen -entry Interview -name interview

const interviewerRole gimbal.WorkflowRole = "interviewer"

// InterviewParams are the inputs to an interview about one topic.
type InterviewParams struct {
	// Topic is the subject whose preferences the interview discovers.
	Topic string
}

// Interview discovers the person's preferences related to a topic.
func Interview(ctx context.Context, env gimbal.Env, params InterviewParams) error {
	gimbal.Set(ctx, "topic", params.Topic)
	interviewer := gimbal.NewSession(ctx, interviewerRole, env.WorkDir)
	transcript, err := gimbal.Interview(ctx, "preferences", interviewer, interviewPurpose)
	if err != nil {
		return err
	}
	gimbal.SetJSON(ctx, "preferences", transcript)
	return nil
}

const interviewPurpose = "Discover the person's preferences related to the topic."
