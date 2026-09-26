package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/wire"
)

func testUsage() model.Usage { return model.Usage{} }

func testModel() *model.Model {
	return &model.Model{
		ID: "mock", Name: "mock", Api: model.APIOpenAIResponses, Provider: model.ProviderOpenAI,
		BaseURL: "https://example.invalid", Reasoning: false, Input: []string{"text"},
		ContextWindow: 8192, MaxTokens: 2048,
	}
}

func testAssistant(content model.ContentList, stop model.StopReason) *model.AssistantMessage {
	return &model.AssistantMessage{
		Content:    content,
		Api:        model.APIOpenAIResponses,
		Provider:   model.ProviderOpenAI,
		Model:      "mock",
		Usage:      testUsage(),
		StopReason: stop,
		Timestamp:  timeNowMillis(),
	}
}

func testAssistantText(text string) *model.AssistantMessage {
	return testAssistant(model.ContentList{model.TextContent{Text: text}}, model.StopStop)
}

func testUser(text string) model.AgentMessage {
	return model.NewUserText(text, timeNowMillis())
}

func testTool(name string) model.AgentTool {
	return model.AgentTool{
		Name:        name,
		Label:       name,
		Description: name + " tool",
		Parameters:  model.Object(),
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate model.ToolUpdateFunc) (model.AgentToolResult, error) {
			return model.AgentToolResult{Content: model.ContentList{model.TextContent{Text: name}}, Details: map[string]any{}}, nil
		},
	}
}

func identityConverter(messages []model.AgentMessage) []model.Message {
	return defaultConvertToLlm(messages)
}

func doneStream(message *model.AssistantMessage) StreamFunction {
	return func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		stream := wire.NewAssistantMessageEventStream()
		stream.Push(model.AssistantMessageEvent{Type: model.EventDone, Reason: message.StopReason, Message: message})
		return stream, nil
	}
}

// sequenceStream returns the next message from the queue on each call.
func sequenceStream(messages ...*model.AssistantMessage) StreamFunction {
	var mu sync.Mutex
	index := 0
	return func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		mu.Lock()
		var message *model.AssistantMessage
		if index < len(messages) {
			message = messages[index]
		}
		index++
		mu.Unlock()
		stream := wire.NewAssistantMessageEventStream()
		if message == nil {
			message = testAssistantText("done")
		}
		stream.Push(model.AssistantMessageEvent{Type: model.EventDone, Reason: message.StopReason, Message: message})
		return stream, nil
	}
}

type eventCollector struct {
	mu     sync.Mutex
	events []model.AgentEvent
}

func (c *eventCollector) sink() model.EventSink {
	return func(event model.AgentEvent) error {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.events = append(c.events, event)
		return nil
	}
}

func (c *eventCollector) snapshot() []model.AgentEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]model.AgentEvent(nil), c.events...)
}

func containsEventType(events []model.AgentEvent, want model.AgentEventType) bool {
	for _, event := range events {
		if event.Type == want {
			return true
		}
	}
	return false
}

