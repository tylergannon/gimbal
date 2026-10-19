package taskdaemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"

	"github.com/tylergannon/gimbal/runstore"
)

func TestInvalidOutputClassificationPreservesRawResult(t *testing.T) {
	d := New(t.TempDir())
	done := make(chan struct{})
	close(done)
	output := []byte(" \n {\"n\":\"wrong\"} \t")
	raw, _ := json.Marshal(backend.TurnResult{Output: output, Usage: json.RawMessage(`{"m":{"Cost":0.25}}`)})
	d.tasks[key("r", "t")] = &task{done: done, result: raw, err: fmt.Errorf("%w: expected integer", gimbal.ErrInvalidOutput)}
	server := httptest.NewServer(d.Handler())
	defer server.Close()
	received, err := effects.Wait(context.Background(), server.URL, effects.Target{ExecutionIdentity: backend.NewExecutionIdentity("close"), Run: "r", ID: "t"}, nil)
	if _, ok := errors.AsType[*effects.InvalidOutput](err); !ok {
		t.Fatalf("wire error=%v", err)
	}
	var wire backend.TurnResult
	if err := json.Unmarshal(received, &wire); err != nil {
		t.Fatal(err)
	}
	if string(wire.Output) != string(output) {
		t.Fatalf("wire changed native output %q", wire.Output)
	}
	result := d.result(effects.Target{ExecutionIdentity: backend.NewExecutionIdentity("close"), Run: "r", ID: "t"})
	if !result.InvalidOutput {
		t.Fatal("local backend lost invalid output classification")
	}
}

type invalidNativeHarness struct {
	testHarness
	calls         int
	alwaysInvalid bool
}

func (h *invalidNativeHarness) RunTurn(_ context.Context, _, _ string, schema json.RawMessage, _ func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	h.calls++
	raw := json.RawMessage(" \n {\"n\":2} \t")
	if h.calls < 3 || h.alwaysInvalid {
		raw = json.RawMessage(`{"n":"wrong"}`)
	}
	result := gimbal.TurnResult{Output: raw}
	return result, gimbal.ValidateTurnResult(schema, result)
}
func TestRemoteHarnessReturnsInvalidClassificationAndExactNativeOutput(t *testing.T) {
	d := New(t.TempDir())
	native := &invalidNativeHarness{}
	d.adapter = func(string) (gimbal.HarnessAdapter, error) { return native, nil }
	server := httptest.NewServer(d.Handler())
	defer server.Close()
	h := &Harness{Endpoint: server.URL, Name: "fake"}
	ctx := effects.WithOwner(context.Background(), effects.Owner{Run: "r", Task: "s"})
	id, err := h.CreateSession(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("session")), "m", "", "/")
	if err != nil {
		t.Fatal(err)
	}
	schema := json.RawMessage(`{"type":"object","properties":{"n":{"type":"integer"}},"required":["n"]}`)
	for i := 1; i <= 3; i++ {
		turn := effects.WithOwner(ctx, effects.Owner{Run: "r", Task: fmt.Sprintf("t%d", i)})
		result, err := h.RunTurn(effects.WithExecutionIdentity(turn, backend.NewExecutionIdentity("turn")), id, "answer", schema, nil)
		if i < 3 && (!errors.Is(err, gimbal.ErrInvalidOutput) || string(result.Output) != `{"n":"wrong"}`) {
			t.Fatalf("invalid result %s: %v", result.Output, err)
		}
		if i == 3 && (err != nil || string(result.Output) != " \n {\"n\":2} \t") {
			t.Fatalf("valid result %q: %v", result.Output, err)
		}
	}
}

type remoteAnswer struct {
	N int `json:"n"`
}

func (remoteAnswer) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"n":{"type":"integer"}},"required":["n"]}`)
}
func TestRemoteGenerateBoundsInvalidOutputReasks(t *testing.T) {
	d := New(t.TempDir())
	native := &invalidNativeHarness{alwaysInvalid: true}
	d.adapter = func(string) (gimbal.HarnessAdapter, error) { return native, nil }
	server := httptest.NewServer(d.Handler())
	defer server.Close()
	h := &Harness{Endpoint: server.URL, Name: "fake"}
	ctx := effects.WithEndpoint(gimbal.Project(context.Background(), t.TempDir()), server.URL)
	err := gimbal.Run(ctx, string(runstore.NewRunID()), "remote-invalid-output", map[gimbal.WorkflowRole]gimbal.ModelBinding{"coder": {Adapter: h, Model: "m"}}, func(ctx context.Context) error {
		_, err := gimbal.NewSession(ctx, "coder", t.TempDir()).Generate[remoteAnswer](ctx, "answer")
		return err
	})
	if !errors.Is(err, gimbal.ErrInvalidOutput) || native.calls != 3 {
		t.Fatalf("native calls=%d error=%v", native.calls, err)
	}
}
