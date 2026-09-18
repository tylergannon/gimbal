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

func TestProjectorBindsRawResponseAndNormalizesTokens(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("thread-1", "turn-1", "gpt-test", func(event gimble.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	mustProject(t, p.itemStarted(json.RawMessage(`{"item":{"id":"msg-item","type":"agentMessage"}}`)))
	mustProject(t, p.textDelta(json.RawMessage(`{"delta":"hel"}`)))
	_, _, err := p.itemCompleted(json.RawMessage(`{"item":{"id":"msg-item","type":"agentMessage","text":"hello"}}`))
	mustProject(t, err)
	mustProject(t, p.rawResponseCompleted(json.RawMessage(`{"responseId":"resp_123","usage":{"inputTokens":100,"cachedInputTokens":25,"cacheWriteInputTokens":5,"outputTokens":30,"reasoningOutputTokens":10}}`)))
	mustProject(t, p.turnCompleted(json.RawMessage(`{"turn":{"status":"completed"}}`)))
	if !slices.Contains(types(events), "session.step.ended") {
		t.Fatal("completed turn did not end the tool-free step")
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
	if !jsonContains(streamedEvent.NativeRef, `"responseID":"resp_123"`) {
		t.Fatalf("response boundary not bound: streamed=%s", streamedEvent.NativeRef)
	}
	if ended["cost"] != float64(0) {
		t.Fatalf("step cost = %#v, want 0: Codex states no cost", ended["cost"])
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

func TestProjectorAttributesToolDeliveredAfterRawResponse(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("thread", "turn", "model", func(event gimble.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	mustProject(t, p.itemStarted(json.RawMessage(`{"item":{"id":"reasoning","type":"reasoning"}}`)))
	_, _, err := p.itemCompleted(json.RawMessage(`{"item":{"id":"reasoning","type":"reasoning","summary":["calling the tool"]}}`))
	mustProject(t, err)
	mustProject(t, p.rawResponseItemCompleted(json.RawMessage(`{"item":{"type":"function_call","call_id":"call"}}`)))
	mustProject(t, p.rawResponseCompleted(json.RawMessage(`{"responseId":"resp_tool"}`)))
	mustProject(t, p.itemStarted(json.RawMessage(`{"item":{"id":"call","type":"commandExecution","command":"printf proof","cwd":"/w"}}`)))
	_, _, err = p.itemCompleted(json.RawMessage(`{"item":{"id":"call","type":"commandExecution","status":"completed","aggregatedOutput":"proof"}}`))
	mustProject(t, err)
	mustProject(t, p.rawResponseItemCompleted(json.RawMessage(`{"item":{"type":"function_call_output","call_id":"call","output":"proof"}}`)))

	if got := countType(events, "session.step.started"); got != 1 {
		t.Fatalf("step.started count = %d, want delayed tool in the response's existing step; types=%v", got, types(events))
	}
	var started, tool map[string]any
	decodeData(t, firstType(t, events, "session.step.started"), &started)
	decodeData(t, firstType(t, events, "session.tool.called"), &tool)
	if tool["assistantMessageID"] != started["assistantMessageID"] {
		t.Fatalf("tool message = %v, response message = %v", tool["assistantMessageID"], started["assistantMessageID"])
	}
	if got := types(events); got[len(got)-2] != "session.tool.success" || got[len(got)-1] != "session.step.ended" {
		t.Fatalf("terminal event types = %v", got)
	}
}

func TestProjectorAttributesSerialToolsFromRawResponseItems(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("thread", "turn", "model", func(event gimble.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	mustProject(t, p.itemStarted(json.RawMessage(`{"item":{"id":"reasoning","type":"reasoning"}}`)))
	_, _, err := p.itemCompleted(json.RawMessage(`{"item":{"id":"reasoning","type":"reasoning","summary":["two calls"]}}`))
	mustProject(t, err)
	mustProject(t, p.rawResponseItemCompleted(json.RawMessage(`{"item":{"type":"local_shell_call","call_id":"call-a"}}`)))
	mustProject(t, p.rawResponseItemCompleted(json.RawMessage(`{"item":{"type":"function_call","call_id":"call-b"}}`)))
	mustProject(t, p.rawResponseCompleted(json.RawMessage(`{"responseId":"resp_tools"}`)))
	mustProject(t, p.itemStarted(json.RawMessage(`{"item":{"id":"call-a","type":"commandExecution","command":"printf a","cwd":"/w"}}`)))
	_, _, err = p.itemCompleted(json.RawMessage(`{"item":{"id":"call-a","type":"commandExecution","status":"completed","aggregatedOutput":"a"}}`))
	mustProject(t, err)
	mustProject(t, p.rawResponseItemCompleted(json.RawMessage(`{"item":{"type":"function_call_output","call_id":"call-a","output":"a"}}`)))
	if got := countType(events, "session.step.ended"); got != 0 {
		t.Fatalf("first serial tool ended response with another owned tool pending; types=%v", types(events))
	}
	mustProject(t, p.itemStarted(json.RawMessage(`{"item":{"id":"call-b","type":"fileChange","changes":[]}}`)))
	_, _, err = p.itemCompleted(json.RawMessage(`{"item":{"id":"call-b","type":"fileChange","status":"completed","changes":[]}}`))
	mustProject(t, err)
	mustProject(t, p.rawResponseItemCompleted(json.RawMessage(`{"item":{"type":"function_call_output","call_id":"call-b","output":"done"}}`)))

	var toolMessages []string
	for _, event := range events {
		if event.Type != "session.tool.called" {
			continue
		}
		var data map[string]any
		decodeData(t, event, &data)
		toolMessages = append(toolMessages, data["assistantMessageID"].(string))
	}
	if len(toolMessages) != 2 || toolMessages[0] != toolMessages[1] {
		t.Fatalf("serial tool messages = %v, want one raw response step", toolMessages)
	}
	if got := countType(events, "session.step.started"); got != 1 {
		t.Fatalf("step.started count = %d, want 1; types=%v", got, types(events))
	}
	if got := countType(events, "session.step.ended"); got != 1 {
		t.Fatalf("step.ended count = %d, want 1; types=%v", got, types(events))
	}
}

func TestProjectorSeparatesToolFirstNextResponse(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("thread", "turn", "model", func(event gimble.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	mustProject(t, p.rawResponseItemCompleted(json.RawMessage(`{"item":{"type":"function_call","call_id":"call-a"}}`)))
	mustProject(t, p.itemStarted(json.RawMessage(`{"item":{"id":"call-a","type":"commandExecution","command":"printf a","cwd":"/w"}}`)))
	_, _, err := p.itemCompleted(json.RawMessage(`{"item":{"id":"call-a","type":"commandExecution","status":"completed","aggregatedOutput":"a"}}`))
	mustProject(t, err)
	mustProject(t, p.rawResponseCompleted(json.RawMessage(`{"responseId":"resp_one"}`)))
	mustProject(t, p.rawResponseItemCompleted(json.RawMessage(`{"item":{"type":"function_call_output","call_id":"call-a","output":"a"}}`)))
	mustProject(t, p.rawResponseItemCompleted(json.RawMessage(`{"item":{"type":"function_call","call_id":"call-b"}}`)))
	mustProject(t, p.itemStarted(json.RawMessage(`{"item":{"id":"call-b","type":"commandExecution","command":"printf b","cwd":"/w"}}`)))
	_, _, err = p.itemCompleted(json.RawMessage(`{"item":{"id":"call-b","type":"commandExecution","status":"completed","aggregatedOutput":"b"}}`))
	mustProject(t, err)
	mustProject(t, p.rawResponseCompleted(json.RawMessage(`{"responseId":"resp_two"}`)))
	mustProject(t, p.rawResponseItemCompleted(json.RawMessage(`{"item":{"type":"function_call_output","call_id":"call-b","output":"b"}}`)))

	var toolMessages []string
	for _, event := range events {
		if event.Type != "session.tool.called" {
			continue
		}
		var data map[string]any
		decodeData(t, event, &data)
		toolMessages = append(toolMessages, data["assistantMessageID"].(string))
	}
	if len(toolMessages) != 2 || toolMessages[0] == toolMessages[1] {
		t.Fatalf("tool-first response messages = %v, want distinct raw response steps", toolMessages)
	}
	if got := countType(events, "session.step.started"); got != 2 {
		t.Fatalf("step.started count = %d, want 2; types=%v", got, types(events))
	}
	if got := countType(events, "session.step.ended"); got != 2 {
		t.Fatalf("step.ended count = %d, want 2; types=%v", got, types(events))
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
	mustProject(t, p.turnCompleted(json.RawMessage(`{"turn":{"status":"completed"}}`)))
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

func TestProjectorCompletesResumedTurnWithoutRawResponse(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("thread", "turn", "model", func(event gimble.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	mustProject(t, p.itemStarted(json.RawMessage(`{"item":{"id":"message","type":"agentMessage"}}`)))
	_, _, err := p.itemCompleted(json.RawMessage(`{"item":{"id":"message","type":"agentMessage","text":"done"}}`))
	mustProject(t, err)
	mustProject(t, p.turnCompleted(json.RawMessage(`{"turn":{"status":"completed"}}`)))

	if got := countType(events, "session.step.ended"); got != 1 {
		t.Fatalf("step.ended count = %d, want 1; types=%v", got, types(events))
	}
}

// The sample is thread/tokenUsage/updated verbatim from
// ephemeral/research/issue-149/codex/turn2-sameproc.jsonl, where total is the
// process's running sum and last is that turn's one model call.
func TestProjectorFillsStepFromRecordedTokenUsage(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("01a0988c-e92a-7be3-a0dd-98e708d1b6e5", "01a0988d-1c26-78b2-a0fc-5e97d2295747", "gpt-5.6-luna", func(event gimble.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	mustProject(t, p.itemStarted(json.RawMessage(`{"item":{"id":"message","type":"agentMessage"}}`)))
	_, _, err := p.itemCompleted(json.RawMessage(`{"item":{"id":"message","type":"agentMessage","text":"done"}}`))
	mustProject(t, err)
	mustProject(t, p.tokenUsageUpdated(json.RawMessage(`{"threadId":"01a0988c-e92a-7be3-a0dd-98e708d1b6e5","turnId":"01a0988d-1c26-78b2-a0fc-5e97d2295747","tokenUsage":{"total":{"totalTokens":40531,"inputTokens":40508,"cachedInputTokens":29184,"cacheWriteInputTokens":0,"outputTokens":23,"reasoningOutputTokens":11},"last":{"totalTokens":20285,"inputTokens":20267,"cachedInputTokens":19200,"cacheWriteInputTokens":0,"outputTokens":18,"reasoningOutputTokens":11},"modelContextWindow":258400}}`)))
	mustProject(t, p.turnCompleted(json.RawMessage(`{"turn":{"status":"completed"}}`)))

	var ended map[string]any
	decodeData(t, firstType(t, events, "session.step.ended"), &ended)
	tokens := ended["tokens"].(map[string]any)
	cache := tokens["cache"].(map[string]any)
	if tokens["input"] != float64(1067) || tokens["output"] != float64(7) || tokens["reasoning"] != float64(11) || cache["read"] != float64(19200) || cache["write"] != float64(0) {
		t.Fatalf("tokens from tokenUsage.last = %#v", tokens)
	}
}

func TestProjectorPropagatesCallbackError(t *testing.T) {
	boom := errors.New("observer stopped")
	p := newProjector("thread", "turn", "model", func(gimble.AgentEvent) error { return boom })
	if err := p.itemStarted(json.RawMessage(`{"item":{"id":"one","type":"agentMessage"}}`)); !errors.Is(err, boom) {
		t.Fatalf("callback error = %v, want %v", err, boom)
	}
}

func TestProjectorRecordsNativeRetryAndTerminalError(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("thread", "turn", "model", func(event gimble.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	retrying, err := p.harnessError(json.RawMessage(`{"threadId":"thread","turnId":"turn","error":{"message":"stream disconnected","codexErrorInfo":"responseStreamDisconnected"},"willRetry":true}`))
	mustProject(t, err)
	if !retrying {
		t.Fatal("willRetry notification reported terminal")
	}
	if got := types(events); !slices.Equal(got, []string{"session.step.started", "session.retry.scheduled"}) {
		t.Fatalf("retry event types = %v", got)
	}
	var retry map[string]any
	decodeData(t, firstType(t, events, "session.retry.scheduled"), &retry)
	if _, exists := retry["attempt"]; exists {
		t.Fatalf("retry invented an attempt: %#v", retry)
	}
	retrying, err = p.harnessError(json.RawMessage(`{"threadId":"thread","turnId":"turn","error":{"message":"upstream failed","additionalDetails":"request exhausted"},"willRetry":false}`))
	mustProject(t, err)
	if retrying {
		t.Fatal("terminal notification reported retrying")
	}
	if got := types(events); got[len(got)-1] != "session.step.failed" {
		t.Fatalf("terminal event types = %v", got)
	}
	var failed map[string]any
	decodeData(t, firstType(t, events, "session.step.failed"), &failed)
	if failed["assistantMessageID"] != retry["assistantMessageID"] {
		t.Fatalf("terminal error changed step identity: retry=%#v failed=%#v", retry, failed)
	}
}

func TestProjectorRecordsApprovalRequestAndFixedDecision(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("thread", "turn", "model", func(event gimble.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	params := json.RawMessage(`{"threadId":"thread","turnId":"turn","itemId":"call","command":"rm one","reason":"outside sandbox","startedAtMs":1}`)
	id, err := p.approvalRequested("item/commandExecution/requestApproval", params)
	mustProject(t, err)
	if id != "call" {
		t.Fatalf("approval id = %q, want call", id)
	}
	mustProject(t, p.approvalReplied(id, "denied", "item/commandExecution/requestApproval", params))
	if got := types(events); !slices.Equal(got, []string{"permission.asked", "permission.replied"}) {
		t.Fatalf("approval event types = %v", got)
	}
	var asked, replied map[string]any
	decodeData(t, events[0], &asked)
	decodeData(t, events[1], &replied)
	if asked["permission"] != "item/commandExecution/requestApproval" || replied["reply"] != "denied" {
		t.Fatalf("approval projection = asked %#v replied %#v", asked, replied)
	}
}

func TestProjectorKeepsNativeChildTranscriptInsideCollabTool(t *testing.T) {
	var events []gimble.AgentEvent
	parent := newProjector("parent", "parent-turn", "model", func(event gimble.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	started := json.RawMessage(`{"threadId":"parent","turnId":"parent-turn","item":{"id":"spawn","type":"collabAgentToolCall","tool":"spawn_agent","receiverThreadIds":["child"],"senderThreadId":"parent","agentsStates":{},"status":"inProgress"}}`)
	mustProject(t, parent.itemStarted(started))
	tool, children := codexChildThreads(started)
	if tool != "spawn" || !slices.Equal(children, []string{"child"}) {
		t.Fatalf("child routing = tool %q children %v", tool, children)
	}
	child := newProjector("child", "child-turn", "model", func(event gimble.AgentEvent) error {
		return parent.nestedEvent(tool, event)
	})
	mustProject(t, child.itemStarted(json.RawMessage(`{"threadId":"child","turnId":"child-turn","item":{"id":"child-message","type":"agentMessage"}}`)))
	mustProject(t, child.textDelta(json.RawMessage(`{"threadId":"child","turnId":"child-turn","itemId":"child-message","delta":"child answer"}`)))
	_, _, err := child.itemCompleted(json.RawMessage(`{"threadId":"child","turnId":"child-turn","item":{"id":"child-message","type":"agentMessage","text":"child answer"}}`))
	mustProject(t, err)
	_, _, err = parent.itemCompleted(json.RawMessage(`{"threadId":"parent","turnId":"parent-turn","item":{"id":"spawn","type":"collabAgentToolCall","tool":"spawn_agent","receiverThreadIds":["child"],"senderThreadId":"parent","agentsStates":{"child":{"status":"completed"}},"status":"completed"}}`))
	mustProject(t, err)

	if got := countType(events, "session.step.started"); got != 1 {
		t.Fatalf("child opened a top-level parent step: types=%v", types(events))
	}
	if got := countType(events, "session.text.delta"); got != 0 {
		t.Fatalf("child text entered parent text: types=%v", types(events))
	}
	var success map[string]any
	decodeData(t, firstType(t, events, "session.tool.success"), &success)
	content := success["content"].([]any)
	if len(content) != 2 || content[1].(map[string]any)["type"] != "transcript" {
		t.Fatalf("collab tool content = %#v", content)
	}
	transcript := content[1].(map[string]any)["events"].([]any)
	if len(transcript) != 4 {
		t.Fatalf("nested event count = %d, want 4", len(transcript))
	}
}

func TestProjectorAcceptsChildEventsAfterCollabToolCompletes(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("parent", "turn", "model", func(event gimble.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	started := json.RawMessage(`{"item":{"id":"spawn","type":"collabAgentToolCall","tool":"spawnAgent","receiverThreadIds":[],"status":"inProgress"}}`)
	mustProject(t, p.itemStarted(started))
	mustProject(t, p.rawResponseCompleted(json.RawMessage(`{"responseId":"response","usage":{}}`)))
	_, _, err := p.itemCompleted(json.RawMessage(`{"item":{"id":"spawn","type":"collabAgentToolCall","tool":"spawnAgent","receiverThreadIds":["child"],"status":"completed"}}`))
	mustProject(t, err)
	mustProject(t, p.nestedEvent("spawn", gimble.AgentEvent{Type: "session.text.delta", Data: json.RawMessage(`{"delta":"late child"}`)}))

	var progress map[string]any
	decodeData(t, firstType(t, events, "session.tool.progress"), &progress)
	if progress["assistantMessageID"] != "spawn" {
		t.Fatalf("late child attached to message %#v, want spawn", progress["assistantMessageID"])
	}
}

func TestNestedProgressEmitsOneEntryPerEvent(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("parent", "turn", "model", func(event gimble.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	mustProject(t, p.itemStarted(json.RawMessage(`{"item":{"id":"spawn","type":"collabAgentToolCall","tool":"spawnAgent","receiverThreadIds":[],"status":"inProgress"}}`)))
	events = nil
	for i := range 1000 {
		mustProject(t, p.nestedEvent("spawn", gimble.AgentEvent{Type: "session.text.delta", Data: json.RawMessage(fmt.Sprintf(`{"sequence":%d,"delta":"child"}`, i))}))
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

func TestChildThreadRemainsOwnedByItsSpawnTool(t *testing.T) {
	conn := &connection{threads: make(map[string]chan rpcMessage)}
	ch := make(chan rpcMessage)
	parents := make(map[string]string)
	registerCodexChildren(conn, ch, json.RawMessage(`{"item":{"id":"spawn","type":"collabAgentToolCall","receiverThreadIds":["child"]}}`), parents)
	registerCodexChildren(conn, ch, json.RawMessage(`{"item":{"id":"wait","type":"collabAgentToolCall","receiverThreadIds":["child"]}}`), parents)
	if parents["child"] != "spawn" {
		t.Fatalf("child parent = %q, want spawn", parents["child"])
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

func TestProjectorDropsOutputForAToolTheStepNoLongerHolds(t *testing.T) {
	var events []gimble.AgentEvent
	p := newProjector("thread-1", "turn-1", "gpt-test", func(event gimble.AgentEvent) error { events = append(events, event); return nil })
	mustProject(t, p.itemStarted(json.RawMessage(`{"item":{"id":"call-1","type":"commandExecution","command":"go test ./...","cwd":"/w"}}`)))
	_, _, err := p.itemCompleted(json.RawMessage(`{"item":{"id":"call-1","type":"commandExecution","status":"completed","aggregatedOutput":"ok"}}`))
	mustProject(t, err)
	before := len(events)
	// Output that arrives after the command completed, and output for a
	// command this step never saw, are dropped rather than ending the turn.
	mustProject(t, p.toolOutputDelta(json.RawMessage(`{"itemId":"call-1","delta":"ok\n"}`)))
	mustProject(t, p.toolOutputDelta(json.RawMessage(`{"itemId":"call-from-an-ended-step","delta":"ok\n"}`)))
	if got := types(events[before:]); len(got) != 0 {
		t.Fatalf("late deltas produced events: %v", got)
	}
}