func TestRunAgentLoopEmitsLifecycleAndMessages(t *testing.T) {
	collector := &eventCollector{}
	config := model.AgentLoopConfig{Model: testModel(), ConvertToLlm: identityConverter}
	messages, err := RunAgentLoop(context.Background(), []model.AgentMessage{testUser("Hello")},
		model.AgentContext{Messages: []model.AgentMessage{}, Tools: []model.AgentTool{}},
		config, collector.sink(), doneStream(testAssistantText("Hi there!")))
	if err != nil {
		t.Fatalf("RunAgentLoop: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(messages))
	}
	if messages[0].MessageRole() != model.RoleUser || messages[1].MessageRole() != model.RoleAssistant {
		t.Fatalf("roles = %s, %s", messages[0].MessageRole(), messages[1].MessageRole())
	}
	for _, want := range []model.AgentEventType{
		model.EvAgentStart, model.EvTurnStart, model.EvMessageStart, model.EvMessageEnd, model.EvTurnEnd, model.EvAgentEnd,
	} {
		if !containsEventType(collector.snapshot(), want) {
			t.Errorf("missing event %s", want)
		}
	}
}

func TestRunAgentLoopProviderContextIsTranscriptOnly(t *testing.T) {
	initialSystem := model.NewSystemText("Transcript prompt", 1)
	agentCtx := model.AgentContext{Messages: []model.AgentMessage{}, Tools: []model.AgentTool{}}
	config := model.AgentLoopConfig{Model: testModel(), ConvertToLlm: identityConverter}

	var seen model.TranscriptContext
	streamFn := func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		seen = transcript
		response := wire.NewAssistantMessageEventStream()
		response.Push(model.AssistantMessageEvent{Type: model.EventDone, Reason: model.StopStop, Message: testAssistantText("done")})
		return response, nil
	}
	if _, err := RunAgentLoop(context.Background(), []model.AgentMessage{initialSystem, testUser("Hello")}, agentCtx, config, (&eventCollector{}).sink(), streamFn); err != nil {
		t.Fatalf("RunAgentLoop: %v", err)
	}
	if len(seen.Messages) != 2 {
		t.Fatalf("provider messages = %d, want 2", len(seen.Messages))
	}
	if seen.Messages[0].MessageRole() != model.RoleSystem {
		t.Fatalf("first provider message role = %s, want system", seen.Messages[0].MessageRole())
	}
}

func TestRunAgentLoopAppliesTransformContextBeforeConvert(t *testing.T) {
	agentCtx := model.AgentContext{Messages: []model.AgentMessage{
		testUser("old 1"), testAssistantText("old 1"), testUser("old 2"), testAssistantText("old 2"),
	}}
	var transformed, converted int
	config := model.AgentLoopConfig{
		Model: testModel(),
		TransformContext: func(ctx context.Context, messages []model.AgentMessage) []model.AgentMessage {
			pruned := messages[len(messages)-2:]
			transformed = len(pruned)
			return pruned
		},
		ConvertToLlm: func(messages []model.AgentMessage) []model.Message {
			converted = len(messages)
			return identityConverter(messages)
		},
	}
	if _, err := RunAgentLoop(context.Background(), []model.AgentMessage{testUser("new")}, agentCtx, config, (&eventCollector{}).sink(), doneStream(testAssistantText("ok"))); err != nil {
		t.Fatalf("RunAgentLoop: %v", err)
	}
	if transformed != 2 || converted != 2 {
		t.Fatalf("transformed=%d converted=%d, want 2/2", transformed, converted)
	}
}

func TestRunAgentLoopExecutesToolCallAndPatchesUsage(t *testing.T) {
	toolUsage := &model.Usage{Input: 1, Output: 2}
	patchedUsage := &model.Usage{Input: 5, Output: 6}
	var executed []string
	tool := model.AgentTool{
		Name: "echo", Description: "Echo", Parameters: model.Object(model.Prop("value", model.String())),
		Execute: func(ctx context.Context, id string, params map[string]any, onUpdate model.ToolUpdateFunc) (model.AgentToolResult, error) {
			executed = append(executed, params["value"].(string))
			return model.AgentToolResult{
				Content: model.ContentList{model.TextContent{Text: "echoed: " + params["value"].(string)}},
				Details: map[string]any{"value": params["value"]},
				Usage:   toolUsage,
			}, nil
		},
	}
	agentCtx := model.AgentContext{Messages: []model.AgentMessage{}, Tools: []model.AgentTool{tool}}
	var observed *model.Usage
	config := model.AgentLoopConfig{
		Model:        testModel(),
		ConvertToLlm: identityConverter,
		AfterToolCall: func(ctx context.Context, c model.AfterToolCallContext) *model.AfterToolCallResult {
			observed = c.Result.Usage
			return &model.AfterToolCallResult{Usage: patchedUsage}
		},
	}
	streamFn := sequenceStream(
		testAssistant(model.ContentList{model.ToolCall{ID: "tool-1", Name: "echo", Arguments: map[string]any{"value": "hello"}}}, model.StopToolUse),
		testAssistantText("done"),
	)
	messages, err := RunAgentLoop(context.Background(), []model.AgentMessage{testUser("echo")}, agentCtx, config, (&eventCollector{}).sink(), streamFn)
	if err != nil {
		t.Fatalf("RunAgentLoop: %v", err)
	}
	if len(executed) != 1 || executed[0] != "hello" {
		t.Fatalf("executed = %v", executed)
	}
	if observed != toolUsage {
		t.Fatalf("afterToolCall observed %v, want %v", observed, toolUsage)
	}
	var result *model.ToolResultMessage
	for _, message := range messages {
		if tr, ok := message.(model.ToolResultMessage); ok {
			copy := tr
			result = &copy
		}
	}
	if result == nil || result.Usage == nil || *result.Usage != *patchedUsage {
		t.Fatalf("tool result usage = %v, want %v", result, patchedUsage)
	}
}

