package claude

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/tylergannon/gimble"
)

func TestRawProjectorUsesNestedMessageIDAndExactToolUseID(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("session-1", "claude-test", func(event gimble.AgentEvent) error { events = append(events, event); return nil })
	fixtures := []string{
		`{"type":"stream_event","uuid":"stream-envelope","session_id":"session-1","event":{"type":"message_start","message":{"id":"msg_native","model":"claude-native","usage":{"input_tokens":40,"cache_read_input_tokens":10,"cache_creation_input_tokens":5}}}}`,
		`{"type":"stream_event","uuid":"stream-envelope","session_id":"session-1","event":{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_actual","name":"Bash","input":{}}}}`,
		`{"type":"stream_event","uuid":"stream-envelope","session_id":"session-1","event":{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"command\":\"go test\"}"}}}`,
		`{"type":"stream_event","uuid":"stream-envelope","session_id":"session-1","event":{"type":"content_block_stop","index":0}}`,
		`{"type":"stream_event","uuid":"stream-envelope","session_id":"session-1","event":{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":12,"output_tokens_details":{"thinking_tokens":2}}}}`,
		`{"type":"stream_event","uuid":"stream-envelope","session_id":"session-1","event":{"type":"message_stop"}}`,
		`{"type":"assistant","uuid":"outer-assistant-uuid","session_id":"session-1","parent_tool_use_id":null,"message":{"id":"msg_native","content":[{"type":"tool_use","id":"toolu_actual","name":"Bash","input":{"command":"go test"}}]}}`,
	}
	for _, fixture := range fixtures {
		mustRaw(t, p.raw(json.RawMessage(fixture)))
	}
	if slices.Contains(claudeTypes(events), "session.step.ended") {
		t.Fatal("message_stop ended the step before its tool result")
	}
	mustRaw(t, p.raw(json.RawMessage(`{"type":"tool_progress","uuid":"progress-uuid","session_id":"session-1","tool_use_id":"toolu_actual","tool_name":"Bash","parent_tool_use_id":"ancestry-only","elapsed_time_seconds":1.5}`)))
	mustRaw(t, p.raw(json.RawMessage(`{"type":"user","uuid":"tool-result-envelope","session_id":"session-1","parent_tool_use_id":"ancestry-only","message":{"id":"user-message","content":[{"type":"tool_result","tool_use_id":"toolu_actual","content":"ok","is_error":false}]}}`)))

	wantTail := []string{"session.tool.progress", "session.tool.success", "session.step.ended"}
	got := claudeTypes(events)
	if !slices.Equal(got[len(got)-3:], wantTail) {
		t.Fatalf("event types = %v", got)
	}
	var start, success, ended map[string]any
	claudeData(t, events[0], &start)
	claudeData(t, events[len(events)-2], &success)
	claudeData(t, events[len(events)-1], &ended)
	if start["assistantMessageID"] != "msg_native" || success["id"] != "toolu_actual" {
		t.Fatalf("identity normalization used envelope/ancestry IDs: start=%#v success=%#v", start, success)
	}
	if !bytes.Contains(events[len(events)-2].NativeRef, []byte(`"parentToolUseID":"ancestry-only"`)) {
		t.Fatalf("native tool identity missing: %s", events[len(events)-2].NativeRef)
	}
	if ended["finish"] != "tool-calls" {
		t.Fatalf("finish = %v", ended["finish"])
	}
	tokens := ended["tokens"].(map[string]any)
	cache := tokens["cache"].(map[string]any)
	if tokens["input"] != float64(40) || tokens["output"] != float64(10) || tokens["reasoning"] != float64(2) || cache["read"] != float64(10) || cache["write"] != float64(5) {
		t.Fatalf("tokens = %#v", tokens)
	}
}

