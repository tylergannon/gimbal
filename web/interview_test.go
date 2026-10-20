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

	"github.com/tylergannon/gimbal/internal/host"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/app"
	"github.com/tylergannon/gimbal/internal/observation"
	routes "github.com/tylergannon/gimbal/internal/skgo/links/onzggl3sn52xizlt"
	"github.com/tylergannon/gimbal/internal/skgo/params"
)

type interactiveing struct {
	mu    sync.Mutex
	turns int
}

func (*interactiveing) CreateSession(context.Context, string, string, string) (string, error) {
	return "native-interactiveer", nil
}

func (i *interactiveing) RunTurn(context.Context, string, string, json.RawMessage, func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.turns++
	var answer string
	switch i.turns {
	case 1:
		answer = `{"type":"chat","message":"Which color?"}`
	case 2:
		answer = `{"type":"chat","message":"Anything else?"}`
	default:
		answer = `{"type":"end"}`
	}
	return gimbal.TurnResult{Output: json.RawMessage(answer)}, nil
}

func (*interactiveing) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*interactiveing) Fork(context.Context, string) (string, error)        { return "fork", nil }
func (*interactiveing) Close(context.Context, string) error                 { return nil }

func pendingQuestion(t *testing.T, runtime *host.Project, runID, except string) observation.InteractiveRow {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		snapshot, err := runtime.Registry().Snapshot(runID)
		if err == nil {
			for id, question := range snapshot.Interactions {
				if id != except && question.Status == observation.InteractiveStatusPending {
					return question
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no pending interactive question after %q: %v", except, err)
		}
		time.Sleep(time.Millisecond)
	}
}

func startedRunID(t *testing.T, project string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		entries, err := os.ReadDir(filepath.Join(project, ".gimbal", "runs"))
		if err == nil && len(entries) == 1 {
			return entries[0].Name()
		}
		if time.Now().After(deadline) {
			t.Fatalf("run did not start: entries = %v, error = %v", entries, err)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestInteractiveAnswerFormReachesTheWaitingInteractive(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	project := t.TempDir()
	_, runtime, err := newProject(ctx, project, WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	adapter := &interactiveing{}
	transcript := make(chan gimbal.InteractiveTranscript, 1)
	var runErr error
	var runWG sync.WaitGroup
	runWG.Go(func() {
		runErr = runtime.Run(ctx, "interactive", map[gimbal.WorkflowRole]gimbal.ModelBinding{
			"interactiveer": {Adapter: adapter, Model: "m"},
		}, func(ctx context.Context) error {
			got, err := gimbal.Interactive(ctx, "preferences", gimbal.NewSession(ctx, "interactiveer", "/w"), "Learn the person's preferences.")
			transcript <- got
			return err
		})
	})

	id := startedRunID(t, project)
	first := pendingQuestion(t, runtime, id, "")
	answered, err := routes.Skgo_answerInteractive(runtime.Context(), app.RequestEvent[params.Params]{}, routes.InteractiveAnswer{
		RequestID: "message-one", Run: id, QuestionID: first.Address, Answer: "Blue",
	})
	if err != nil || !answered.Accepted {
		t.Fatalf("first answer = %+v, %v; want accepted", answered, err)
	}

	var status *skgo.HTTPError
	if _, err := routes.Skgo_answerInteractive(runtime.Context(), app.RequestEvent[params.Params]{}, routes.InteractiveAnswer{
		RequestID: "message-one", Run: id, QuestionID: first.Address, Answer: "duplicate",
	}); !errors.As(err, &status) || status.Status != 404 {
		t.Fatalf("stale answer = %v, want 404", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		snapshot, e := runtime.Registry().Snapshot(id)
		if e == nil {
			ready := false
			for _, row := range snapshot.Interactions {
				ready = ready || len(row.Messages) == 2 && row.Status == "pending"
			}
			if ready {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("chat did not establish wait")
		}
		time.Sleep(time.Millisecond)
	}
	answered, err = routes.Skgo_finishInteractive(runtime.Context(), app.RequestEvent[params.Params]{}, routes.InteractiveFinish{
		ID: "finish", Run: id, Address: first.Address,
	})
	if err != nil || !answered.Accepted {
		t.Fatalf("finish = %+v, %v; want accepted", answered, err)
	}
	runWG.Wait()
	if runErr != nil {
		t.Fatal(runErr)
	}
	got := <-transcript
	if len(got.Messages) != 2 || got.Messages[0].Message != "Blue" || got.EndedBy != "human" {
		t.Fatalf("transcript = %#v, want the first answer and normal human finish", got)
	}
}