func TestRunAgentLoopDoesNotExecuteTruncatedToolCalls(t *testing.T) {
	tool := testTool("echo")
	var executed int
	tool.Execute = func(ctx context.Context, id string, params map[string]any, onUpdate model.ToolUpdateFunc) (model.AgentToolResult, error) {
		executed++
		return model.AgentToolResult{}, nil
	}
	agentCtx := model.AgentContext{Tools: []model.AgentTool{tool}}
	config := model.AgentLoopConfig{Model: testModel(), ConvertToLlm: identityConverter}
	collector := &eventCollector{}
	streamFn := sequenceStream(
		testAssistant(model.ContentList{model.ToolCall{ID: "tool-1", Name: "echo", Arguments: map[string]any{"value": "hel"}}}, model.StopLength),
		testAssistantText("done"),
	)
	if _, err := RunAgentLoop(context.Background(), []model.AgentMessage{testUser("echo")}, agentCtx, config, collector.sink(), streamFn); err != nil {
		t.Fatalf("RunAgentLoop: %v", err)
	}
	if executed != 0 {
		t.Fatalf("executed = %d, want 0", executed)
	}
	var end *model.AgentEvent
	for i := range collector.events {
		if collector.events[i].Type == model.EvToolExecutionEnd {
			end = &collector.events[i]
		}
	}
	if end == nil || !end.IsError {
		t.Fatalf("tool_execution_end = %+v, want error", end)
	}
	result, ok := end.Result.(model.AgentToolResult)
	if !ok || len(result.Content) == 0 {
		t.Fatalf("tool result = %#v", end.Result)
	}
}

func TestRunAgentLoopUsesMutatedBeforeToolCallArgs(t *testing.T) {
	tool := testTool("echo")
	var executed []any
	tool.Parameters = model.Object(model.Prop("value", model.String()))
	tool.Execute = func(ctx context.Context, id string, params map[string]any, onUpdate model.ToolUpdateFunc) (model.AgentToolResult, error) {
		executed = append(executed, params["value"])
		return model.AgentToolResult{Content: model.ContentList{model.TextContent{Text: "ok"}}}, nil
	}
	config := model.AgentLoopConfig{
		Model:        testModel(),
		ConvertToLlm: identityConverter,
		BeforeToolCall: func(ctx context.Context, c model.BeforeToolCallContext) *model.BeforeToolCallResult {
			c.Args["value"] = 123
			return nil
		},
	}
	streamFn := sequenceStream(
		testAssistant(model.ContentList{model.ToolCall{ID: "tool-1", Name: "echo", Arguments: map[string]any{"value": "hello"}}}, model.StopToolUse),
		testAssistantText("done"),
	)
	if _, err := RunAgentLoop(context.Background(), []model.AgentMessage{testUser("echo")}, model.AgentContext{Tools: []model.AgentTool{tool}}, config, (&eventCollector{}).sink(), streamFn); err != nil {
		t.Fatalf("RunAgentLoop: %v", err)
	}
	if len(executed) != 1 || executed[0] != 123 {
		t.Fatalf("executed = %#v, want [123]", executed)
	}
}