func TestRawProjectorStreamsTextAndZeroFillsAbsentUsage(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("session", "model", func(event gimble.AgentEvent) error { events = append(events, event); return nil })
	for _, fixture := range []string{
		`{"type":"stream_event","uuid":"outer","event":{"type":"message_start","message":{"id":"nested","model":"model"}}}`,
		`{"type":"stream_event","uuid":"outer","event":{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}}`,
		`{"type":"stream_event","uuid":"outer","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}}`,
		`{"type":"stream_event","uuid":"outer","event":{"type":"content_block_stop","index":0}}`,
		`{"type":"stream_event","uuid":"outer","event":{"type":"message_delta","delta":{"stop_reason":"end_turn"}}}`,
		`{"type":"stream_event","uuid":"outer","event":{"type":"message_stop"}}`,
	} {
		mustRaw(t, p.raw(json.RawMessage(fixture)))
	}
	got := claudeTypes(events)
	want := []string{"session.step.started", "session.text.started", "session.text.delta", "session.text.ended", "session.step.streamed", "session.step.ended"}
	if !slices.Equal(got, want) {
		t.Fatalf("event types = %v, want %v", got, want)
	}
	ended := events[len(events)-1]
	if !bytes.Contains(ended.Data, []byte(`"cost":0`)) || !bytes.Contains(ended.Data, []byte(`"tokens":{"input":0,"output":0,"reasoning":0,"cache":{"read":0,"write":0}}`)) {
		t.Fatalf("absent usage was not zero-filled: %s", ended.Data)
	}
	if bytes.Contains(ended.NativeRef, []byte("accounting")) {
		t.Fatalf("native ref still carries an accounting sidecar: %s", ended.NativeRef)
	}
}

// The fixture is the accounting half of the `result` message recorded in
// ephemeral/research/issue-149/claude/turn1.jsonl, verbatim.
const resultFixture = `{"type":"result","subtype":"success","session_id":"session","total_cost_usd":0.0241153,` +
	`"usage":{"input_tokens":10,"cache_creation_input_tokens":10341,"cache_read_input_tokens":25183,"output_tokens":181,` +
	`"output_tokens_details":{"thinking_tokens":118},"service_tier":"standard","iterations":[{"input_tokens":10,"output_tokens":181,` +
	`"cache_read_input_tokens":25183,"cache_creation_input_tokens":10341,"type":"message"}]},` +
	`"modelUsage":{"claude-haiku-4-5-20251001":{"inputTokens":10,"outputTokens":181,"cacheReadInputTokens":25183,` +
	`"cacheCreationInputTokens":10341,"webSearchRequests":0,"costUSD":0.0241153,"contextWindow":200000,"maxOutputTokens":32000,` +
	`"thinkingTokens":118,"canonicalModel":"claude-haiku-4-5","provider":"firstParty","costBasis":"list"}},` +
	`"num_turns":1,"is_error":false,"result":"Paris is the capital of France."}`

func TestRawProjectorReportsTurnUsageFromResult(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("session", "haiku", func(event gimble.AgentEvent) error { events = append(events, event); return nil })
	mustRaw(t, p.raw(json.RawMessage(resultFixture)))
	if len(events) != 0 {
		t.Fatalf("result emitted events: %v", claudeTypes(events))
	}

	want := gimble.Usage{Cost: 0.0241153}
	want.Tokens.Input, want.Tokens.Output, want.Tokens.Reasoning = 10, 63, 118
	want.Tokens.Cache.Read, want.Tokens.Cache.Write = 25183, 10341
	got := p.turnUsage()
	if len(got) != 1 || got["claude-haiku-4-5-20251001"] != want {
		t.Fatalf("turn report = %#v, want one entry %#v", got, want)
	}

	// With no modelUsage the fallback is one entry under the session's model,
	// from result.usage and total_cost_usd.
	var trimmed map[string]any
	if err := json.Unmarshal([]byte(resultFixture), &trimmed); err != nil {
		t.Fatal(err)
	}
	delete(trimmed, "modelUsage")
	raw, err := json.Marshal(trimmed)
	if err != nil {
		t.Fatal(err)
	}
	p = newProjector("session", "haiku", func(gimble.AgentEvent) error { return nil })
	mustRaw(t, p.raw(raw))
	if got := p.turnUsage(); len(got) != 1 || got["haiku"] != want {
		t.Fatalf("fallback turn report = %#v, want one entry %#v", got, want)
	}

	// A turn that states nothing reports nothing.
	p = newProjector("session", "haiku", func(gimble.AgentEvent) error { return nil })
	mustRaw(t, p.raw(json.RawMessage(`{"type":"result","subtype":"success","session_id":"session","result":"done"}`)))
	if got := p.turnUsage(); got != nil {
		t.Fatalf("turn report = %#v, want nil", got)
	}
}

