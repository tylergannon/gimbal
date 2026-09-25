package routes

import (
	"context"
	"net/http"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimbal/internal/live"
)

// InterviewAnswer is what one pending question's form posts. The question ID
// is opaque and run-local; together with the run it names exactly one waiter.
type InterviewAnswer struct {
	Run        string `json:"run"`
	QuestionID string `json:"question_id"`
	Answer     string `json:"answer"`
}

// InterviewAnswered confirms that the waiting interview accepted the answer.
type InterviewAnswered struct {
	Accepted bool `json:"accepted"`
}

// answerInterview carries one person's answer from the run page to the live
// interview. An empty answer is valid: it tells the interview to end normally.
func answerInterview(ctx context.Context, arg InterviewAnswer) (InterviewAnswered, error) {
	runs := live.RunsFrom(ctx)
	if runs == nil {
		return InterviewAnswered{}, skgo.Errorf(http.StatusInternalServerError,
			"This server has no table of runs in its context, so no interview can be answered.")
	}
	run, err := runs.InProgress(arg.Run)
	if err != nil {
		return InterviewAnswered{}, skgo.Errorf(http.StatusNotFound,
			"Run %s is not in progress, so its interview cannot be answered.", arg.Run)
	}
	if err := run.AnswerInterview(arg.QuestionID, arg.Answer); err != nil {
		return InterviewAnswered{}, skgo.Errorf(http.StatusNotFound,
			"This interview question is no longer waiting for an answer.")
	}
	return InterviewAnswered{Accepted: true}, nil
}

var _ = skgo.Form(answerInterview)
