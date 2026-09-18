package gimble

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimble/internal/runlog"
)

type interviewResult struct {
	transcript InterviewTranscript
	err        error
}

func startInterview(ctx context.Context, name string, session *Session, purpose string) <-chan interviewResult {
	result := make(chan interviewResult, 1)
	go func() {
		transcript, err := Interview(ctx, name, session, purpose)
		result <- interviewResult{transcript: transcript, err: err}
	}()
	return result
}

func pendingInterviewQuestion(ctx context.Context, r *run) (string, error) {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		r.mu.Lock()
		for id := range r.interviews {
			r.mu.Unlock()
			return id, nil
		}
		r.mu.Unlock()
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
	}
}

func receiveInterview(ctx context.Context, result <-chan interviewResult) (interviewResult, error) {
	select {
	case got := <-result:
		return got, nil
	case <-ctx.Done():
		return interviewResult{}, ctx.Err()
	}
}

func TestInterviewCollectsAnswersAndRecordsLifecycle(t *testing.T) {
	var mu sync.Mutex
	var prompts []string
	responses := []string{`{"question":"Which color?"}`, `{"question":null}`}
	f := &fake{answer: func(_ context.Context, _ string, prompt string, schema json.RawMessage, _ func(AgentEvent) error) (string, error) {
		if !strings.Contains(string(schema), `"question"`) {
			return "", errors.New("interview turn did not receive its structured schema")
		}
		mu.Lock()
		defer mu.Unlock()
		prompts = append(prompts, prompt)
		if len(prompts) > len(responses) {
			return "", errors.New("unexpected extra interview turn")
		}
		return responses[len(prompts)-1], nil
	}}

	var dir string
	err := runTest(t, bind(f, "m", "interviewer"), func(ctx context.Context) error {
		dir = runDir(ctx)
		r, _ := current(ctx)
		result := startInterview(ctx, "preferences", NewSession(ctx, "interviewer", "/w"), "Learn the person's color preference.")
		waitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		questionID, err := pendingInterviewQuestion(waitCtx, r.run)
		if err != nil {
			return err
		}
		if err := r.run.AnswerInterview(questionID, "Blue"); err != nil {
			return err
		}
		got, err := receiveInterview(waitCtx, result)
		if err != nil {
			return err
		}
		if got.err != nil {
			return got.err
		}
		want := InterviewExchange{Question: "Which color?", Answer: "Blue"}
		if len(got.transcript.Exchanges) != 1 || got.transcript.Exchanges[0] != want {
			t.Errorf("Interview transcript = %#v, want %#v", got.transcript, want)
		}
		raw, err := json.Marshal(got.transcript)
		if err != nil {
			return err
		}
		if err := got.transcript.ValidateJSON(raw); err != nil {
			t.Errorf("Interview transcript is not a valid Output: %v", err)
		}
		SetJSON(ctx, "interview", got.transcript)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	if len(prompts) != 2 || !strings.Contains(prompts[0], "Learn the person's color preference.") ||
		!strings.Contains(prompts[1], `"question": "Which color?"`) || !strings.Contains(prompts[1], `"answer": "Blue"`) {
		t.Errorf("interview prompts did not carry purpose and accumulated answers: %#v", prompts)
	}
	mu.Unlock()

	var asked *LifecycleRecord
	var answered *LifecycleRecord
	if err := runlog.Read[LifecycleRecord](t.Context(), dir, func(record LifecycleRecord) error {
		switch record.Event.(type) {
		case InterviewQuestionAsked:
			copy := record
			asked = &copy
		case InterviewQuestionAnswered:
			copy := record
			answered = &copy
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if asked == nil || answered == nil {
		t.Fatalf("interview lifecycle records: asked=%v answered=%v", asked, answered)
	}
	askedEvent := asked.Event.(InterviewQuestionAsked)
	answeredEvent := answered.Event.(InterviewQuestionAnswered)
	if asked.Scope != "" || !asked.Session.Present || asked.Session.Value != "interviewer.1" ||
		askedEvent.Name != "preferences" || askedEvent.Question != "Which color?" || askedEvent.QuestionID == "" {
		t.Errorf("asked record = %#v", asked)
	}
	if answered.Scope != asked.Scope || answered.Session != asked.Session ||
		answeredEvent.QuestionID != askedEvent.QuestionID || answeredEvent.Answer != "Blue" {
		t.Errorf("answered record = %#v, asked = %#v", answered, asked)
	}
}

func TestInterviewDoneOnFirstTurn(t *testing.T) {
	f := &fake{answer: func(_ context.Context, _ string, _ string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		return `{"question":null}`, nil
	}}
	err := runTest(t, bind(f, "m", "interviewer"), func(ctx context.Context) error {
		transcript, err := Interview(ctx, "done", NewSession(ctx, "interviewer", "/w"), "Ask only useful questions.")
		if err != nil {
			return err
		}
		if transcript.Exchanges == nil || len(transcript.Exchanges) != 0 {
			t.Errorf("Interview transcript = %#v, want an empty non-nil exchange list", transcript)
		}
		scope, _ := current(ctx)
		scope.run.mu.Lock()
		defer scope.run.mu.Unlock()
		if len(scope.run.interviews) != 0 {
			t.Errorf("done interview left %d waiters", len(scope.run.interviews))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInterviewCancellationCleansPendingQuestion(t *testing.T) {
	f := &fake{answer: func(_ context.Context, _ string, _ string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		return `{"question":"Still there?"}`, nil
	}}
	err := runTest(t, bind(f, "m", "interviewer"), func(ctx context.Context) error {
		scope, _ := current(ctx)
		interviewCtx, cancelInterview := context.WithCancel(ctx)
		defer cancelInterview()
		result := startInterview(interviewCtx, "cancel", NewSession(ctx, "interviewer", "/w"), "Ask one question.")
		waitCtx, cancelWait := context.WithTimeout(ctx, 2*time.Second)
		defer cancelWait()
		questionID, err := pendingInterviewQuestion(waitCtx, scope.run)
		if err != nil {
			return err
		}
		cancelInterview()
		got, err := receiveInterview(waitCtx, result)
		if err != nil {
			return err
		}
		if !errors.Is(got.err, context.Canceled) {
			t.Errorf("Interview cancellation = %v, want context.Canceled", got.err)
		}
		if err := scope.run.AnswerInterview(questionID, "too late"); err == nil {
			t.Error("cancelled interview accepted a stale answer")
		}
		scope.run.mu.Lock()
		defer scope.run.mu.Unlock()
		if len(scope.run.interviews) != 0 {
			t.Errorf("cancelled interview left %d waiters", len(scope.run.interviews))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInterviewFirstAnswerWinsConcurrentSubmissions(t *testing.T) {
	var turns int
	var turnMu sync.Mutex
	f := &fake{answer: func(_ context.Context, _ string, _ string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		turnMu.Lock()
		defer turnMu.Unlock()
		turns++
		if turns == 1 {
			return `{"question":"Pick one."}`, nil
		}
		return `{"question":null}`, nil
	}}
	err := runTest(t, bind(f, "m", "interviewer"), func(ctx context.Context) error {
		scope, _ := current(ctx)
		result := startInterview(ctx, "race", NewSession(ctx, "interviewer", "/w"), "Collect one answer.")
		waitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		questionID, err := pendingInterviewQuestion(waitCtx, scope.run)
		if err != nil {
			return err
		}

		const contenders = 32
		start := make(chan struct{})
		outcomes := make(chan struct {
			answer string
			err    error
		}, contenders)
		for i := range contenders {
			answer := fmt.Sprintf("answer-%d", i)
			go func() {
				<-start
				outcomes <- struct {
					answer string
					err    error
				}{answer: answer, err: scope.run.AnswerInterview(questionID, answer)}
			}()
		}
		close(start)
		accepted := ""
		for range contenders {
			outcome := <-outcomes
			if outcome.err == nil {
				if accepted != "" {
					t.Errorf("multiple concurrent answers were accepted: %q and %q", accepted, outcome.answer)
				}
				accepted = outcome.answer
			}
		}
		if accepted == "" {
			t.Error("no concurrent answer was accepted")
		}
		got, err := receiveInterview(waitCtx, result)
		if err != nil {
			return err
		}
		if got.err != nil {
			return got.err
		}
		if len(got.transcript.Exchanges) != 1 || got.transcript.Exchanges[0].Answer != accepted {
			t.Errorf("Interview kept %#v, want accepted answer %q", got.transcript, accepted)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInterviewRoutesAnswersToConcurrentQuestions(t *testing.T) {
	var mu sync.Mutex
	turns := map[string]int{}
	f := &fake{answer: func(_ context.Context, session, _ string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		turns[session]++
		if turns[session] == 1 {
			return fmt.Sprintf(`{"question":%q}`, "Question from "+session), nil
		}
		return `{"question":null}`, nil
	}}
	err := runTest(t, bind(f, "m", "interviewer"), func(ctx context.Context) error {
		scope, _ := current(ctx)
		first := startInterview(ctx, "first", NewSession(ctx, "interviewer", "/w"), "Ask once.")
		waitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		firstID, err := pendingInterviewQuestion(waitCtx, scope.run)
		if err != nil {
			return err
		}

		second := startInterview(ctx, "second", NewSession(ctx, "interviewer", "/w"), "Ask once too.")
		var secondID string
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for secondID == "" {
			scope.run.mu.Lock()
			for id := range scope.run.interviews {
				if id != firstID {
					secondID = id
				}
			}
			scope.run.mu.Unlock()
			if secondID != "" {
				break
			}
			select {
			case <-waitCtx.Done():
				return waitCtx.Err()
			case <-ticker.C:
			}
		}

		if err := scope.run.AnswerInterview(secondID, "second answer"); err != nil {
			return err
		}
		if err := scope.run.AnswerInterview(firstID, "first answer"); err != nil {
			return err
		}
		firstResult, err := receiveInterview(waitCtx, first)
		if err != nil {
			return err
		}
		secondResult, err := receiveInterview(waitCtx, second)
		if err != nil {
			return err
		}
		if firstResult.err != nil || secondResult.err != nil {
			return errors.Join(firstResult.err, secondResult.err)
		}
		if len(firstResult.transcript.Exchanges) != 1 || firstResult.transcript.Exchanges[0].Answer != "first answer" {
			t.Errorf("first interview = %#v, want its own answer", firstResult.transcript)
		}
		if len(secondResult.transcript.Exchanges) != 1 || secondResult.transcript.Exchanges[0].Answer != "second answer" {
			t.Errorf("second interview = %#v, want its own answer", secondResult.transcript)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInterviewRejectsMismatchedAndStaleQuestionIDs(t *testing.T) {
	var turn int
	f := &fake{answer: func(_ context.Context, _ string, _ string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		turn++
		if turn == 1 {
			return `{"question":"Current question?"}`, nil
		}
		return `{"question":null}`, nil
	}}
	err := runTest(t, bind(f, "m", "interviewer"), func(ctx context.Context) error {
		scope, _ := current(ctx)
		result := startInterview(ctx, "identity", NewSession(ctx, "interviewer", "/w"), "Ask one question.")
		waitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		questionID, err := pendingInterviewQuestion(waitCtx, scope.run)
		if err != nil {
			return err
		}
		if err := scope.run.AnswerInterview("not-"+questionID, "wrong question"); err == nil {
			t.Error("mismatched question id was accepted")
		}
		if err := scope.run.AnswerInterview(questionID, "right question"); err != nil {
			return err
		}
		if err := scope.run.AnswerInterview(questionID, "duplicate"); err == nil {
			t.Error("stale question id was accepted twice")
		}
		got, err := receiveInterview(waitCtx, result)
		if err != nil {
			return err
		}
		return got.err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInterviewHumanCanEndNormally(t *testing.T) {
	var turns int
	f := &fake{answer: func(_ context.Context, _ string, _ string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		turns++
		return `{"question":"Anything else?"}`, nil
	}}
	err := runTest(t, bind(f, "m", "interviewer"), func(ctx context.Context) error {
		scope, _ := current(ctx)
		result := startInterview(ctx, "human-end", NewSession(ctx, "interviewer", "/w"), "Ask until the person is done.")
		waitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		questionID, err := pendingInterviewQuestion(waitCtx, scope.run)
		if err != nil {
			return err
		}
		if err := scope.run.AnswerInterview(questionID, "   "); err != nil {
			return err
		}
		got, err := receiveInterview(waitCtx, result)
		if err != nil {
			return err
		}
		if got.err != nil {
			return got.err
		}
		if len(got.transcript.Exchanges) != 0 {
			t.Errorf("human-ended Interview transcript = %#v, want no unanswered exchange", got.transcript)
		}
		if turns != 1 {
			t.Errorf("human-ended Interview ran %d model turns, want 1", turns)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
