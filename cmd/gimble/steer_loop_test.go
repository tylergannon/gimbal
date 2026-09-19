package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/web"
)

func TestSteerCommandQueuesForNextPlanningDecision(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	project := t.TempDir()
	runtime, err := web.NewRuntime(ctx, filepath.Join(project, ".gimble"), web.WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	planner := &queuedPlanner{prompts: make(chan string, 2)}
	waiting, resume := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- runtime.Run(ctx, "queued", map[gimble.WorkflowRole]gimble.ModelBinding{
			"planner": {Adapter: planner, Model: "m"},
		}, func(ctx context.Context) error {
			session := gimble.NewSession(ctx, "planner", project)
			loop := gimble.PromiseLoop(ctx, "plan", "Finish the task", session)
			for range loop.Tasks {
				close(waiting)
				select {
				case <-resume:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return loop.Err()
		})
	}()
	select {
	case <-waiting:
	case <-ctx.Done():
		t.Fatal("planner did not dispatch its task")
	}
	runID := oneCLIRunID(t, filepath.Join(project, ".gimble"))
	var output bytes.Buffer
	message := "Prioritize the CLI"
	if err := run([]string{"steer", runID, "--work-dir", project, "--loop", "plan.1", message}, &output, io.Discard, os.Getenv); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Queued bool `json:"queued"`
	}
	if err := json.Unmarshal(output.Bytes(), &response); err != nil || !response.Queued {
		t.Fatalf("queued reply = %s, %v", output.String(), err)
	}
	close(resume)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	<-planner.prompts
	if prompt := <-planner.prompts; !strings.Contains(prompt, message) {
		t.Fatalf("next planning prompt did not receive %q", message)
	}
}

type queuedPlanner struct {
	prompts chan string
	calls   int
}

func (*queuedPlanner) CreateSession(context.Context, string, string, string) (string, error) {
	return "planner", nil
}

func (p *queuedPlanner) RunTurn(_ context.Context, _, prompt string, _ json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	p.prompts <- prompt
	p.calls++
	if p.calls == 1 {
		return gimble.TurnResult{Output: json.RawMessage(`{"tasks":[{"name":"one","description":"work","definition_of_done":"finish","validation":{"command":"","query":""}}],"next":0}`)}, nil
	}
	return gimble.TurnResult{Output: json.RawMessage(`{"tasks":[],"next":null}`)}, nil
}

func (*queuedPlanner) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*queuedPlanner) Fork(context.Context, string) (string, error)        { return "fork", nil }
func (*queuedPlanner) Close(context.Context, string) error                 { return nil }