func TestRunAgentLoopPreparesArgumentsBeforeValidation(t *testing.T) {
	edit := model.Object(model.Prop("edits", model.Array(model.Object(model.Prop("oldText", model.String()), model.Prop("newText", model.String())))))
	var executed [][]any
	tool := model.AgentTool{
		Name: "edit", Description: "Edit", Parameters: edit,
		PrepareArguments: func(raw map[string]any) map[string]any {
			oldText, _ := raw["oldText"].(string)
			newText, _ := raw["newText"].(string)
			if oldText == "" || newText == "" {
				return raw
			}
			edits, _ := raw["edits"].([]any)
			edits = append(edits, map[string]any{"oldText": oldText, "newText": newText})
			return map[string]any{"edits": edits}
		},
		Execute: func(ctx context.Context, id string, params map[string]any, onUpdate model.ToolUpdateFunc) (model.AgentToolResult, error) {
			edits, _ := params["edits"].([]any)
			executed = append(executed, edits)
			return model.AgentToolResult{Content: model.ContentList{model.TextContent{Text: "ok"}}}, nil
		},
	}
	config := model.AgentLoopConfig{Model: testModel(), ConvertToLlm: identityConverter}
	streamFn := sequenceStream(
		testAssistant(model.ContentList{model.ToolCall{ID: "tool-1", Name: "edit", Arguments: map[string]any{"oldText": "before", "newText": "after"}}}, model.StopToolUse),
		testAssistantText("done"),
	)
	if _, err := RunAgentLoop(context.Background(), []model.AgentMessage{testUser("edit")}, model.AgentContext{Tools: []model.AgentTool{tool}}, config, (&eventCollector{}).sink(), streamFn); err != nil {
		t.Fatalf("RunAgentLoop: %v", err)
	}
	if len(executed) != 1 || len(executed[0]) != 1 {
		t.Fatalf("executed = %#v", executed)
	}
}

func TestRunAgentLoopParallelCompletionOrderAndSourceOrderResults(t *testing.T) {
	var parallelObserved bool
	tool := model.AgentTool{
		Name: "echo", Description: "Echo", Parameters: model.Object(model.Prop("value", model.String())),
		Execute: func(ctx context.Context, id string, params map[string]any, onUpdate model.ToolUpdateFunc) (model.AgentToolResult, error) {
			value := params["value"].(string)
			if value == "first" {
				time.Sleep(50 * time.Millisecond)
			}
			if value == "second" {
				parallelObserved = true
			}
			return model.AgentToolResult{Content: model.ContentList{model.TextContent{Text: value}}}, nil
		},
	}
	collector := &eventCollector{}
	calls := 0
	sequenced := func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		calls++
		response := wire.NewAssistantMessageEventStream()
		if calls == 1 {
			message := testAssistant(model.ContentList{
				model.ToolCall{ID: "tool-1", Name: "echo", Arguments: map[string]any{"value": "first"}},
				model.ToolCall{ID: "tool-2", Name: "echo", Arguments: map[string]any{"value": "second"}},
			}, model.StopToolUse)
			response.Push(model.AssistantMessageEvent{Type: model.EventDone, Reason: message.StopReason, Message: message})
			return response, nil
		}
		message := testAssistantText("done")
		response.Push(model.AssistantMessageEvent{Type: model.EventDone, Reason: message.StopReason, Message: message})
		return response, nil
	}
	config := model.AgentLoopConfig{Model: testModel(), ConvertToLlm: identityConverter, ToolExecution: model.ToolParallel}
	messages, err := RunAgentLoop(context.Background(), []model.AgentMessage{testUser("both")}, model.AgentContext{Tools: []model.AgentTool{tool}}, config, collector.sink(), sequenced)
	if err != nil {
		t.Fatalf("RunAgentLoop: %v", err)
	}
	if !parallelObserved {
		t.Fatalf("second tool did not start before the first resolved")
	}
	var resultIDs []string
	for _, message := range messages {
		if tr, ok := message.(model.ToolResultMessage); ok {
			resultIDs = append(resultIDs, tr.ToolCallID)
		}
	}
	if len(resultIDs) != 2 || resultIDs[0] != "tool-1" || resultIDs[1] != "tool-2" {
		t.Fatalf("tool result order = %v, want [tool-1 tool-2]", resultIDs)
	}
	var endIDs []string
	for _, event := range collector.snapshot() {
		if event.Type == model.EvToolExecutionEnd {
			endIDs = append(endIDs, event.ToolCallID)
		}
	}
	if len(endIDs) != 2 || endIDs[0] != "tool-2" || endIDs[1] != "tool-1" {
		t.Fatalf("tool execution end order = %v, want [tool-2 tool-1]", endIDs)
	}
}

