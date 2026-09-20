package claude

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"strings"
	"sync"
	"testing"
	"time"

	claudeagent "github.com/roasbeef/claude-agent-sdk-go"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/tylergannon/gimble"
)

type scriptedTurnStream struct {
	messages  chan claudeagent.Message
	interrupt func()
	once      sync.Once
}

func (s *scriptedTurnStream) Messages() iter.Seq[claudeagent.Message] {
	return func(yield func(claudeagent.Message) bool) {
		for message := range s.messages {
			if !yield(message) {
				return
			}
		}
	}
}

func (s *scriptedTurnStream) InterruptWithReceipt(context.Context) (*claudeagent.InterruptReceipt, error) {
	s.once.Do(func() {
		if s.interrupt != nil {
			s.interrupt()
		}
	})
	return &claudeagent.InterruptReceipt{}, nil
}

func result(session, state string, value any, origin claudeagent.MessageOriginKind) claudeagent.ResultMessage {
	structured := map[string]any{"state": state}
	if state == "waiting" {
		structured["message"] = "pending"
	}
	if value != nil {
		structured["value"] = value
	}
	message := claudeagent.ResultMessage{Type: "result", Status: "success", Subtype: "success", SessionID: session, StructuredOutput: structured}
	if origin != "" {
		message.Origin = &claudeagent.MessageOrigin{Kind: origin}
	}
	return message
}

func promptBarrier(messages chan<- claudeagent.Message, sessionID string) {
	messages <- claudeagent.SystemMessage{Type: "system", Subtype: "init", SessionID: sessionID}
	requesting := claudeagent.SDKStatusRequesting
	messages <- claudeagent.StatusMessage{Type: "system", Subtype: "status", Status: &requesting, SessionID: sessionID}
}

func TestWaitTurnAttributesCompletionAfterCurrentPrompt(t *testing.T) {
	const sessionID = "session"
	state, decoded, valid := completionValue(result(sessionID, "completed", "fresh", claudeagent.MessageOriginKindTaskNotification).StructuredOutput)
	if state != "completed" || string(decoded) != `"fresh"` || !valid {
		t.Fatalf("completionValue = %q/%s/%v", state, decoded, valid)
	}
	stream := &scriptedTurnStream{messages: make(chan claudeagent.Message, 5)}
	// Origin and schema validity do not attribute this buffered result to the
	// prompt: it arrived before the process accepted a new request.
	stream.messages <- result(sessionID, "completed", "stale", "")
	promptBarrier(stream.messages, sessionID)
	stream.messages <- result(sessionID, "waiting", "provisional", "")
	stream.messages <- result(sessionID, "completed", "fresh", claudeagent.MessageOriginKindTaskNotification)
	close(stream.messages)

	nativeErrors := make(chan error)
	out, err := waitTurn(context.Background(), stream, nativeErrors, sessionID, &session{})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `"fresh"` {
		t.Fatalf("result = %s, want current prompt's automatic completion", out)
	}
}

func TestWaitTurnRejectsMissingTextPayload(t *testing.T) {
	stream := &scriptedTurnStream{messages: make(chan claudeagent.Message, 3)}
	promptBarrier(stream.messages, "session")
	stream.messages <- result("session", "completed", nil, "")
	close(stream.messages)
	out, err := waitTurn(context.Background(), stream, make(chan error), "session", &session{})
	if err != nil {
		t.Fatal(err)
	}
	if json.Valid(out) {
		t.Fatalf("missing payload became valid JSON %s", out)
	}
	if err := (gimble.Text("")).ValidateJSON(out); err == nil {
		t.Fatal("missing payload became a successful empty Text")
	}
}

