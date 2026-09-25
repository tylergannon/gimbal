package program

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/checklist"
	"github.com/tylergannon/gimbal/harness"
)

func TestCommandReturnsNonzeroExitAndCombinedOutputAsData(t *testing.T) {
	runtime, err := NewRuntime(Config{Workdir: t.TempDir(), RunDir: t.TempDir(), Adapters: fakeAdapters(&fakeAdapter{})})
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(runtime.RunDir, "agents")); err != nil || !info.IsDir() {
		t.Fatalf("native agent log directory not prepared: %v", err)
	}
	result, err := runtime.Command(context.Background(), `printf 'out'; printf 'err' >&2; exit 7`)
	if err != nil {
		t.Fatalf("Command infrastructure error: %v", err)
	}
	if result.ExitCode != 7 || result.Output != "outerr" {
		t.Fatalf("result = %#v", result)
	}
	contents, err := os.ReadFile(filepath.Join(runtime.RunDir, "operations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), `"type":"operation_start"`) || !strings.Contains(string(contents), `"exit_code":7`) {
		t.Fatalf("operation log = %s", contents)
	}
}

func TestCommandReturnsInfrastructureFailureSeparately(t *testing.T) {
	runtime, err := NewRuntime(Config{Workdir: t.TempDir(), RunDir: t.TempDir(), Adapters: fakeAdapters(&fakeAdapter{})})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = runtime.Command(ctx, "echo never")
	if !errors.Is(err, context.Canceled) && (err == nil || !strings.Contains(err.Error(), "context canceled")) {
		t.Fatalf("error = %v", err)
	}
}

func TestCodergenDecodesValidatedResultIntoRequestedType(t *testing.T) {
	adapter := &fakeAdapter{result: json.RawMessage(`{"passed":true,"notes":"observed"}`)}
	runtime, err := NewRuntime(Config{Workdir: t.TempDir(), RunDir: t.TempDir(), Adapters: fakeAdapters(adapter)})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Codergen[Validation](context.Background(), runtime, CodergenRequest{
		Prompt: "judge", JSONSchema: validationSchema, Model: "gpt-5.6-sol",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != (Validation{Passed: true, Notes: "observed"}) {
		t.Fatalf("result = %#v", got)
	}
	if adapter.input.Model != "gpt-5.6-sol" || adapter.input.Workdir != runtime.Workdir || adapter.input.SessionID != "session-1" {
		t.Fatalf("turn = %#v", adapter.input)
	}
	if adapter.session.model != "gpt-5.6-sol" || adapter.session.workdir != runtime.Workdir {
		t.Fatalf("session = %#v", adapter.session)
	}
}

func TestCodergenWithoutSchemaReturnsText(t *testing.T) {
	adapter := &fakeAdapter{result: json.RawMessage(`"plain answer"`)}
	runtime, err := NewRuntime(Config{Workdir: t.TempDir(), RunDir: t.TempDir(), Adapters: fakeAdapters(adapter)})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Codergen[string](context.Background(), runtime, CodergenRequest{Prompt: "say it", Model: "gpt-5.6-sol"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "plain answer" || adapter.input.OutputSchema != nil {
		t.Fatalf("text = %q, schema = %s", got, adapter.input.OutputSchema)
	}
}

func TestCodergenPropagatesContextCancellation(t *testing.T) {
	adapter := &fakeAdapter{block: true}
	runtime, err := NewRuntime(Config{Workdir: t.TempDir(), RunDir: t.TempDir(), Adapters: fakeAdapters(adapter)})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Codergen[string](ctx, runtime, CodergenRequest{Prompt: "hang", Model: "gpt-5.6-sol"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestValidateRunsInferenceAfterCommandFailure(t *testing.T) {
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "evidence.txt"), []byte("actual evidence"), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := &fakeAdapter{result: json.RawMessage(`{"passed":true,"notes":"evidence itself looks right"}`)}
	runtime, err := NewRuntime(Config{Workdir: workdir, RunDir: t.TempDir(), Adapters: fakeAdapters(adapter)})
	if err != nil {
		t.Fatal(err)
	}
	got, err := runtime.Validate(context.Background(), checklist.Item{Name: "proof", Check: "observable",
		Command: "echo command-ran; exit 4", Infer: &checklist.Infer{Files: []string{"evidence.txt"}, Prompt: "inspect it"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Passed {
		t.Fatalf("validation = %#v, want failed because the command failed", got)
	}
	prompt := adapter.input.Parts[0].Text
	if !strings.Contains(prompt, "command exit: 4") || !strings.Contains(prompt, "evidence.txt") {
		t.Fatalf("judge prompt = %q", prompt)
	}
	operations, err := os.ReadFile(filepath.Join(runtime.RunDir, "operations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(operations), `"name":"validate-proof"`) {
		t.Fatalf("operations = %s", operations)
	}
}

func fakeAdapters(adapter *fakeAdapter) map[string]harness.HarnessAdapter {
	return map[string]harness.HarnessAdapter{"codex": adapter, "claude": adapter, "agy": adapter}
}

type fakeSession struct {
	model   string
	workdir string
}

// fakeAdapter records the session and turn it was asked for and returns a
// fixed result.
type fakeAdapter struct {
	result  json.RawMessage
	block   bool
	session fakeSession
	input   harness.RunTurnInput
}

func (f *fakeAdapter) CreateSession(model, workdir string) (string, error) {
	f.session = fakeSession{model: model, workdir: workdir}
	return "session-1", nil
}

func (f *fakeAdapter) RunTurn(ctx context.Context, input harness.RunTurnInput, onEvent harness.OnEvent) (json.RawMessage, error) {
	if err := harness.ValidateRunTurnInput(input, onEvent); err != nil {
		return nil, err
	}
	f.input = input
	if f.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	onEvent(harness.Event{"type": harness.EventUser, "parts": input.Parts})
	return f.result, nil
}

func (f *fakeAdapter) Steer(string, []harness.ContentPart) {}
func (f *fakeAdapter) Interrupt(string)                    {}
func (f *fakeAdapter) Compact(string, string) error        { return nil }