func TestRunAgentLoopInjectsSteeringAfterToolCalls(t *testing.T) {
	var executed []string
	tool := testTool("echo")
	tool.Parameters = model.Object(model.Prop("value", model.String()))
	tool.Execute = func(ctx context.Context, id string, params map[string]any, onUpdate model.ToolUpdateFunc) (model.AgentToolResult, error) {
		executed = append(executed, params["value"].(string))
		return model.AgentToolResult{Content: model.ContentList{model.TextContent{Text: "ok"}}}, nil
	}
	queued := false
	config := model.AgentLoopConfig{
		Model:         testModel(),
		ConvertToLlm:  identityConverter,
		ToolExecution: model.ToolSequential,
		GetSteeringMessages: func() []model.AgentMessage {
			if len(executed) >= 2 && !queued {
				queued = true
				return []model.AgentMessage{testUser("interrupt")}
			}
			return nil
		},
	}
	streamFn := sequenceStream(
		testAssistant(model.ContentList{
			model.ToolCall{ID: "tool-1", Name: "echo", Arguments: map[string]any{"value": "first"}},
			model.ToolCall{ID: "tool-2", Name: "echo", Arguments: map[string]any{"value": "second"}},
		}, model.StopToolUse),
		testAssistantText("done"),
	)
	if _, err := RunAgentLoop(context.Background(), []model.AgentMessage{testUser("start")}, model.AgentContext{Tools: []model.AgentTool{tool}}, config, (&eventCollector{}).sink(), streamFn); err != nil {
		t.Fatalf("RunAgentLoop: %v", err)
	}
	if len(executed) != 2 || executed[0] != "first" || executed[1] != "second" {
		t.Fatalf("executed = %v", executed)
	}
}

func TestRunAgentLoopTerminatesBatchWhenAllResultsTerminate(t *testing.T) {
	tool := testTool("echo")
	tool.Execute = func(ctx context.Context, id string, params map[string]any, onUpdate model.ToolUpdateFunc) (model.AgentToolResult, error) {
		return model.AgentToolResult{Content: model.ContentList{model.TextContent{Text: "ok"}}, Terminate: true}, nil
	}
	config := model.AgentLoopConfig{Model: testModel(), ConvertToLlm: identityConverter}
	streamFn := sequenceStream(testAssistant(model.ContentList{model.ToolCall{ID: "tool-1", Name: "echo", Arguments: map[string]any{}}}, model.StopToolUse))
	messages, err := RunAgentLoop(context.Background(), []model.AgentMessage{testUser("run")}, model.AgentContext{Tools: []model.AgentTool{tool}}, config, (&eventCollector{}).sink(), streamFn)
	if err != nil {
		t.Fatalf("RunAgentLoop: %v", err)
	}
	roles := make([]model.Role, len(messages))
	for i, message := range messages {
		roles[i] = message.MessageRole()
	}
	if len(roles) != 4 || roles[3] != model.RoleToolResult {
		t.Fatalf("roles = %v, want [system user assistant toolResult]", roles)
	}
}