func TestWaitTurnRejectsMalformedEnvelope(t *testing.T) {
	stream := &scriptedTurnStream{messages: make(chan claudeagent.Message, 3)}
	promptBarrier(stream.messages, "session")
	malformed := result("session", "completed", "wrong", "")
	malformed.StructuredOutput.(map[string]any)["unexpected"] = true
	stream.messages <- malformed
	close(stream.messages)
	out, err := waitTurn(context.Background(), stream, make(chan error), "session", &session{})
	if err != nil {
		t.Fatal(err)
	}
	if json.Valid(out) {
		t.Fatalf("malformed envelope became valid caller output %s", out)
	}
}

func TestWaitTurnTerminatesWhileWaiting(t *testing.T) {
	t.Run("EOF", func(t *testing.T) {
		stream := &scriptedTurnStream{messages: make(chan claudeagent.Message, 3)}
		promptBarrier(stream.messages, "session")
		stream.messages <- result("session", "waiting", nil, "")
		close(stream.messages)
		_, err := waitTurn(context.Background(), stream, make(chan error), "session", &session{})
		if err == nil || !strings.Contains(err.Error(), "without a completed result") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("provider failure", func(t *testing.T) {
		stream := &scriptedTurnStream{messages: make(chan claudeagent.Message, 4)}
		promptBarrier(stream.messages, "session")
		stream.messages <- result("session", "waiting", nil, "")
		stream.messages <- claudeagent.ResultMessage{Type: "result", Status: "error", Subtype: "error_during_execution", SessionID: "session", Errors: []string{"provider stopped"}}
		close(stream.messages)
		_, err := waitTurn(context.Background(), stream, make(chan error), "session", &session{})
		if err == nil || !strings.Contains(err.Error(), "provider stopped") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("cancellation", func(t *testing.T) {
		stream := &scriptedTurnStream{messages: make(chan claudeagent.Message)}
		stream.interrupt = func() { close(stream.messages) }
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := waitTurn(ctx, stream, make(chan error), "session", &session{})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context cancellation", err)
		}
	})
}

func TestCompletionSchemaPreservesCallerReferences(t *testing.T) {
	caller := json.RawMessage(`{
		"type":"object",
		"properties":{"head":{"$ref":"#/$defs/node"}},
		"required":["head"],
		"additionalProperties":false,
		"$defs":{"node":{"type":"object","properties":{"name":{"type":"string"},"next":{"anyOf":[{"$ref":"#/$defs/node"},{"type":"null"}]}},"required":["name","next"],"additionalProperties":false}}
	}`)
	composed, err := completionSchema(caller)
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	var document any
	if err := json.Unmarshal(composed, &document); err != nil {
		t.Fatal(err)
	}
	if err := compiler.AddResource("completion.json", document); err != nil {
		t.Fatal(err)
	}
	compiled, err := compiler.Compile("completion.json")
	if err != nil {
		t.Fatalf("composed schema does not compile: %v\n%s", err, composed)
	}
	for _, raw := range []string{
		`{"state":"waiting","message":"command is running"}`,
		`{"state":"completed","value":{"head":{"name":"a","next":{"name":"b","next":null}}}}`,
	} {
		value, err := jsonschema.UnmarshalJSON(strings.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := compiled.Validate(value); err != nil {
			t.Fatalf("%s does not validate: %v\n%s", raw, err, composed)
		}
	}
}

func TestCompletionSchemaPreservesNamedAnchors(t *testing.T) {
	caller := json.RawMessage(`{
		"$anchor":"node",
		"type":"object",
		"properties":{"next":{"anyOf":[{"$ref":"#node"},{"type":"null"}]}},
		"required":["next"],
		"additionalProperties":false
	}`)
	compiled := compileCompletionSchema(t, caller)
	value, err := jsonschema.UnmarshalJSON(strings.NewReader(`{"state":"completed","value":{"next":{"next":null}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := compiled.Validate(value); err != nil {
		t.Fatalf("named-anchor completion does not validate: %v", err)
	}
}

func TestCompletionSchemaDoesNotRewriteLiteralRefData(t *testing.T) {
	caller := json.RawMessage(`{
		"type":"object",
		"properties":{"choice":{
			"const":{"$ref":"#const-literal"},
			"default":{"$ref":"#default-literal"},
			"examples":[{"$ref":"#example-literal"}]
		}},
		"required":["choice"],
		"additionalProperties":false
	}`)
	composed, err := completionSchema(caller)
	if err != nil {
		t.Fatal(err)
	}
	for _, literal := range []string{"#const-literal", "#default-literal", "#example-literal"} {
		if !strings.Contains(string(composed), literal) {
			t.Fatalf("literal %q was rewritten in %s", literal, composed)
		}
	}
	compiled := compileCompletionSchema(t, caller)
	value, err := jsonschema.UnmarshalJSON(strings.NewReader(`{"state":"completed","value":{"choice":{"$ref":"#const-literal"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := compiled.Validate(value); err != nil {
		t.Fatalf("original const literal does not validate: %v", err)
	}
}

func TestCompletionSchemaTreatsPropertyNamesAsNames(t *testing.T) {
	caller := json.RawMessage(`{
		"type":"object",
		"properties":{
			"name":{"type":"string"},
			"default":{"$ref":"#/properties/name"},
			"$id":{"$ref":"#/properties/name"}
		},
		"required":["default","$id"],
		"additionalProperties":false
	}`)
	compiled := compileCompletionSchema(t, caller)
	value, err := jsonschema.UnmarshalJSON(strings.NewReader(`{"state":"completed","value":{"default":"ok","$id":"also ok"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := compiled.Validate(value); err != nil {
		t.Fatalf("property-name schemas were not relocated: %v", err)
	}
}

func compileCompletionSchema(t *testing.T, caller json.RawMessage) *jsonschema.Schema {
	t.Helper()
	composed, err := completionSchema(caller)
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	var document any
	if err := json.Unmarshal(composed, &document); err != nil {
		t.Fatal(err)
	}
	if err := compiler.AddResource("completion.json", document); err != nil {
		t.Fatal(err)
	}
	compiled, err := compiler.Compile("completion.json")
	if err != nil {
		t.Fatalf("composed schema does not compile: %v\n%s", err, composed)
	}
	return compiled
}

type retryAdapter struct {
	calls   int
	prompts []string
}

func (a *retryAdapter) CreateSession(context.Context, string, string, string) (string, error) {
	return "native", nil
}
func (a *retryAdapter) Fork(context.Context, string) (string, error) { return "fork", nil }
func (a *retryAdapter) Close(context.Context, string) error          { return nil }
func (a *retryAdapter) Steer(context.Context, string, string) (bool, error) {
	return false, nil
}
func (a *retryAdapter) RunTurn(_ context.Context, _ string, prompt string, _ json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	a.calls++
	a.prompts = append(a.prompts, prompt)
	if a.calls == 1 {
		_, _, valid := completionValue(map[string]any{"state": "completed", "value": "wrong", "unexpected": true})
		if valid {
			return gimble.TurnResult{}, errors.New("malformed Claude envelope was accepted")
		}
		return gimble.TurnResult{Output: invalidCompletion}, nil
	}
	return gimble.TurnResult{Output: json.RawMessage(`"recovered"`)}, nil
}

func TestMalformedCompletionUsesGenerateValidationRetry(t *testing.T) {
	adapter := &retryAdapter{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = gimble.Project(ctx, t.TempDir())
	err := gimble.Run(ctx, "claude-completion-retry", map[gimble.WorkflowRole]gimble.ModelBinding{
		"worker": {Adapter: adapter, Model: "test"},
	}, func(ctx context.Context) error {
		worker := gimble.NewSession(ctx, "worker", t.TempDir())
		got, err := worker.Generate[gimble.Text](ctx, "answer")
		if err != nil {
			return err
		}
		if got != "recovered" {
			t.Fatalf("Text = %q", got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if adapter.calls != 2 || !strings.Contains(adapter.prompts[1], "previous answer was invalid") {
		t.Fatalf("calls/prompts = %d/%q, want bounded validation re-ask", adapter.calls, adapter.prompts)
	}
}
