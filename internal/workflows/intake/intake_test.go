package intake

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/tylergannon/gimble"
)

type guideHarness struct {
	replies []decision
	prompts []string
}

type unreadableReader struct{}

func (unreadableReader) Read([]byte) (int, error) { panic("reader was used") }

func (*guideHarness) CreateSession(context.Context, string, string) (string, error) {
	return "guide", nil
}
func (h *guideHarness) RunTurn(_ context.Context, _ string, prompt string, _ json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	h.prompts = append(h.prompts, prompt)
	answer := h.replies[0]
	h.replies = h.replies[1:]
	data, err := json.Marshal(answer)
	return gimble.TurnResult{Output: data}, err
}
func (*guideHarness) Fork(context.Context, string) (string, error) {
	panic("guide must keep its conversation")
}
func (*guideHarness) Close(context.Context, string) error                 { return nil }
func (*guideHarness) Steer(context.Context, string, string) (bool, error) { return false, nil }

func TestGuideClarificationPreservesGoalAndAnswer(t *testing.T) {
	h := &guideHarness{replies: []decision{{Workflow: "plan", Reason: "unknown target", Question: "Which output format?"}, {Workflow: "lfg", Reason: "one clear output"}}}
	var got Advice
	err := gimble.Run(gimble.Project(context.Background(), t.TempDir()), "guide-test", func(ctx context.Context) error {
		var err error
		got, err = guide(ctx, Input{Repo: t.TempDir(), Goal: "Export the report", Acceptance: "Only the chosen format", Constraints: "Keep existing files"}, bufio.NewReader(strings.NewReader("JSON, with exact names.\n")), io.Discard, h)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Workflow != "lfg" || len(h.prompts) != 2 {
		t.Fatalf("advice=%+v prompts=%d", got, len(h.prompts))
	}
	for _, want := range []string{"Export the report", "Only the chosen format", "Keep existing files", "JSON, with exact names."} {
		if !strings.Contains(h.prompts[1], want) {
			t.Errorf("follow-up prompt lost %q", want)
		}
	}
	if !strings.Contains(got.Clarifications, "Which output format?") {
		t.Fatal("question not preserved")
	}
}

func TestGuideDoesNotInventChoiceOrAnswerAtEOF(t *testing.T) {
	for _, reply := range []decision{{Workflow: "unknown", Reason: "bad"}, {Workflow: "plan", Question: "Which database?"}} {
		h := &guideHarness{replies: []decision{reply}}
		err := gimble.Run(gimble.Project(context.Background(), t.TempDir()), "guide-test", func(ctx context.Context) error {
			_, err := guide(ctx, Input{Repo: t.TempDir(), Goal: "Build a report"}, bufio.NewReader(strings.NewReader("")), io.Discard, h)
			return err
		})
		if err == nil {
			t.Fatalf("accepted incomplete advice %+v", reply)
		}
	}
}

func TestReadAnswerChecksCancellationBeforeReadingAndAcceptsFinalLine(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readAnswer(ctx, bufio.NewReader(unreadableReader{})); err != context.Canceled {
		t.Fatalf("pre-cancel error = %v", err)
	}
	got, err := readAnswer(context.Background(), bufio.NewReader(strings.NewReader("final answer")))
	if err != nil || got != "final answer" {
		t.Fatalf("final line = %q, %v", got, err)
	}
}