func TestRunAgentLoopFinishTurnRunsAfterToolResultsBeforeTurnEnd(t *testing.T) {
	tool := testTool("echo")
	tool.Execute = func(ctx context.Context, id string, params map[string]any, onUpdate model.ToolUpdateFunc) (model.AgentToolResult, error) {
		return model.AgentToolResult{Content: model.ContentList{model.TextContent{Text: "ok"}}, Terminate: true}, nil
	}
	ordering := []string{}
	config := model.AgentLoopConfig{
		Model:        testModel(),
		ConvertToLlm: identityConverter,
		FinishTurn: func(ctx context.Context, turn model.AgentTurnContext) model.AgentTurnDecision {
			if len(turn.ToolResults) != 1 {
				t.Errorf("tool results = %d, want 1", len(turn.ToolResults))
			}
			ordering = append(ordering, "finishTurn")
			return ""
		},
	}
	sink := func(event model.AgentEvent) error {
		if event.Type == model.EvMessageEnd {
			ordering = append(ordering, "message_end:"+string(event.Message.MessageRole()))
		}
		if event.Type == model.EvTurnEnd {
			ordering = append(ordering, "turn_end")
		}
		return nil
	}
	streamFn := sequenceStream(
		testAssistant(model.ContentList{model.ToolCall{ID: "tool-1", Name: "echo", Arguments: map[string]any{}}}, model.StopToolUse),
		testAssistantText("done"),
	)
	if _, err := RunAgentLoop(context.Background(), []model.AgentMessage{testUser("echo")}, model.AgentContext{Tools: []model.AgentTool{tool}}, config, sink, streamFn); err != nil {
		t.Fatalf("RunAgentLoop: %v", err)
	}
	got := ordering[len(ordering)-3:]
	want := []string{"message_end:" + string(model.RoleToolResult), "finishTurn", "turn_end"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("ordering tail = %v, want %v", got, want)
	}
}

func TestRunAgentLoopFailedResponseKeepsHardExit(t *testing.T) {
	for _, stop := range []model.StopReason{model.StopError, model.StopAborted} {
		ordering := []string{}
		providerCalls := 0
		steeringPolls := 0
		followUpPolls := 0
		config := model.AgentLoopConfig{
			Model:        testModel(),
			ConvertToLlm: identityConverter,
			FinishTurn: func(ctx context.Context, turn model.AgentTurnContext) model.AgentTurnDecision {
				if turn.Message.StopReason != stop {
					t.Errorf("stop reason = %s, want %s", turn.Message.StopReason, stop)
				}
				ordering = append(ordering, "finishTurn")
				return model.TurnContinue
			},
			GetSteeringMessages: func() []model.AgentMessage { steeringPolls++; return nil },
			GetFollowUpMessages: func() []model.AgentMessage { followUpPolls++; return []model.AgentMessage{testUser("queued")} },
		}
		streamFn := func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
			providerCalls++
			response := wire.NewAssistantMessageEventStream()
			response.Push(model.AssistantMessageEvent{Type: model.EventError, Reason: stop, Error: testAssistant(model.ContentList{}, stop)})
			return response, nil
		}
		sink := func(event model.AgentEvent) error {
			if event.Type == model.EvTurnEnd {
				ordering = append(ordering, "turn_end")
			}
			return nil
		}
		if _, err := RunAgentLoop(context.Background(), []model.AgentMessage{testUser("run")}, model.AgentContext{}, config, sink, streamFn); err != nil {
			t.Fatalf("RunAgentLoop: %v", err)
		}
		if providerCalls != 1 || steeringPolls != 1 || followUpPolls != 0 {
			t.Fatalf("calls=%d steering=%d followUp=%d", providerCalls, steeringPolls, followUpPolls)
		}
		if len(ordering) != 2 || ordering[0] != "finishTurn" || ordering[1] != "turn_end" {
			t.Fatalf("ordering = %v, want [finishTurn turn_end]", ordering)
		}
	}
}

func TestRunAgentLoopFinishTurnEndSkipsQueuesAndPreparation(t *testing.T) {
	providerCalls := 0
	steeringPolls := 0
	followUpPolls := 0
	prepareNextTurnCalls := 0
	config := model.AgentLoopConfig{
		Model:        testModel(),
		ConvertToLlm: identityConverter,
		FinishTurn: func(ctx context.Context, turn model.AgentTurnContext) model.AgentTurnDecision {
			return model.TurnEnd
		},
		PrepareNextTurn: func(turn model.AgentTurnContext) *model.AgentLoopTurnUpdate {
			prepareNextTurnCalls++
			return nil
		},
		GetSteeringMessages: func() []model.AgentMessage { steeringPolls++; return nil },
		GetFollowUpMessages: func() []model.AgentMessage { followUpPolls++; return []model.AgentMessage{testUser("queued")} },
	}
	streamFn := func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		providerCalls++
		return doneStream(testAssistantText("done"))(ctx, m, transcript, options)
	}
	if _, err := RunAgentLoop(context.Background(), []model.AgentMessage{testUser("run")}, model.AgentContext{}, config, (&eventCollector{}).sink(), streamFn); err != nil {
		t.Fatalf("RunAgentLoop: %v", err)
	}
	if providerCalls != 1 || followUpPolls != 0 || prepareNextTurnCalls != 0 {
		t.Fatalf("provider=%d followUp=%d prepareNext=%d", providerCalls, followUpPolls, prepareNextTurnCalls)
	}
}