func TestRawProjectorKeepsCumulativeContinuationUsage(t *testing.T) {
	p := newProjector("session", "haiku", func(gimble.AgentEvent) error { return nil })
	mustRaw(t, p.raw(json.RawMessage(resultFixture)))
	var continuation map[string]any
	if err := json.Unmarshal([]byte(resultFixture), &continuation); err != nil {
		t.Fatal(err)
	}
	continuation["total_cost_usd"] = 0.031
	model := continuation["modelUsage"].(map[string]any)["claude-haiku-4-5-20251001"].(map[string]any)
	model["costUSD"], model["inputTokens"], model["outputTokens"] = 0.031, 18.0, 300.0
	model["thinkingTokens"], model["cacheReadInputTokens"], model["cacheCreationInputTokens"] = 150.0, 40000.0, 12000.0
	raw, err := json.Marshal(continuation)
	if err != nil {
		t.Fatal(err)
	}
	mustRaw(t, p.raw(raw))

	want := gimble.Usage{Cost: 0.031}
	want.Tokens.Input, want.Tokens.Output, want.Tokens.Reasoning = 18, 150, 150
	want.Tokens.Cache.Read, want.Tokens.Cache.Write = 40000, 12000
	got := p.turnUsage()
	if len(got) != 1 || got["claude-haiku-4-5-20251001"] != want {
		t.Fatalf("continuation report = %#v, want latest cumulative %#v", got, want)
	}
}

func TestRawProjectorDistinguishesToolErrorAndGuardsSingleOpen(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("session", "model", func(event gimble.AgentEvent) error { events = append(events, event); return nil })
	mustRaw(t, p.raw(json.RawMessage(`{"type":"stream_event","event":{"type":"message_start","message":{"id":"nested","model":"model","usage":{}}}}`)))
	mustRaw(t, p.raw(json.RawMessage(`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"text"}}}`)))
	if err := p.raw(json.RawMessage(`{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"text"}}}`)); err == nil {
		t.Fatal("second open text block was accepted")
	}

	events = nil
	p = newProjector("session", "model", func(event gimble.AgentEvent) error { events = append(events, event); return nil })
	for _, fixture := range []string{
		`{"type":"stream_event","event":{"type":"message_start","message":{"id":"tool-message","model":"model","usage":{}}}}`,
		`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_error","name":"Bash","input":{}}}}`,
		`{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`,
		`{"type":"stream_event","event":{"type":"message_delta","delta":{"stop_reason":"tool_use"}}}`,
		`{"type":"stream_event","event":{"type":"message_stop"}}`,
		`{"type":"user","parent_tool_use_id":"ancestry","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_error","content":"permission denied","is_error":true}]}}`,
	} {
		mustRaw(t, p.raw(json.RawMessage(fixture)))
	}
	if got := claudeTypes(events); got[len(got)-2] != "session.tool.failed" || got[len(got)-1] != "session.step.ended" {
		t.Fatalf("tool error event types = %v", got)
	}

	boom := errors.New("observer stopped")
	p = newProjector("session", "model", func(gimble.AgentEvent) error { return boom })
	if err := p.raw(json.RawMessage(`{"type":"stream_event","event":{"type":"message_start","message":{"id":"nested","model":"model"}}}`)); !errors.Is(err, boom) {
		t.Fatalf("callback error = %v, want %v", err, boom)
	}
}

