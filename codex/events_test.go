package codex

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/tylergannon/gimble"
)

func TestProjectorBindsRawResponseAndNormalizesAccounting(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("thread-1", "turn-1", "gpt-test", func(event gimble.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	mustProject(t, p.itemStarted(json.RawMessage(`{"item":{"id":"msg-item","type":"agentMessage"}}`)))
	mustProject(t, p.textDelta(json.RawMessage(`{"delta":"hel"}`)))
	_, _, err := p.itemCompleted(json.RawMessage(`{"item":{"id":"msg-item","type":"agentMessage","text":"hello"}}`))
	mustProject(t, err)
	mustProject(t, p.rawResponseCompleted(json.RawMessage(`{"responseId":"resp_123","usage":{"input_tokens":100,"cached_input_tokens":25,"cache_write_input_tokens":5,"output_tokens":30,"reasoning_output_tokens":10}}`)))
	if !slices.Contains(types(events), "session.step.ended") {
		t.Fatal("settled rawResponse/completed did not end the tool-free step")
	}

	want := []string{"session.step.started", "session.text.started", "session.text.delta", "session.text.ended", "session.step.streamed", "session.step.ended"}
	if got := types(events); !slices.Equal(got, want) {
		t.Fatalf("event types = %v, want %v", got, want)
	}
	var started, streamed, ended map[string]any
	startedEvent := firstType(t, events, "session.step.started")
	streamedEvent := firstType(t, events, "session.step.streamed")
	endedEvent := firstType(t, events, "session.step.ended")
	decodeData(t, startedEvent, &started)
	decodeData(t, streamedEvent, &streamed)
	decodeData(t, endedEvent, &ended)
	if started["assistantMessageID"] != streamed["assistantMessageID"] || streamed["assistantMessageID"] != ended["assistantMessageID"] {
		t.Fatalf("provisional identity changed: %#v %#v %#v", started, streamed, ended)
	}
	if !jsonContains(streamedEvent.NativeRef, `"responseID":"resp_123"`) || !jsonContains(endedEvent.NativeRef, `"tokensAvailable":true`) || !jsonContains(endedEvent.NativeRef, `"costAvailable":false`) {
		t.Fatalf("boundary/accounting audit missing: streamed=%s ended=%s", streamedEvent.NativeRef, endedEvent.NativeRef)
	}
	tokens := ended["tokens"].(map[string]any)
	cache := tokens["cache"].(map[string]any)
	if tokens["input"] != float64(70) || tokens["output"] != float64(20) || tokens["reasoning"] != float64(10) || cache["read"] != float64(25) || cache["write"] != float64(5) {
		t.Fatalf("normalized tokens = %#v", tokens)
	}
}

func TestProjectorWaitsForToolAndDistinguishesFailure(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("thread-1", "turn-1", "gpt-test", func(event gimble.AgentEvent) error { events = append(events, event); return nil })
	mustProject(t, p.itemStarted(json.RawMessage(`{"item":{"id":"call-1","type":"commandExecution","command":"false","cwd":"/w"}}`)))
	mustProject(t, p.rawResponseCompleted(json.RawMessage(`{"responseId":"resp_tool"}`)))
	if slices.Contains(types(events), "session.step.ended") {
		t.Fatal("step ended with a tool still open even without tokenUsage")
	}
	_, _, err := p.itemCompleted(json.RawMessage(`{"item":{"id":"call-1","type":"commandExecution","status":"failed","aggregatedOutput":"boom"}}`))
	mustProject(t, err)
	if got := types(events); got[len(got)-2] != "session.tool.failed" || got[len(got)-1] != "session.step.ended" {
		t.Fatalf("terminal event types = %v", got)
	}

	before := len(events)
	mustProject(t, p.itemStarted(json.RawMessage(`{"item":{"id":"compact-1","type":"contextCompaction"}}`)))
	mustProject(t, p.rawResponseCompleted(json.RawMessage(`{"responseId":"resp_compact","usage":{}}`)))
	_, _, err = p.itemCompleted(json.RawMessage(`{"item":{"id":"compact-1","type":"contextCompaction"}}`))
	mustProject(t, err)
	if got := types(events[before:]); len(got) != 0 {
		t.Fatalf("compaction produced ordinary assistant events: %v", types(events[before:]))
	}
}

func TestProjectorCompletesTwoResponsesWithoutTokenUsage(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("thread", "turn", "model", func(event gimble.AgentEvent) error { events = append(events, event); return nil })
	for i, response := range []string{"resp_one", "resp_two"} {
		item := fmt.Sprintf("item-%d", i)
		mustProject(t, p.itemStarted(json.RawMessage(fmt.Sprintf(`{"item":{"id":%q,"type":"agentMessage"}}`, item))))
		_, _, err := p.itemCompleted(json.RawMessage(fmt.Sprintf(`{"item":{"id":%q,"type":"agentMessage","text":"done"}}`, item)))
		mustProject(t, err)
		mustProject(t, p.rawResponseCompleted(json.RawMessage(fmt.Sprintf(`{"responseId":%q}`, response))))
	}
	if got := countType(events, "session.step.ended"); got != 2 {
		t.Fatalf("step.ended count = %d, want 2; types=%v", got, types(events))
	}
	var ids []string
	for _, event := range events {
		if event.Type != "session.step.ended" {
			continue
		}
		var data map[string]any
		decodeData(t, event, &data)
		ids = append(ids, data["assistantMessageID"].(string))
	}
	if ids[0] == ids[1] {
		t.Fatalf("responses reused message identity: %v", ids)
	}
}

func TestProjectorMarksTurnBoundaryWhenResumedTurnHasNoRawResponse(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("thread", "turn", "model", func(event gimble.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	mustProject(t, p.itemStarted(json.RawMessage(`{"item":{"id":"message","type":"agentMessage"}}`)))
	_, _, err := p.itemCompleted(json.RawMessage(`{"item":{"id":"message","type":"agentMessage","text":"done"}}`))
	mustProject(t, err)
	mustProject(t, p.turnCompleted(json.RawMessage(`{"turn":{"status":"completed"}}`)))

	if slices.Contains(types(events), "session.step.streamed") {
		t.Fatal("turn boundary was presented as an observed raw response boundary")
	}
	ended := firstType(t, events, "session.step.ended")
	if !jsonContains(ended.NativeRef, `"boundarySource":"turn/completed"`) || !jsonContains(ended.NativeRef, `"tokensAvailable":false`) {
		t.Fatalf("fallback boundary is not explicit: %s", ended.NativeRef)
	}
}

func TestNormalizeUsageDistinguishesKnownZeroFromNullableMissing(t *testing.T) {
	known := normalizeUsage(map[string]any{"inputTokens": float64(0), "cachedInputTokens": float64(0), "cacheWriteInputTokens": float64(0), "outputTokens": float64(0), "reasoningOutputTokens": float64(0)}, true)
	if !known.available {
		t.Fatal("observed numeric zeros were marked unavailable")
	}
	missing := normalizeUsage(map[string]any{"inputTokens": nil, "outputTokens": float64(0)}, true)
	if missing.available || missing.fieldAvailability["input"] || missing.fieldAvailability["output"] {
		t.Fatalf("nullable/missing fields were marked available: %+v", missing)
	}
}

func TestProjectorRejectsSecondOpenTextAndPropagatesCallbackError(t *testing.T) {
	p := newProjector("thread", "turn", "model", func(gimble.AgentEvent) error { return nil })
	mustProject(t, p.itemStarted(json.RawMessage(`{"item":{"id":"one","type":"agentMessage"}}`)))
	if err := p.itemStarted(json.RawMessage(`{"item":{"id":"two","type":"agentMessage"}}`)); err == nil {
		t.Fatal("second open text part was accepted")
	}

	boom := errors.New("observer stopped")
	p = newProjector("thread", "turn", "model", func(gimble.AgentEvent) error { return boom })
	if err := p.itemStarted(json.RawMessage(`{"item":{"id":"one","type":"agentMessage"}}`)); !errors.Is(err, boom) {
		t.Fatalf("callback error = %v, want %v", err, boom)
	}
}

func types(events []gimble.AgentEvent) []string {
	var out []string
	for _, event := range events {
		if event.Type != "" {
			out = append(out, event.Type)
		}
	}
	return out
}

func decodeData(t *testing.T, event gimble.AgentEvent, target any) {
	t.Helper()
	if err := json.Unmarshal(event.Data, target); err != nil {
		t.Fatal(err)
	}
}

func firstType(t *testing.T, events []gimble.AgentEvent, eventType string) gimble.AgentEvent {
	t.Helper()
	for _, event := range events {
		if event.Type == eventType {
			return event
		}
	}
	t.Fatalf("event %s not found", eventType)
	return gimble.AgentEvent{}
}

func countType(events []gimble.AgentEvent, eventType string) int {
	count := 0
	for _, event := range events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}

func mustProject(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func jsonContains(raw json.RawMessage, text string) bool { return bytes.Contains(raw, []byte(text)) }
