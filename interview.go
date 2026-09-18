package gimble

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/tylergannon/polytype"
)

// InterviewTranscript is the question-and-answer history returned by
// Interview. It is a generated Output, so a workflow can retain it with
// SetJSON.
type InterviewTranscript struct {
	// Exchanges are the questions the interviewer asked and the answers the
	// person gave, in order.
	Exchanges []InterviewExchange `json:"exchanges"`
}

// InterviewExchange is one answered question in an InterviewTranscript.
type InterviewExchange struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// interviewDecision is one turn's decision: ask Question, or use null when
// the purpose is fulfilled and the interview is done.
type interviewDecision struct {
	// Question is the single next useful question. Use null when no more
	// questions are needed.
	Question polytype.Nullable[string] `json:"question"`
}

// Interview asks questions on session until the interviewer is done or the
// person ends the interview. Each question is produced as a normal typed
// session turn, then Interview blocks until the live run receives its answer.
// An empty answer ends the interview normally without adding an exchange.
func Interview(ctx context.Context, name string, session *Session, purpose string) (InterviewTranscript, error) {
	transcript := InterviewTranscript{Exchanges: []InterviewExchange{}}
	if session == nil {
		return transcript, errors.New("gimble: Interview requires a session")
	}
	scope, err := current(ctx)
	if err != nil {
		return transcript, err
	}

	for {
		decision, err := dispatch[interviewDecision](ctx, session, interviewPrompt(purpose, transcript, scopeText(ctx)), nil)
		if err != nil {
			return transcript, err
		}
		if err := ctx.Err(); err != nil {
			return transcript, err
		}
		if !decision.Question.Present || strings.TrimSpace(decision.Question.Value) == "" {
			return transcript, nil
		}

		question := decision.Question.Value
		questionID, waiter := scope.run.registerInterviewQuestion()
		scope.run.event(scope.key, session.id, "", InterviewQuestionAsked{
			Name: name, QuestionID: questionID, Question: question,
		})

		answer, err := scope.run.waitInterviewAnswer(ctx, questionID, waiter)
		if err != nil {
			return transcript, err
		}
		if strings.TrimSpace(answer) == "" {
			scope.run.event(scope.key, session.id, "", InterviewQuestionAnswered{QuestionID: questionID, Answer: answer})
			return transcript, nil
		}
		transcript.Exchanges = append(transcript.Exchanges, InterviewExchange{Question: question, Answer: answer})
		scope.run.event(scope.key, session.id, "", InterviewQuestionAnswered{QuestionID: questionID, Answer: answer})
	}
}

func interviewPrompt(purpose string, transcript InterviewTranscript, scopeContext string) string {
	history, err := json.MarshalIndent(transcript, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("gimble: encode interview transcript: %v", err))
	}
	prompt := fmt.Sprintf(`Conduct the interview for this purpose:

%s

Here is the interview so far:

%s

Decide the single next useful question. Set question to null when the purpose is fulfilled and no more questions are needed.`, purpose, history)
	if scopeContext != "" {
		prompt += "\n\nThe workflow's current context is:\n\n" + scopeContext
	}
	return prompt
}
