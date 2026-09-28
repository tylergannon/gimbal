package execution

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
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

// scheduler is a Temporal client whose only operation is ExecuteActivity,
// each call returning a harness activity that completes as get says.
type scheduler struct {
	client.Client
	scheduled atomic.Int32
	get       func(ctx context.Context, cancelled <-chan struct{}) error
}

func (s *scheduler) ExecuteActivity(context.Context, client.StartActivityOptions, any, ...any) (client.ActivityHandle, error) {
	s.scheduled.Add(1)
	return &harnessActivityHandle{get: s.get, cancelled: make(chan struct{})}, nil
}

type harnessActivityHandle struct {
	client.ActivityHandle
	get       func(ctx context.Context, cancelled <-chan struct{}) error
	cancelled chan struct{}
}

func (h *harnessActivityHandle) Get(ctx context.Context, _ any) error { return h.get(ctx, h.cancelled) }
func (h *harnessActivityHandle) Cancel(context.Context, client.CancelActivityOptions) error {
	close(h.cancelled)
	return nil
}

// unconfirmedTurn is the Codex adapter's unconfirmed-turn failure as the
// controller receives it from Temporal.
var unconfirmedTurn = temporal.NewApplicationError("codex: turn t1 in thread s was not confirmed stopped", codexTurnUnconfirmed)

func harnessProxyFor(t *testing.T, get func(ctx context.Context, cancelled <-chan struct{}) error) (*harnessProxy, *scheduler, string) {
	env, dockerLog := testEnvironment(t, 0)
	s := &scheduler{get: get}
	env.backend.temporal = s
	return &harnessProxy{environment: env, role: "coder", binding: RoleBinding{Harness: "codex", Model: "gpt-5.6-luna"}}, s, dockerLog
}

// An unconfirmed turn removes the environment on either completion path, and
// every later operation in it is refused without reaching the worker.
func TestUnconfirmedTurnRemovesEnvironment(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		name := "live ctx"
		if cancelled {
			name = "cancelled ctx"
		}
		t.Run(name, func(t *testing.T) {
			proxy, s, dockerLog := harnessProxyFor(t, func(ctx context.Context, stop <-chan struct{}) error {
				if cancelled {
					<-stop
				}
				return unconfirmedTurn
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if cancelled {
				proxy.environment.backend.temporal = &cancelOnSchedule{scheduler: s, cancel: cancel}
			}
			_, err := proxy.RunTurn(ctx, "s", "prompt", nil, nil)
			if !errors.Is(err, unconfirmedTurn) {
				t.Fatalf("RunTurn = %v, want the unconfirmed turn preserved", err)
			}
			assertRemoved(t, proxy.environment, dockerLog, err)
			scheduled := s.scheduled.Load()
			if _, err := proxy.RunTurn(context.Background(), "s", "again", nil, nil); !errors.Is(err, proxy.environment.removal()) {
				t.Fatalf("RunTurn after removal = %v", err)
			}
			if _, err := proxy.CreateSession(context.Background(), "gpt-5.6-luna", "", proxy.environment.bootstrap.Mounts[0]); !errors.Is(err, proxy.environment.removal()) {
				t.Fatalf("CreateSession after removal = %v", err)
			}
			if err := proxy.Close(context.Background(), "s"); !errors.Is(err, proxy.environment.removal()) {
				t.Fatalf("Close after removal = %v", err)
			}
			if s.scheduled.Load() != scheduled {
				t.Fatal("work was scheduled in a removed environment")
			}
		})
	}
}

// cancelOnSchedule cancels the caller's ctx once the activity exists.
type cancelOnSchedule struct {
	*scheduler
	cancel context.CancelFunc
}

func (c *cancelOnSchedule) ExecuteActivity(ctx context.Context, options client.StartActivityOptions, activity any, args ...any) (client.ActivityHandle, error) {
	handle, err := c.scheduler.ExecuteActivity(ctx, options, activity, args...)
	c.cancel()
	return handle, err
}

// A request (create, fork, steer, close) follows the same rules: a confirmed
// cancellation keeps the environment, and no confirmation removes it.
func TestHarnessRequestCancellation(t *testing.T) {
	previous := activityWaitTimeout
	activityWaitTimeout = 20 * time.Millisecond
	t.Cleanup(func() { activityWaitTimeout = previous })
	for _, tc := range []struct {
		name    string
		get     func(ctx context.Context, stop <-chan struct{}) error
		removed bool
	}{
		{"confirmed", func(_ context.Context, stop <-chan struct{}) error { <-stop; return temporal.NewCanceledError() }, false},
		{"provider failure", func(_ context.Context, stop <-chan struct{}) error {
			<-stop
			return temporal.NewApplicationError("codex: close failed", "wrapError")
		}, false},
		{"unconfirmed turn", func(_ context.Context, stop <-chan struct{}) error { <-stop; return unconfirmedTurn }, true},
		{"no completion", func(ctx context.Context, _ <-chan struct{}) error { <-ctx.Done(); return ctx.Err() }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proxy, s, dockerLog := harnessProxyFor(t, tc.get)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			proxy.environment.backend.temporal = &cancelOnSchedule{scheduler: s, cancel: cancel}
			err := proxy.Close(ctx, "s")
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("Close = %v, want the cancellation", err)
			}
			if tc.removed {
				assertRemoved(t, proxy.environment, dockerLog, err)
				return
			}
			if calls := dockerCalls(t, dockerLog); calls != "" || proxy.environment.removal() != nil {
				t.Fatalf("a confirmed stop removed the environment: docker %q, removal %v", calls, proxy.environment.removal())
			}
		})
	}
}

// A provider failure is the worker's confirmation that the turn ended; only
// the unconfirmed-turn failure removes the environment.
func TestConfirmedTurnFailureKeepsEnvironment(t *testing.T) {
	failure := temporal.NewApplicationError("codex: provider failed", "errorString")
	proxy, _, dockerLog := harnessProxyFor(t, func(context.Context, <-chan struct{}) error { return failure })
	if _, err := proxy.RunTurn(context.Background(), "s", "prompt", nil, nil); !errors.Is(err, failure) {
		t.Fatalf("RunTurn = %v, want the provider failure", err)
	}
	if calls := dockerCalls(t, dockerLog); calls != "" || proxy.environment.removal() != nil {
		t.Fatalf("a confirmed failure removed the environment: docker %q, removal %v", calls, proxy.environment.removal())
	}
}
