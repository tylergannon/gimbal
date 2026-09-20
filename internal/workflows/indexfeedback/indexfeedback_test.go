package indexfeedback

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimble"
)

type feedbackAdapter struct {
	t           *testing.T
	next        int
	retrievals  int
	failSampler bool
}

func (a *feedbackAdapter) CreateSession(context.Context, string, string, string) (string, error) {
	a.next++
	return fmt.Sprint(a.next), nil
}
func (a *feedbackAdapter) Close(context.Context, string) error { return nil }
func (a *feedbackAdapter) Fork(context.Context, string) (string, error) {
	panic("must use fresh sessions")
}
func (a *feedbackAdapter) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (a *feedbackAdapter) RunTurn(ctx context.Context, id, prompt string, schema json.RawMessage, emit func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	if strings.HasPrefix(prompt, samplePrompt) {
		if a.failSampler {
			return gimble.TurnResult{}, fmt.Errorf("sampler unavailable")
		}
		out, _ := json.Marshal(Questions{Questions: []Sample{
			{"first question", "PRIVATE_EVIDENCE one"},
			{"second question", "PRIVATE_EVIDENCE two"},
			{"third question", "PRIVATE_EVIDENCE three"},
		}})
		return gimble.TurnResult{Output: out}, nil
	}
	if strings.HasPrefix(prompt, retrievePrompt) {
		a.retrievals++
		if strings.Contains(prompt, "PRIVATE_EVIDENCE") || strings.Contains(prompt, "feedback file") || strings.Contains(prompt, "BUILDER_CONTEXT") {
			a.t.Fatalf("retriever saw private context: %s", prompt)
		}
		if id != fmt.Sprint(a.retrievals+1) {
			a.t.Fatalf("retriever session reused: %s", id)
		}
		for _, event := range []gimble.AgentEvent{
			{Type: "session.step.started", Data: json.RawMessage(`{"model":{"id":"test","providerID":"test"}}`)},
			{Type: "session.tool.input.started", Data: json.RawMessage(`{"id":"read1","name":"exec_command"}`)},
			{Type: "session.tool.called", Data: json.RawMessage(`{"id":"read1","executed":true}`)},
			{Type: "session.tool.success", Data: json.RawMessage(`{"id":"read1","executed":true,"content":[{"type":"text","text":"source"}]}`)},
		} {
			var data map[string]any
			_ = json.Unmarshal(event.Data, &data)
			data["sessionID"] = id
			data["assistantMessageID"] = "message"
			event.Data, _ = json.Marshal(data)
			event.NativeRef = json.RawMessage(`{"provider":"test"}`)
			if err := emit(event); err != nil {
				return gimble.TurnResult{}, err
			}
		}
		if a.retrievals == 2 {
			return gimble.TurnResult{}, fmt.Errorf("retrieval interrupted")
		}
	}
	if strings.HasPrefix(prompt, assessPrompt) && (!strings.Contains(prompt, "PRIVATE_EVIDENCE") || !strings.Contains(prompt, "retrieval interrupted")) {
		a.t.Fatalf("assessor lacks evidence or failure: %s", prompt)
	}
	if strings.HasPrefix(prompt, assessPrompt) && (strings.Contains(prompt, "feedback file") || strings.Contains(prompt, "BUILDER_CONTEXT")) {
		a.t.Fatalf("assessor saw output or builder context: %s", prompt)
	}
	return gimble.TurnResult{Output: json.RawMessage(`"Source-backed feedback"`), Usage: map[string]gimble.Usage{"test": {Tokens: gimble.Tokens{Input: 10, Output: 2}}}}, nil
}

func TestFeedbackFreshRetrievalAccountingAndAdvisoryFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			workdir := t.TempDir()
			corpus := filepath.Join(workdir, "corpus")
			if err := os.Mkdir(corpus, 0o755); err != nil {
				t.Fatal(err)
			}
			index := filepath.Join(corpus, "INDEX.md")
			if err := os.WriteFile(index, []byte("source index"), 0o644); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(workdir, "feedback.md")
			a := &feedbackAdapter{t: t, failSampler: fail}
			models := map[gimble.WorkflowRole]gimble.ModelBinding{}
			for _, role := range []gimble.WorkflowRole{"index-sampling", "index-retrieval", "index-assessment"} {
				models[role] = gimble.ModelBinding{Adapter: a, Model: "test"}
			}
			err := gimble.Run(gimble.Project(t.Context(), filepath.Join(workdir, ".gimble")), "index-feedback", models, func(ctx context.Context) error {
				gimble.Set(ctx, "builder secret", "BUILDER_CONTEXT")
				return IndexFeedback(ctx, gimble.Env{WorkDir: workdir}, Params{Goal: "answer useful questions", Corpus: corpus, Index: index, Output: output})
			})
			if (err != nil) != fail {
				t.Fatalf("run error = %v", err)
			}
			data, e := os.ReadFile(output)
			if e != nil {
				t.Fatal(e)
			}
			if fail {
				if !strings.Contains(string(data), "incomplete") || !strings.Contains(string(data), "sampler unavailable") {
					t.Fatalf("feedback: %s", data)
				}
				return
			}
			for _, want := range []string{"Source-backed feedback", "retrieval interrupted", "tool calls: 1", "6 bytes", "Recorded provider usage"} {
				if !strings.Contains(string(data), want) {
					t.Errorf("feedback lacks %q: %s", want, data)
				}
			}
			if a.retrievals != 3 {
				t.Fatalf("retrievals = %d", a.retrievals)
			}
		})
	}
}

func TestToolCountsDeduplicateFailuresAndExcludeSubmission(t *testing.T) {
	var log strings.Builder
	for _, event := range []struct{ kind, data string }{
		{"input.started", `{"id":"read","name":"exec_command"}`},
		{"called", `{"id":"read","executed":true}`},
		{"progress", `{"id":"read","metadata":{"output":"partial"}}`},
		{"success", `{"id":"read","content":[{"type":"text","text":"1234567890"}]}`},
		{"success", `{"id":"read","content":[{"type":"text","text":"1234567890"}]}`},
		{"called", `{"id":"bad","executed":true}`},
		{"failed", `{"id":"bad","error":{"message":"oops"}}`},
		{"called", `{"id":"unfinished","executed":true}`},
		{"input.started", `{"id":"submit","name":"submit_result"}`},
		{"success", `{"id":"submit","content":[{"type":"text","text":"not retrieval"}]}`},
		{"called", `{"id":"not-executed","executed":false}`},
	} {
		record := gimble.AgentRecord{Turn: "turn1", Event: gimble.AgentEvent{Type: "session.tool." + event.kind, Data: json.RawMessage(event.data)}}
		if err := json.NewEncoder(&log).Encode(record); err != nil {
			t.Fatal(err)
		}
	}
	got, err := countTools(strings.NewReader(log.String()))
	if err != nil {
		t.Fatal(err)
	}
	if got != (toolCounts{calls: 3, failed: 1, unfinished: 1, bytes: 14}) {
		t.Fatalf("counts = %+v", got)
	}
}