func TestRawProjectorKeepsNestedTranscriptInsideParentTool(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("session", "model", func(event gimble.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	for _, fixture := range []string{
		`{"type":"stream_event","event":{"type":"message_start","message":{"id":"parent-message","model":"model","usage":{}}}}`,
		`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"task-call","name":"Task","input":{}}}}`,
		`{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`,
		`{"type":"stream_event","parent_tool_use_id":"task-call","event":{"type":"message_start","message":{"id":"child-message","model":"model","usage":{"input_tokens":9}}}}`,
		`{"type":"stream_event","parent_tool_use_id":"task-call","event":{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}}`,
		`{"type":"stream_event","parent_tool_use_id":"task-call","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"child answer"}}}`,
		`{"type":"user","parent_tool_use_id":"task-call","message":{"content":[{"type":"tool_result","tool_use_id":"child-tool","content":"child tool output"}]}}`,
		`{"type":"assistant","parent_tool_use_id":"task-call","message":{"id":"child-message","content":[{"type":"text","text":"child answer"}]}}`,
		`{"type":"stream_event","event":{"type":"message_delta","delta":{"stop_reason":"tool_use"}}}`,
		`{"type":"stream_event","event":{"type":"message_stop"}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"task-call","content":"agent done","is_error":false}]}}`,
	} {
		mustRaw(t, p.raw(json.RawMessage(fixture)))
	}
	if got := countClaudeType(events, "session.step.started"); got != 1 {
		t.Fatalf("nested child opened a top-level step: types=%v", claudeTypes(events))
	}
	if got := countClaudeType(events, "session.text.delta"); got != 0 {
		t.Fatalf("nested child text entered parent text: types=%v", claudeTypes(events))
	}
	var success map[string]any
	claudeData(t, firstClaudeType(t, events, "session.tool.success"), &success)
	content := success["content"].([]any)
	if len(content) != 2 || content[1].(map[string]any)["type"] != "transcript" {
		t.Fatalf("nested transcript content = %#v", content)
	}
	transcript := content[1].(map[string]any)["events"].([]any)
	if len(transcript) != 5 {
		t.Fatalf("nested transcript event count = %d, want 5", len(transcript))
	}
}

func TestRawProjectorAcceptsNestedTranscriptAfterAsyncToolCompletes(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("session", "model", func(event gimble.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	for _, fixture := range []string{
		`{"type":"stream_event","event":{"type":"message_start","message":{"id":"parent-message","model":"model","usage":{}}}}`,
		`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"task-call","name":"Agent","input":{}}}}`,
		`{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"task-call","content":"launched","is_error":false}]}}`,
		`{"type":"stream_event","event":{"type":"message_delta","delta":{"stop_reason":"tool_use"}}}`,
		`{"type":"stream_event","event":{"type":"message_stop"}}`,
		`{"type":"stream_event","event":{"type":"message_start","message":{"id":"next-message","model":"model","usage":{}}}}`,
		`{"type":"assistant","parent_tool_use_id":"task-call","message":{"id":"child-message","content":[{"type":"text","text":"late child"}]}}`,
	} {
		mustRaw(t, p.raw(json.RawMessage(fixture)))
	}

	var progress map[string]any
	claudeData(t, firstClaudeType(t, events, "session.tool.progress"), &progress)
	if progress["assistantMessageID"] != "parent-message" {
		t.Fatalf("late child attached to message %#v, want parent-message", progress["assistantMessageID"])
	}
	var native map[string]any
	if err := json.Unmarshal(firstClaudeType(t, events, "session.tool.progress").NativeRef, &native); err != nil {
		t.Fatal(err)
	}
	if native["messageID"] != "parent-message" {
		t.Fatalf("late child NativeRef messageID = %#v, want parent-message", native["messageID"])
	}
}

func TestNestedProgressEmitsOneEntryPerEvent(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("session", "model", func(event gimble.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	mustRaw(t, p.raw(json.RawMessage(`{"type":"stream_event","event":{"type":"message_start","message":{"id":"parent-message","model":"model","usage":{}}}}`)))
	mustRaw(t, p.raw(json.RawMessage(`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"task-call","name":"Task","input":{}}}}`)))
	events = nil
	for i := range 1000 {
		mustRaw(t, p.nestedEvent("task-call", map[string]any{"type": "assistant", "sequence": i, "message": map[string]any{"content": "child"}}))
	}
	if len(events) != 1000 {
		t.Fatalf("nested progress event count = %d, want 1000", len(events))
	}

	total := 0
	for _, event := range events {
		total += len(event.Data) + len(event.NativeRef)
		var data struct {
			Metadata struct {
				Mode       string `json:"mode"`
				Transcript []any  `json:"transcript"`
			} `json:"metadata"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			t.Fatal(err)
		}
		if data.Metadata.Mode != "append" || len(data.Metadata.Transcript) != 1 {
			t.Fatalf("nested progress metadata = %#v, want one append entry", data.Metadata)
		}
	}
	if total > 2<<20 {
		t.Fatalf("1000 nested progress events emitted %d bytes, want linear payload under 2 MiB", total)
	}
}

func TestRawProjectorRecordsRetryFailureAndPermissionDecision(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("session", "model", func(event gimble.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	mustRaw(t, p.raw(json.RawMessage(`{"type":"stream_event","event":{"type":"message_start","message":{"id":"message","model":"model","usage":{}}}}`)))
	mustRaw(t, p.raw(json.RawMessage(`{"type":"system","subtype":"api_retry","attempt":2,"max_retries":5,"retry_delay_ms":400,"error_status":529,"error":"server_error","uuid":"retry","session_id":"session"}`)))
	mustRaw(t, p.raw(json.RawMessage(`{"type":"assistant","uuid":"failed","session_id":"session","request_id":"req_217","message":{"id":"message","content":[]},"error":"rate_limit"}`)))
	mustRaw(t, p.raw(json.RawMessage(`{"type":"system","subtype":"permission_denied","tool_use_id":"tool","tool_name":"Bash","uuid":"permission","session_id":"session"}`)))

	if got := claudeTypes(events); !slices.Equal(got, []string{"session.step.started", "session.retry.scheduled", "session.step.failed", "permission.asked", "permission.replied"}) {
		t.Fatalf("native event types = %v", got)
	}
	var retry, failed, asked, replied map[string]any
	claudeData(t, events[1], &retry)
	claudeData(t, events[2], &failed)
	claudeData(t, events[3], &asked)
	claudeData(t, events[4], &replied)
	if retry["attempt"] != float64(2) || retry["delayMS"] != float64(400) {
		t.Fatalf("retry = %#v", retry)
	}
	if failed["assistantMessageID"] != "message" || asked["id"] != "tool" || replied["reply"] != "denied" {
		t.Fatalf("failure/permission = failed %#v asked %#v replied %#v", failed, asked, replied)
	}
	errorValue := failed["error"].(map[string]any)
	if errorValue["requestID"] != "req_217" {
		t.Fatalf("failure did not retain provider request ID: %#v", errorValue)
	}
}

func firstClaudeType(t *testing.T, events []gimble.AgentEvent, eventType string) gimble.AgentEvent {
	t.Helper()
	for _, event := range events {
		if event.Type == eventType {
			return event
		}
	}
	t.Fatalf("event %s not found", eventType)
	return gimble.AgentEvent{}
}

func countClaudeType(events []gimble.AgentEvent, eventType string) int {
	count := 0
	for _, event := range events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}

func claudeTypes(events []gimble.AgentEvent) []string {
	var result []string
	for _, event := range events {
		if event.Type != "" {
			result = append(result, event.Type)
		}
	}
	return result
}

func claudeData(t *testing.T, event gimble.AgentEvent, target any) {
	t.Helper()
	if err := json.Unmarshal(event.Data, target); err != nil {
		t.Fatal(err)
	}
}

func mustRaw(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestRawProjectorKeepsTheTurnWhenToolInputIsNotJSON(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("session-1", "claude-test", func(event gimble.AgentEvent) error { events = append(events, event); return nil })
	fixtures := []string{
		`{"type":"stream_event","uuid":"e","session_id":"session-1","event":{"type":"message_start","message":{"id":"msg_native","model":"claude-native","usage":{"input_tokens":1}}}}`,
		`{"type":"stream_event","uuid":"e","session_id":"session-1","event":{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_bad","name":"Read","input":{}}}}`,
		`{"type":"stream_event","uuid":"e","session_id":"session-1","event":{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"offset\": 230,"}}}`,
		`{"type":"stream_event","uuid":"e","session_id":"session-1","event":{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":" 245}"}}}`,
		`{"type":"stream_event","uuid":"e","session_id":"session-1","event":{"type":"content_block_stop","index":0}}`,
		`{"type":"stream_event","uuid":"e","session_id":"session-1","event":{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":3}}}`,
		`{"type":"stream_event","uuid":"e","session_id":"session-1","event":{"type":"message_stop"}}`,
		`{"type":"user","uuid":"u","session_id":"session-1","message":{"id":"user-message","content":[{"type":"tool_result","tool_use_id":"toolu_bad","content":"InputValidationError: Read was called with input that could not be parsed as JSON.","is_error":true}]}}`,
	}
	for _, fixture := range fixtures {
		mustRaw(t, p.raw(json.RawMessage(fixture)))
	}
	got := claudeTypes(events)
	if !slices.Contains(got, "session.tool.called") || !slices.Contains(got, "session.tool.failed") || got[len(got)-1] != "session.step.ended" {
		t.Fatalf("event types = %v", got)
	}
	var called map[string]any
	claudeData(t, events[slices.Index(got, "session.tool.called")], &called)
	unparsed, _ := called["input"].(map[string]any)["__unparsedToolInput"].(map[string]any)
	if unparsed["raw"] != `{"offset": 230, 245}` {
		t.Fatalf("input = %#v", called["input"])
	}
}

// TestRawProjectorRecordsTheCLIsExplanationOfAFailedTurn: the reason a turn
// failed is the text of the synthetic assistant message that carries the
// error code. It belongs on the failure record, not only in Claude Code's
// own transcript.
func TestRawProjectorRecordsTheCLIsExplanationOfAFailedTurn(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("session-1", "claude-test", func(event gimble.AgentEvent) error { events = append(events, event); return nil })
	fixtures := []string{
		`{"type":"stream_event","uuid":"e","session_id":"session-1","event":{"type":"message_start","message":{"id":"msg","model":"claude-test"}}}`,
		`{"type":"assistant","uuid":"a","session_id":"session-1","request_id":"req_217","error":"invalid_request","message":{"id":"msg","role":"assistant","content":[{"type":"text","text":"Autocompact is thrashing: the context refilled\nto the limit within 3 turns of the previous compact."}]}}`,
	}
	for _, fixture := range fixtures {
		mustRaw(t, p.raw(json.RawMessage(fixture)))
	}
	var failed map[string]any
	claudeData(t, firstClaudeType(t, events, "session.step.failed"), &failed)
	errorValue := failed["error"].(map[string]any)
	want := "Autocompact is thrashing: the context refilled to the limit within 3 turns of the previous compact."
	if errorValue["harnessMessage"] != want {
		t.Fatalf("harnessMessage = %#v, want %q", errorValue["harnessMessage"], want)
	}
	if message, _ := errorValue["message"].(string); !strings.Contains(message, "invalid_request") || !strings.Contains(message, want) {
		t.Fatalf("message = %q, want the code and the explanation", message)
	}
	if errorValue["requestID"] != "req_217" {
		t.Fatalf("requestID = %#v", errorValue["requestID"])
	}
}
