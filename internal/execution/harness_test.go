package execution

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
)

type harnessTestAdapter struct {
	run func(context.Context, func(gimbal.AgentEvent) error) (gimbal.TurnResult, error)
}

func (h harnessTestAdapter) CreateSession(context.Context, string, string, string) (string, error) {
	return "session", nil
}
func (h harnessTestAdapter) RunTurn(ctx context.Context, _, _ string, _ json.RawMessage, emit func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	return h.run(ctx, emit)
}
func (h harnessTestAdapter) Steer(context.Context, string, string) (bool, error) { return true, nil }
func (h harnessTestAdapter) Fork(context.Context, string) (string, error)        { return "fork", nil }
func (h harnessTestAdapter) Close(context.Context, string) error                 { return nil }

func TestHarnessTurnKeepsProviderResultWhenEventPersistenceFails(t *testing.T) {
	want := gimbal.TurnResult{Output: json.RawMessage(`"provider-result"`), Usage: map[string]gimbal.Usage{"model": {Tokens: gimbal.Tokens{Input: 2, Output: 3}}}}
	adapter := harnessTestAdapter{run: func(_ context.Context, emit func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
		if err := emit(gimbal.AgentEvent{Type: "session.message.completed", Data: json.RawMessage(`{"sessionID":"session"}`)}); err != nil {
			t.Fatalf("emit error = %v", err)
		}
		return want, nil
	}}
	result, degraded, err := runHarnessTurn(context.Background(), adapter, "session", "prompt", nil, func(gimbal.AgentEvent) error { return errors.New("postgres write failed") })
	if err != nil {
		t.Fatalf("runHarnessTurn error = %v", err)
	}
	if string(result.Output) != string(want.Output) || result.Usage["model"] != want.Usage["model"] {
		t.Fatalf("result = %#v, want provider result %#v", result, want)
	}
	if !strings.Contains(degraded, "postgres write failed") {
		t.Fatalf("recording degradation = %q", degraded)
	}
}

func TestHarnessTurnCancellationReachesNativeAdapter(t *testing.T) {
	started := make(chan struct{})
	adapter := harnessTestAdapter{run: func(ctx context.Context, _ func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
		close(started)
		<-ctx.Done()
		return gimbal.TurnResult{}, ctx.Err()
	}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, _, err := runHarnessTurn(ctx, adapter, "session", "prompt", nil, func(gimbal.AgentEvent) error { return nil })
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("adapter did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("runHarnessTurn error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not reach adapter")
	}
}

func TestBackendRunsScopeLogicalEnvironmentAndCodexState(t *testing.T) {
	firstName, firstQueue, firstContainer, firstVolume := environmentIdentity("owner-11111111", "development")
	secondName, secondQueue, secondContainer, secondVolume := environmentIdentity("owner-22222222", "development")
	if firstName == secondName || firstQueue == secondQueue || firstContainer == secondContainer || firstVolume == secondVolume {
		t.Fatalf("two runs selected colliding worker identity: first=%q/%q/%q/%q second=%q/%q/%q/%q", firstName, firstQueue, firstContainer, firstVolume, secondName, secondQueue, secondContainer, secondVolume)
	}
}

func TestEnvironmentRejectsModelDriftBeforeCreatingSession(t *testing.T) {
	proxy := harnessProxy{role: "planner", binding: RoleBinding{Harness: "codex", Model: "gpt-5.6-luna", Effort: "low"}}
	_, err := proxy.CreateSession(context.Background(), "gpt-5.6", "low", "/workspace")
	if err == nil || !strings.Contains(err.Error(), "bound to Codex model") {
		t.Fatalf("CreateSession error = %v, want a clear role binding mismatch", err)
	}
}
