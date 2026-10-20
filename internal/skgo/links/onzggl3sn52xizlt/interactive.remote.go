package routes

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/tylergannon/gimbal/internal/app"
	"github.com/tylergannon/gimbal/internal/skgo/params"
	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimbal/internal/live"
)

// InteractiveAnswer is what one pending question's form posts. The question ID
// is opaque and run-local; together with the run it names exactly one waiter.
type InteractiveAnswer struct {
	RequestID  string `json:"request_id"`
	Run        string `json:"run"`
	QuestionID string `json:"question_id"`
	Answer     string `json:"answer"`
}

// InteractiveAnswered confirms that the waiting interactive accepted the answer.
type InteractiveAnswered struct {
	Accepted bool `json:"accepted"`
}

// answerInteractive carries one person's answer from the run page to the live
// interactive. An empty answer is valid: it tells the interactive to end normally.
func answerInteractive(ctx context.Context, event app.RequestEvent[params.Params], arg InteractiveAnswer) (InteractiveAnswered, error) {
	runs := live.RunsFrom(ctx)
	if runs == nil {
		return InteractiveAnswered{}, skgo.Errorf(http.StatusInternalServerError,
			"This server has no table of runs in its context, so no interactive can be answered.")
	}
	run, err := runs.InProgress(arg.Run)
	if err != nil {
		return InteractiveAnswered{}, skgo.Errorf(http.StatusNotFound,
			"Run %s is not in progress, so its interactive cannot be answered.", arg.Run)
	}
	if arg.RequestID == "" {
		return InteractiveAnswered{}, skgo.Invalidf("request_id", "Input identity required")
	}
	raw, _ := json.Marshal(map[string]any{"id": arg.RequestID, "message": arg.Answer, "finish": arg.Answer == ""})
	if err := run.SubmitInteractive(arg.QuestionID, string(raw)); err != nil {
		return InteractiveAnswered{}, skgo.Errorf(http.StatusNotFound,
			"This interactive question is no longer waiting for an answer.")
	}
	return InteractiveAnswered{Accepted: true}, nil
}

var _ = skgo.Form(answerInteractive)

// InteractiveFinish is a durable request for normal interaction completion.
type InteractiveFinish struct {
	Run     string `json:"run"`
	Address string `json:"address"`
	ID      string `json:"id"`
}

func finishInteractive(ctx context.Context, event app.RequestEvent[params.Params], arg InteractiveFinish) (InteractiveAnswered, error) {
	runs := live.RunsFrom(ctx)
	if runs == nil {
		return InteractiveAnswered{}, skgo.Errorf(500, "Run controls unavailable")
	}
	run, err := runs.InProgress(arg.Run)
	if err != nil {
		return InteractiveAnswered{}, skgo.Errorf(404, "%s", err)
	}
	if arg.ID == "" {
		return InteractiveAnswered{}, skgo.Errorf(400, "Input identity required")
	}
	raw, _ := json.Marshal(map[string]any{"id": arg.ID, "finish": true})
	if err := run.SubmitInteractive(arg.Address, string(raw)); err != nil {
		return InteractiveAnswered{}, skgo.Errorf(400, "%s", err)
	}
	return InteractiveAnswered{Accepted: true}, nil
}

var _ = skgo.Command(finishInteractive)
