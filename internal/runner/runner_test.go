package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/internal/live"
)

type retryClose struct {
	gimbal.HarnessAdapter
	attempts atomic.Int32
}

func (a *retryClose) CreateSession(context.Context, string, string, string) (string, error) {
	return "session", nil
}
func (a *retryClose) Close(context.Context, string) error {
	if a.attempts.Add(1) == 1 {
		return errors.New("cleanup unavailable")
	}
	return nil
}

type testOutput struct {
	Done bool `json:"done"`
}

func (testOutput) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"done":{"type":"boolean"}},"required":["done"],"additionalProperties":false}`)
}
func (testOutput) ValidateJSON([]byte) error { return nil }
func (a *retryClose) RunTurn(context.Context, string, string, json.RawMessage, func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	return gimbal.TurnResult{Output: json.RawMessage(`{"done":true}`)}, nil
}

func TestPanicReleasesRunnerAfterCloseFailure(t *testing.T) {
	project := t.TempDir()
	adapter := &retryClose{}
	read, write := io.Pipe()
	defer func() { _ = read.Close() }()
	done := make(chan error, 1)
	go func() {
		done <- run(invocation.Request{RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Project: project, Workflow: "panic"}, write, map[gimbal.WorkflowRole]gimbal.ModelBinding{"worker": {Adapter: adapter, Model: "fake"}}, func(ctx context.Context) error {
			session := gimbal.NewSession(ctx, gimbal.WorkflowRole("worker"), project)
			_, _ = session.Generate[testOutput](ctx, "one turn")
			panic("workflow failed")
		})
	}()
	var response Response
	if err := json.NewDecoder(read).Decode(&response); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "workflow failed") {
			t.Fatalf("panic result: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runner did not exit after cleanup")
	}
	if adapter.attempts.Load() != 1 {
		t.Fatalf("close attempts %d", adapter.attempts.Load())
	}
}

type controlled struct {
	live.Controller
	last atomic.Value
}

func (c *controlled) SubmitInteractive(id, message string) error {
	c.last.Store(operation{Action: "interactive", Target: id, Message: message})
	return nil
}
func (c *controlled) Steer(_ context.Context, id, message string) (bool, error) {
	c.last.Store(operation{Action: "steer", Target: id, Message: message})
	return true, nil
}
func (c *controlled) SteerLoop(id, message string) error {
	c.last.Store(operation{Action: "loop", Target: id, Message: message})
	return nil
}
func (c *controlled) CancelScope(id string, cause error) error {
	c.last.Store(operation{Action: "scope", Target: id, Message: cause.Error()})
	return nil
}
func (c *controlled) CancelTurn(id string, cause error) error {
	c.last.Store(operation{Action: "turn", Target: id, Message: cause.Error()})
	return errors.New("turn already ended")
}
func TestControlTransport(t *testing.T) {
	listener, err := net.Listen("unix", filepath.Join(t.TempDir(), "control.sock"))
	if err != nil {
		t.Fatal(err)
	}
	controller := &controlled{}
	server := &http.Server{Handler: handler(controller)}
	go func() { _ = server.Serve(listener) }()
	defer func() { _ = server.Close() }()
	client := remote{socket: listener.Addr().String()}
	if err := client.SubmitInteractive("question", "answer"); err != nil || controller.last.Load().(operation).Target != "question" {
		t.Fatalf("answer: %+v %v", controller.last, err)
	}
	if landed, err := client.Steer(t.Context(), "session", "message"); err != nil || !landed || controller.last.Load().(operation).Target != "session" {
		t.Fatalf("steer: %+v %v", controller.last, err)
	}
	if err := client.SteerLoop("loop", "finish"); err != nil || controller.last.Load().(operation).Target != "loop" {
		t.Fatalf("loop: %+v %v", controller.last, err)
	}
	if err := client.CancelScope("scope", gimbal.Killed{Target: "scope", By: "person", Reason: "stop"}); err != nil || !strings.Contains(controller.last.Load().(operation).Message, "person") {
		t.Fatalf("scope: %+v %v", controller.last, err)
	}
	if err := client.CancelTurn("turn", context.Canceled); err == nil || err.Error() != "turn already ended" || controller.last.Load().(operation).Target != "turn" {
		t.Fatalf("turn: %+v %v", controller.last, err)
	}
}

func TestTaskRouteFailureReportsAuthoritativeStartupError(t *testing.T) {
	for _, route := range []backend.Route{{Name: "missing-backend", Configuration: json.RawMessage(`{}`)}, {Name: "http", Configuration: json.RawMessage(`{"endpoint":"not-http"}`)}} {
		t.Run(route.Name, func(t *testing.T) {
			var startup bytes.Buffer
			err := Run(invocation.Request{Workflow: "review", Input: json.RawMessage(`{"goal":"inspect"}`), Project: t.TempDir(), TaskRoute: route}, &startup)
			if err == nil {
				t.Fatal("invalid task backend admitted")
			}
			var response Response
			if decodeErr := json.Unmarshal(startup.Bytes(), &response); decodeErr != nil {
				t.Fatalf("missing startup reply: %s %v", startup.String(), decodeErr)
			}
			if response.Error != err.Error() || response.RunID != "" {
				t.Fatalf("non-authoritative reply %+v, error %v", response, err)
			}
		})
	}
}
