package taskdaemon

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
)

type filesHarness struct {
	testHarness
	read chan string
}

func (h *filesHarness) RunTurn(ctx context.Context, id, prompt string, schema json.RawMessage, event func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	path := strings.TrimSuffix(strings.Split(prompt, " at ")[1], ".")
	raw, err := os.ReadFile(path)
	if err != nil {
		return gimbal.TurnResult{}, err
	}
	h.read <- string(raw)
	return gimbal.TurnResult{Output: json.RawMessage(`"ok"`)}, nil
}
func TestConnectionExecutesWithIndependentEventsAndExplicitNativeFiles(t *testing.T) {
	d := New(t.TempDir())
	native := &filesHarness{read: make(chan string, 1)}
	d.adapter = func(string) (gimbal.HarnessAdapter, error) { return native, nil }
	connection := d.Connection()
	connection.Events = nil
	h := &Harness{Name: "test", Connection: connection}
	ctx := effects.WithOwner(context.Background(), effects.Owner{Run: "run", Task: "session"})
	id, err := h.CreateSession(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("session")), "model", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx = effects.WithOwner(effects.WithTaskFiles(ctx, []backend.TaskFile{{Name: "source", Bytes: []byte("independent-filesystem")}}), effects.Owner{Run: "run", Task: "turn"})
	result, err := h.RunTurn(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("turn")), id, "read supplied file", nil, func(gimbal.AgentEvent) error { return nil })
	if err != nil || string(result.Output) != `"ok"` {
		t.Fatalf("result %s %v", result.Output, err)
	}
	if got := <-native.read; got != "independent-filesystem" {
		t.Fatal(got)
	}
	if err = d.cleanup(context.Background(), effects.Scope{Run: "run"}); err != nil {
		t.Fatal(err)
	}
}

func TestMaterializedTaskFileKeepsFormatExtension(t *testing.T) {
	d := New(t.TempDir())
	owner := backend.Owner{Run: "format", Scope: "format", Task: "read"}
	file := backend.Artifact{Owner: owner, Name: "report.pdf"}
	if err := d.writeBytes(file, strings.NewReader("%PDF-1.4\n")); err != nil {
		t.Fatal(err)
	}
	path := d.files[file]
	defer func() { _ = os.Remove(path) }()
	if !strings.HasSuffix(path, ".pdf") {
		t.Fatalf("PDF path lost its format: %q", path)
	}
}
