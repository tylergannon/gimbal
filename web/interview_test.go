package web

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimble/internal/host"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/observation"
	routes "github.com/tylergannon/gimble/internal/skgo/links/onzggl3sn52xizlt"
)

type interviewing struct {
	mu    sync.Mutex
	turns int
}

func (*interviewing) CreateSession(context.Context, string, string, string) (string, error) {
	return "native-interviewer", nil
}

func (i *interviewing) RunTurn(context.Context, string, string, json.RawMessage, func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.turns++
	var answer string
	switch i.turns {
	case 1:
		answer = `{"question":"Which color?"}`
	case 2:
		answer = `{"question":"Anything else?"}`
	default:
		answer = `{"question":null}`
	}
	return gimble.TurnResult{Output: json.RawMessage(answer)}, nil
}

func (*interviewing) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*interviewing) Fork(context.Context, string) (string, error)        { return "fork", nil }
func (*interviewing) Close(context.Context, string) error                 { return nil }

func pendingQuestion(t *testing.T, runtime *host.Project, runID, except string) observation.InterviewRow {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		snapshot, err := runtime.Registry().Snapshot(runID)
		if err == nil {
			for id, question := range snapshot.Interviews {
				if id != except && question.Status == observation.InterviewStatusPending {
					return question
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no pending interview question after %q: %v", except, err)
		}
		time.Sleep(time.Millisecond)
	}
}

func startedRunID(t *testing.T, project string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		entries, err := os.ReadDir(filepath.Join(project, ".gimble", "runs"))
		if err == nil && len(entries) == 1 {
			return entries[0].Name()
		}
		if time.Now().After(deadline) {
			t.Fatalf("run did not start: entries = %v, error = %v", entries, err)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestInterviewAnswerFormReachesTheWaitingInterview(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	project := t.TempDir()
	_, runtime, err := newProject(ctx, project, WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	adapter := &interviewing{}
	transcript := make(chan gimble.InterviewTranscript, 1)
	var runErr error
	var runWG sync.WaitGroup
	runWG.Go(func() {
		runErr = runtime.Run(ctx, "interview", map[gimble.WorkflowRole]gimble.ModelBinding{
			"interviewer": {Adapter: adapter, Model: "m"},
		}, func(ctx context.Context) error {
			got, err := gimble.Interview(ctx, "preferences", gimble.NewSession(ctx, "interviewer", "/w"), "Learn the person's preferences.")
			transcript <- got
			return err
		})
	})

	id := startedRunID(t, project)
	first := pendingQuestion(t, runtime, id, "")
	answered, err := routes.Skgo_answerInterview(runtime.Context(), routes.InterviewAnswer{
		Run: id, QuestionID: first.QuestionID, Answer: "Blue",
	})
	if err != nil || !answered.Accepted {
		t.Fatalf("first answer = %+v, %v; want accepted", answered, err)
	}

	var status *skgo.HTTPError
	if _, err := routes.Skgo_answerInterview(runtime.Context(), routes.InterviewAnswer{
		Run: id, QuestionID: first.QuestionID, Answer: "duplicate",
	}); !errors.As(err, &status) || status.Status != 404 {
		t.Fatalf("stale answer = %v, want 404", err)
	}

	second := pendingQuestion(t, runtime, id, first.QuestionID)
	answered, err = routes.Skgo_answerInterview(runtime.Context(), routes.InterviewAnswer{
		Run: id, QuestionID: second.QuestionID, Answer: "",
	})
	if err != nil || !answered.Accepted {
		t.Fatalf("empty answer = %+v, %v; want accepted", answered, err)
	}
	runWG.Wait()
	if runErr != nil {
		t.Fatal(runErr)
	}
	got := <-transcript
	if len(got.Exchanges) != 1 || got.Exchanges[0].Answer != "Blue" {
		t.Fatalf("transcript = %#v, want the first answer and normal empty-answer end", got)
	}
}
