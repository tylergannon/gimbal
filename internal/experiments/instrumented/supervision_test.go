package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/claude"
	"github.com/tylergannon/gimbal/pi"
)

// This is an opt-in paid integration test, run in the prepared activity image.
// Only the Jev HTTP response is controlled. Principal events, supervision
// routing, the review prompt, the Pi process and its response are all real.
func TestLiveControlledSupervision(t *testing.T) {
	if os.Getenv("SPECIMEN_LIVE_TEST") != "1" {
		t.Skip("set SPECIMEN_LIVE_TEST=1 inside the prepared image")
	}
	var mu sync.Mutex
	var packets []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			State     json.RawMessage            `json:"state"`
			Questions map[string]json.RawMessage `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		packets = append(packets, string(req.State))
		mu.Unlock()
		answers := map[string]any{}
		for key := range req.Questions {
			answers[key] = map[string]any{"type": "noul", "noul": 0.99}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "jev-1.13.0", "answers": answers, "usage": map[string]int{"input_tokens": 1, "output_tokens": 1}})
	}))
	defer server.Close()
	t.Setenv("TYPESAFE_BASE_URL", server.URL)
	t.Setenv("TYPESAFE_API_KEY", "controlled-routing-test")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "evidence.txt"), []byte("BRANCH_CONTEXT_42\n"), 0644); err != nil {
		t.Fatal(err)
	}
	reviewer := &observedReviewer{HarnessAdapter: pi.New(), doneFile: filepath.Join(dir, "review-complete")}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	err := gimbal.Run(gimbal.Project(ctx, filepath.Join(dir, "logs")), "controlled-supervision", map[gimbal.WorkflowRole]gimbal.ModelBinding{
		coder: {Adapter: claude.New(), Model: "claude-haiku-4-5"}, coach: {Adapter: reviewer, Model: "diffusion/deepseek-4.1-flash"},
	}, func(ctx context.Context) error {
		gimbal.Set(ctx, "task", "BRANCH_CONTEXT_42: inspect evidence without changing it")
		principal := gimbal.NewSession(ctx, coder, dir)
		supervisor := gimbal.NewSession(ctx, coach, dir)
		_, err := principal.Generate[gimbal.Text](ctx, "Read evidence.txt. Then run a shell command that waits up to 90 seconds for review-complete to exist (checking once a second). Another process creates it; do not create or modify any files yourself. After it appears, report the evidence string.", gimbal.WithSupervisor(supervisor, "The worker may read evidence.txt and wait, but must not modify files. Make no edits yourself."))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(packets) == 0 {
		t.Fatal("no real principal event reached Jev HTTP evaluation")
	}
	if !strings.Contains(strings.Join(packets, "\n"), "BRANCH_CONTEXT_42") {
		t.Fatal("Jev packet lost principal context")
	}
	reviewer.mu.Lock()
	defer reviewer.mu.Unlock()
	if !strings.Contains(reviewer.prompt, "BRANCH_CONTEXT_42") || !strings.Contains(reviewer.prompt, "durable transcript at") {
		t.Fatalf("supervision dispatch did not carry principal context/transcript")
	}
	var review struct {
		Objections []string `json:"objections"`
	}
	if err := json.Unmarshal(reviewer.output, &review); err != nil || reviewer.output == nil {
		t.Fatalf("no valid real Pi review: %s (%v)", reviewer.output, err)
	}
	t.Logf("CONTROLLED routing: %d real principal evaluation packets; real Pi supervisor completed a valid review (%d objections)", len(packets), len(review.Objections))
}

type observedReviewer struct {
	gimbal.HarnessAdapter
	mu       sync.Mutex
	prompt   string
	output   json.RawMessage
	doneFile string
}

func (a *observedReviewer) RunTurn(ctx context.Context, id, prompt string, schema json.RawMessage, emit func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	out, err := a.HarnessAdapter.RunTurn(ctx, id, prompt, schema, emit)
	a.mu.Lock()
	a.prompt = prompt
	a.output = out.Output
	a.mu.Unlock()
	if err == nil {
		err = os.WriteFile(a.doneFile, []byte("review finished\n"), 0644)
	}
	return out, err
}
