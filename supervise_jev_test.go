package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
	"unicode/utf8"

	jev "github.com/kazz187/jev-sdk-go"
	"github.com/kazz187/jev-sdk-go/jevtest"
)

func TestJevToolInputPrefix(t *testing.T) {
	input := json.RawMessage(`"` + strings.Repeat("é", 101) + `"`)
	prefix, truncated := toolInputPrefix(input)
	if !truncated || utf8.RuneCountInString(prefix) != 100 || !strings.HasPrefix(string(input), prefix) {
		t.Fatalf("prefix = %q, truncated = %t", prefix, truncated)
	}
}

func TestJevClipContextKeepsTaskAndThinkingEnds(t *testing.T) {
	clipped := jevClipContext("task start "+strings.Repeat("é", 100)+" final scope rule", 80)
	if len(clipped) > 80 || !utf8.ValidString(clipped) || !strings.HasPrefix(clipped, "task start") || !strings.HasSuffix(clipped, "final scope rule") || !strings.Contains(clipped, "middle omitted") {
		t.Errorf("context clip = %q (%d bytes)", clipped, len(clipped))
	}
}

func TestJevSupervisionPacketAndCooldown(t *testing.T) {
	provider := jevtest.New().On(jevtest.State("rendered task: keep the workflow simple"), jevtest.Yes(0.91))
	client, err := jev.New(jev.WithProvider(provider), jev.WithModel("jev-1.13.0"))
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var looks []string
	adapter := &fake{}
	adapter.answer = func(ctx context.Context, session, prompt string, schema json.RawMessage, emit func(AgentEvent) error) (string, error) {
		if session != "native-1" {
			mu.Lock()
			looks = append(looks, prompt)
			mu.Unlock()
			return `{"objections":["stop adding a plugin system"]}`, nil
		}
		input := map[string]any{"command": strings.Repeat("é", 110) + " build plugins"}
		if err := emit(fakeAgentEvent("session.tool.called", session, "message-1", map[string]any{"id": "call-1", "input": input})); err != nil {
			return "", err
		}
		if err := emit(fakeAgentEvent("session.tool.success", session, "message-1", map[string]any{"id": "call-1", "content": "SECRET_TOOL_RESULT"})); err != nil {
			return "", err
		}
		if err := emit(fakeAgentEvent("session.reasoning.ended", session, "message-1", map[string]any{"text": "I might add a plugin system."})); err != nil {
			return "", err
		}
		deadline := time.After(2 * time.Second)
		for {
			adapter.mu.Lock()
			landed := len(adapter.steers) > 0
			adapter.mu.Unlock()
			if landed {
				break
			}
			select {
			case <-deadline:
				return "", context.DeadlineExceeded
			case <-time.After(time.Millisecond):
			}
		}
		if err := emit(fakeAgentEvent("session.reasoning.ended", session, "message-2", map[string]any{"text": "I will reconsider."})); err != nil {
			return "", err
		}
		for len(provider.Calls()) < 4 {
			select {
			case <-deadline:
				return "", context.DeadlineExceeded
			case <-time.After(time.Millisecond):
			}
		}
		time.Sleep(20 * time.Millisecond)
		return "done", nil
	}
	err = runTest(t, bind(adapter, "fake", "worker", "rule_a", "rule_b"), func(ctx context.Context) error {
		worker := NewSession(ctx, "worker", ".")
		a := NewSession(ctx, "rule_a", ".")
		b := NewSession(ctx, "rule_b", ".")
		out, err := superviseWithJevClient(ctx, worker, "rendered task: keep the workflow simple", []supervisor{
			{session: a, instruction: "no plugin system"},
			{session: b, instruction: "keep code small"},
		}, nil, "/tmp/worker-transcript.jsonl", client, Text(""))
		if string(out) != `"done"` {
			t.Errorf("worker result = %q", out)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := provider.Calls()
	if len(calls) != 4 {
		t.Fatalf("Jev calls = %d, want two thinking messages times two rules", len(calls))
	}
	for _, call := range calls {
		if call.Model != "jev-1.13.0" || len(call.Questions) != 1 {
			t.Errorf("Jev request model=%q questions=%d", call.Model, len(call.Questions))
		}
		body, err := json.Marshal(call.State)
		if err != nil {
			t.Fatal(err)
		}
		state := string(body)
		if !strings.Contains(state, "rendered task: keep the workflow simple") || strings.Contains(state, "SECRET_TOOL_RESULT") {
			t.Errorf("incorrect Jev state: %s", state)
		}
		var packet struct {
			jevPacket
			Rule string `json:"supervisor_rule"`
		}
		if err := json.Unmarshal(body, &packet); err != nil {
			t.Fatal(err)
		}
		if packet.Rule != "no plugin system" && packet.Rule != "keep code small" {
			t.Errorf("rule = %q", packet.Rule)
		}
		for _, tool := range packet.ToolCalls {
			if utf8.RuneCountInString(tool.InputPrefix) > 100 {
				t.Errorf("tool input prefix has %d characters", utf8.RuneCountInString(tool.InputPrefix))
			}
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(looks) != 1 || !strings.Contains(looks[0], "rendered task: keep the workflow simple") || !strings.Contains(looks[0], "I might add a plugin system") || strings.Contains(looks[0], "SECRET_TOOL_RESULT") {
		t.Errorf("supervisor looks = %q", looks)
	}
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if len(adapter.steers) != 1 || !strings.Contains(adapter.steers[0], "stop adding a plugin system") {
		t.Errorf("steers = %q", adapter.steers)
	}
}

// stubJev exercises the real HTTP client without external credentials or calls.
func stubJev(t *testing.T) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Questions map[string]json.RawMessage `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		answers := make(map[string]any)
		for key := range req.Questions {
			answers[key] = map[string]any{"type": "noul", "noul": 0.91}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "jev-1.13.0", "answers": answers, "usage": map[string]int{"input_tokens": 1, "output_tokens": 1}})
	}))
	t.Cleanup(server.Close)
	t.Setenv("TYPESAFE_API_KEY", "test-key")
	t.Setenv("TYPESAFE_BASE_URL", server.URL)
}

func TestSuperviseWithoutTypeSafeKeyFailsBeforeWorkerTurn(t *testing.T) {
	workerRan := false
	adapter := &fake{answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
		workerRan = true
		return "done", nil
	}}
	err := runTest(t, bind(adapter, "fake", "worker", "coach"), func(ctx context.Context) error {
		t.Setenv("TYPESAFE_API_KEY", "")
		worker := NewSession(ctx, "worker", ".")
		coach := NewSession(ctx, "coach", ".")
		_, err := worker.Generate[Text](ctx, "work", WithSupervisor(coach, "stay in scope"))
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "TYPESAFE_API_KEY is required when using WithSupervisor") || workerRan {
		t.Fatalf("error=%v worker ran=%t", err, workerRan)
	}
}

func TestJevDoesNotFallBackToTimedReviews(t *testing.T) {
	for _, thinking := range []bool{false, true} {
		name := "no completed thinking"
		if thinking {
			name = "failed Jev check"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				provider := jevtest.New().On(jevtest.Any(), jevtest.Fail(errors.New("Jev unavailable")))
				client, err := jev.New(jev.WithProvider(provider))
				if err != nil {
					t.Fatal(err)
				}
				reviews := 0
				adapter := &fake{answer: func(ctx context.Context, session, prompt string, _ json.RawMessage, emit func(AgentEvent) error) (string, error) {
					if session != "native-1" {
						reviews++
						return `{"objections":[]}`, nil
					}
					_ = emit(fakeAgentEvent("session.tool.success", session, "message-1", map[string]any{"id": "call-1", "content": "plugin system"}))
					if thinking {
						_ = emit(fakeAgentEvent("session.reasoning.ended", session, "message-1", map[string]any{"text": "I will build a plugin system"}))
					}
					// Advance past the old default interval without a real four-minute wait.
					time.Sleep(4 * time.Minute)
					return "done", nil
				}}
				err = runTest(t, bind(adapter, "fake", "worker", "coach"), func(ctx context.Context) error {
					worker := NewSession(ctx, "worker", ".")
					coach := NewSession(ctx, "coach", ".")
					out, err := superviseWithJevClient(ctx, worker, "work", []supervisor{{session: coach, instruction: "no plugins"}}, nil, "/tmp/worker.jsonl", client, Text(""))
					if string(out) != `"done"` {
						t.Errorf("worker result=%q", out)
					}
					return err
				})
				wantChecks := 0
				if thinking {
					wantChecks = 1
				}
				if err != nil || reviews != 0 || len(provider.Calls()) != wantChecks {
					t.Fatalf("error=%v reviews=%d checks=%d", err, reviews, len(provider.Calls()))
				}
			})
		})
	}
}

func TestJevSupervisionRequiresTypeSafeKeyAtUse(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", " 	\n")
	_, err := superviseWithJev(t.Context(), nil, "task", nil, nil, "", Text(""))
	if err == nil || !strings.Contains(err.Error(), "TYPESAFE_API_KEY is required when using WithSupervisor") {
		t.Fatalf("error = %v", err)
	}
}