func TestRunAgentLoopPrepareNextTurnSnapshotReachesSecondRequest(t *testing.T) {
	tool := testTool("echo")
	var secondHasUpdate bool
	prepareCalls := 0
	config := model.AgentLoopConfig{
		Model:        testModel(),
		ConvertToLlm: identityConverter,
		PrepareNextTurn: func(turn model.AgentTurnContext) *model.AgentLoopTurnUpdate {
			prepareCalls++
			return &model.AgentLoopTurnUpdate{
				Context:  turn.Context,
				Messages: []model.AgentMessage{model.NewSystemText("updated guidance", 1)},
			}
		},
	}
	providerCalls := 0
	streamFn := func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		providerCalls++
		if providerCalls == 2 {
			for _, message := range transcript.Messages {
				if message.MessageRole() == model.RoleSystem {
					if text, ok := message.(model.SystemMessage); ok && model.GetSystemMessageText(text) == "updated guidance" {
						secondHasUpdate = true
					}
				}
			}
		}
		response := wire.NewAssistantMessageEventStream()
		var message *model.AssistantMessage
		if providerCalls == 1 {
			message = testAssistant(model.ContentList{model.ToolCall{ID: "tool-1", Name: "echo", Arguments: map[string]any{}}}, model.StopToolUse)
		} else {
			message = testAssistantText("done")
		}
		response.Push(model.AssistantMessageEvent{Type: model.EventDone, Reason: message.StopReason, Message: message})
		return response, nil
	}
	if _, err := RunAgentLoop(context.Background(), []model.AgentMessage{testUser("echo")}, model.AgentContext{Tools: []model.AgentTool{tool}}, config, (&eventCollector{}).sink(), streamFn); err != nil {
		t.Fatalf("RunAgentLoop: %v", err)
	}
	if providerCalls != 2 || prepareCalls != 1 || !secondHasUpdate {
		t.Fatalf("provider=%d prepare=%d secondHasUpdate=%v", providerCalls, prepareCalls, secondHasUpdate)
	}
}

func TestAgentLoopContinueRejectsEmptyAndAssistantTail(t *testing.T) {
	config := model.AgentLoopConfig{Model: testModel(), ConvertToLlm: identityConverter}
	if _, err := AgentLoopContinue(context.Background(), model.AgentContext{}, config, doneStream(testAssistantText("x"))); err == nil {
		t.Fatalf("expected empty-context error")
	}
	agentCtx := model.AgentContext{Messages: []model.AgentMessage{testUser("hi"), testAssistantText("yo")}}
	if _, err := AgentLoopContinue(context.Background(), agentCtx, config, doneStream(testAssistantText("x"))); err == nil {
		t.Fatalf("expected assistant-tail error")
	}
}

func TestAgentLoopContinueEmitsOnlyNewMessages(t *testing.T) {
	agentCtx := model.AgentContext{Messages: []model.AgentMessage{testUser("Hello")}}
	config := model.AgentLoopConfig{Model: testModel(), ConvertToLlm: identityConverter}
	stream, err := AgentLoopContinue(context.Background(), agentCtx, config, doneStream(testAssistantText("Response")))
	if err != nil {
		t.Fatalf("AgentLoopContinue: %v", err)
	}
	var messageEvents int
	for event := range stream.Events() {
		if event.Type == model.EvMessageStart || event.Type == model.EvMessageEnd {
			messageEvents++
		}
	}
	messages := stream.Result()
	if len(messages) != 1 || messages[0].MessageRole() != model.RoleAssistant {
		t.Fatalf("messages = %v", messages)
	}
	if messageEvents != 2 {
		t.Fatalf("message events = %d, want 2", messageEvents)
	}
}
