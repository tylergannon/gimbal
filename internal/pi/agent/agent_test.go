package agent

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/wire"
)

func TestAgentDefaultState(t *testing.T) {
	agent := NewAgent(AgentOptions{})
	state := agent.State()
	if state.Model == nil {
		t.Fatalf("model is nil")
	}
	if state.ThinkingLevel != model.ThinkingOff {
		t.Fatalf("thinking level = %s, want off", state.ThinkingLevel)
	}
	if len(state.Tools) != 0 || len(state.Messages) != 0 {
		t.Fatalf("tools = %d, messages = %d, want 0/0", len(state.Tools), len(state.Messages))
	}
	if state.IsStreaming {
		t.Fatalf("isStreaming = true, want false")
	}
	if len(state.PendingToolCalls) != 0 {
		t.Fatalf("pending tool calls = %v", state.PendingToolCalls)
	}
	if state.ErrorMessage != "" {
		t.Fatalf("error message = %q", state.ErrorMessage)
	}
}

func TestAgentInitialStateSeedsSystemMessage(t *testing.T) {
	agent := NewAgent(AgentOptions{InitialState: &model.AgentState{
		SystemPrompt:  "You are a helpful assistant.",
		ThinkingLevel: model.ThinkingLow,
	}})
	state := agent.State()
	if len(state.Messages) != 1 {
		t.Fatalf("messages = %v", state.Messages)
	}
	system, ok := state.Messages[0].(model.SystemMessage)
	if !ok {
		t.Fatalf("first message = %T, want SystemMessage", state.Messages[0])
	}
	if text, _ := system.StringContent(); text != "You are a helpful assistant." {
		t.Fatalf("system content = %q", text)
	}
	if state.ThinkingLevel != model.ThinkingLow {
		t.Fatalf("thinking level = %s", state.ThinkingLevel)
	}
}

func TestAgentInitialPromptAndToolsBecomeTranscript(t *testing.T) {
	tool := model.AgentTool{Name: "echo", Description: "Echo input", Parameters: model.Object()}
	agent := NewAgent(AgentOptions{InitialState: &model.AgentState{SystemPrompt: "You are helpful.", Tools: []model.AgentTool{tool}}})
	state := agent.State()
	system, ok := state.Messages[0].(model.SystemMessage)
	if !ok {
		t.Fatalf("first message = %T", state.Messages[0])
	}
	if text, _ := system.StringContent(); text != "You are helpful." {
		t.Fatalf("content = %q", text)
	}
	if len(system.ToolsAdded) != 1 || system.ToolsAdded[0].Name != "echo" {
		t.Fatalf("toolsAdded = %v", system.ToolsAdded)
	}
}

func TestAgentDeclaresToolLoadoutChanges(t *testing.T) {
	first := model.AgentTool{Name: "first", Description: "first tool", Parameters: model.Object()}
	second := model.AgentTool{Name: "second", Description: "second tool", Parameters: model.Object()}
	var mu sync.Mutex
	var requests [][]string
	streamFn := func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		var descriptions []string
		for _, message := range transcript.Messages {
			system, ok := message.(model.SystemMessage)
			if !ok {
				continue
			}
			added := make([]string, len(system.ToolsAdded))
			for i, tool := range system.ToolsAdded {
				added[i] = tool.Name
			}
			removed := make([]string, len(system.ToolsRemoved))
			for i, tool := range system.ToolsRemoved {
				removed[i] = tool.Name
			}
			descriptions = append(descriptions, "+"+joinNames(added), "-"+joinNames(removed))
		}
		mu.Lock()
		requests = append(requests, descriptions)
		mu.Unlock()
		return doneStream(testAssistantText("done"))(ctx, m, transcript, options)
	}
	agent := NewAgent(AgentOptions{
		InitialState: &model.AgentState{SystemPrompt: "You are helpful.", Tools: []model.AgentTool{first}},
		StreamFn:     streamFn,
	})
	if err := agent.Prompt(context.Background(), "one"); err != nil {
		t.Fatalf("prompt one: %v", err)
	}
	agent.SetTools([]model.AgentTool{second})
	if err := agent.Prompt(context.Background(), "two"); err != nil {
		t.Fatalf("prompt two: %v", err)
	}
	if err := agent.Prompt(context.Background(), "three"); err != nil {
		t.Fatalf("prompt three: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 3 {
		t.Fatalf("requests = %d, want 3: %v", len(requests), requests)
	}
	if requests[0][0] != "+first" || requests[0][1] != "-" {
		t.Fatalf("request 0 = %v", requests[0])
	}
	if requests[1][0] != "+first" || requests[1][2] != "+second" || requests[1][3] != "-first" {
		t.Fatalf("request 1 = %v", requests[1])
	}
}

func joinNames(names []string) string {
	return strings.Join(names, ",")
}

func TestAgentResetRestoresBaseline(t *testing.T) {
	tool := model.AgentTool{Name: "echo", Description: "Echo input", Parameters: model.Object()}
	agent := NewAgent(AgentOptions{InitialState: &model.AgentState{
		SystemPrompt: "You are helpful.",
		Tools:        []model.AgentTool{tool},
		Messages:     []model.AgentMessage{model.NewUserText("old", 1)},
	}})
	if err := agent.Reset(); err != nil {
		t.Fatalf("reset: %v", err)
	}
	state := agent.State()
	if len(state.Messages) != 1 {
		t.Fatalf("messages = %v", state.Messages)
	}
	system, ok := state.Messages[0].(model.SystemMessage)
	if !ok {
		t.Fatalf("first message = %T", state.Messages[0])
	}
	if text, _ := system.StringContent(); text != "You are helpful." {
		t.Fatalf("content = %q", text)
	}
	if len(system.ToolsAdded) != 1 || system.ToolsAdded[0].Name != "echo" {
		t.Fatalf("toolsAdded = %v", system.ToolsAdded)
	}
}

func TestAgentThrownRunFailureEmitsLifecycle(t *testing.T) {
	agent := NewAgent(AgentOptions{StreamFn: func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		return nil, errors.New("provider exploded")
	}})
	var mu sync.Mutex
	var events []model.AgentEventType
	agent.Subscribe(func(ctx context.Context, event model.AgentEvent) error {
		mu.Lock()
		events = append(events, event.Type)
		mu.Unlock()
		return nil
	})
	if err := agent.Prompt(context.Background(), "hello"); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []model.AgentEventType{
		model.EvAgentStart, model.EvTurnStart, model.EvMessageStart, model.EvMessageEnd,
		model.EvMessageStart, model.EvMessageEnd, model.EvTurnEnd, model.EvAgentEnd,
	}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events = %v, want %v", events, want)
		}
	}
	state := agent.State()
	last, ok := state.Messages[len(state.Messages)-1].(*model.AssistantMessage)
	if !ok {
		t.Fatalf("last message = %T", state.Messages[len(state.Messages)-1])
	}
	if last.StopReason != model.StopError || last.ErrorMessage != "provider exploded" {
		t.Fatalf("last = %+v", last)
	}
	if state.ErrorMessage != "provider exploded" {
		t.Fatalf("state error = %q", state.ErrorMessage)
	}
}

func TestAgentWaitsForAsyncSubscribers(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	agent := NewAgent(AgentOptions{StreamFn: doneStream(testAssistantText("ok"))})
	agent.Subscribe(func(ctx context.Context, event model.AgentEvent) error {
		if event.Type == model.EvAgentEnd {
			close(started)
			<-release
		}
		return nil
	})
	promptDone := make(chan error, 1)
	go func() { promptDone <- agent.Prompt(context.Background(), "hello") }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatalf("listener did not start")
	}
	select {
	case <-promptDone:
		t.Fatalf("prompt resolved before the listener finished")
	case <-time.After(20 * time.Millisecond):
	}
	if !agent.State().IsStreaming {
		t.Fatalf("state is not streaming while listeners run")
	}
	close(release)
	if err := <-promptDone; err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if agent.State().IsStreaming {
		t.Fatalf("state still streaming after prompt resolved")
	}
}

func TestAgentWaitForIdleWaitsForListeners(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	agent := NewAgent(AgentOptions{StreamFn: doneStream(testAssistantText("ok"))})
	agent.Subscribe(func(ctx context.Context, event model.AgentEvent) error {
		if event.Type == model.EvMessageEnd {
			if assistant, ok := asAssistant(event.Message); ok && assistant.StopReason == model.StopStop {
				close(started)
				<-release
			}
		}
		return nil
	})
	promptDone := make(chan error, 1)
	go func() { promptDone <- agent.Prompt(context.Background(), "hello") }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatalf("listener did not start")
	}
	idleDone := make(chan struct{})
	go func() { agent.WaitForIdle(); close(idleDone) }()
	select {
	case <-idleDone:
		t.Fatalf("WaitForIdle returned before the listener finished")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-promptDone; err != nil {
		t.Fatalf("prompt: %v", err)
	}
	select {
	case <-idleDone:
	case <-time.After(2 * time.Second):
		t.Fatalf("WaitForIdle did not return")
	}
}

func TestAgentAbortCancelsListenerContext(t *testing.T) {
	var mu sync.Mutex
	var runCtx context.Context
	streamFn := func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		stream := wire.NewAssistantMessageEventStream()
		go func() {
			<-ctx.Done()
			stream.Push(model.AssistantMessageEvent{Type: model.EventError, Reason: model.StopAborted, Error: testAssistant(model.ContentList{}, model.StopAborted)})
		}()
		return stream, nil
	}
	agent := NewAgent(AgentOptions{StreamFn: streamFn})
	agent.Subscribe(func(ctx context.Context, event model.AgentEvent) error {
		if event.Type == model.EvAgentStart {
			mu.Lock()
			runCtx = ctx
			mu.Unlock()
		}
		return nil
	})
	promptDone := make(chan error, 1)
	go func() { promptDone <- agent.Prompt(context.Background(), "hello") }()
	time.Sleep(20 * time.Millisecond)
	agent.Abort()
	if err := <-promptDone; err != nil {
		t.Fatalf("prompt: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if runCtx == nil || runCtx.Err() == nil {
		t.Fatalf("listener context was not canceled")
	}
}

func TestAgentIgnoresLateToolUpdates(t *testing.T) {
	var update model.ToolUpdateFunc
	tool := model.AgentTool{
		Name: "delayed", Description: "Delayed", Parameters: model.Object(),
		Execute: func(ctx context.Context, id string, params map[string]any, onUpdate model.ToolUpdateFunc) (model.AgentToolResult, error) {
			update = onUpdate
			onUpdate(model.AgentToolResult{Content: model.ContentList{model.TextContent{Text: "running"}}})
			return model.AgentToolResult{Content: model.ContentList{model.TextContent{Text: "ok"}}}, nil
		},
	}
	agent := NewAgent(AgentOptions{
		InitialState: &model.AgentState{Tools: []model.AgentTool{tool}},
		StreamFn: sequenceStream(
			testAssistant(model.ContentList{model.ToolCall{ID: "call-1", Name: "delayed", Arguments: map[string]any{}}}, model.StopToolUse),
			testAssistantText("done"),
		),
	})
	var mu sync.Mutex
	updates := 0
	agent.Subscribe(func(ctx context.Context, event model.AgentEvent) error {
		if event.Type == model.EvToolExecutionUpdate {
			mu.Lock()
			updates++
			mu.Unlock()
		}
		return nil
	})
	if err := agent.Prompt(context.Background(), "run"); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if update == nil {
		t.Fatalf("tool did not capture update callback")
	}
	update(model.AgentToolResult{Content: model.ContentList{model.TextContent{Text: "late"}}})
	time.Sleep(10 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if updates != 1 {
		t.Fatalf("tool updates = %d, want 1", updates)
	}
}

func TestAgentPeekQueuedMessagesDoesNotConsume(t *testing.T) {
	agent := NewAgent(AgentOptions{SteeringMode: model.QueueOneAtATime, FollowUpMode: model.QueueAll})
	first := model.NewUserText("first steering", 1)
	second := model.NewUserText("second steering", 2)
	followUp := model.NewUserText("follow-up", 3)
	agent.Steer(first)
	agent.Steer(second)
	agent.FollowUp(followUp)
	if got := agent.PeekQueuedMessages(); len(got) != 1 || got[0].(model.UserMessage).Timestamp != 1 {
		t.Fatalf("peek = %v", got)
	}
	if got := agent.PeekQueuedMessages(); len(got) != 1 || got[0].(model.UserMessage).Timestamp != 1 {
		t.Fatalf("second peek = %v", got)
	}
	agent.ClearSteeringQueue()
	if got := agent.PeekQueuedMessages(); len(got) != 1 || got[0].(model.UserMessage).Timestamp != 3 {
		t.Fatalf("peek after clear = %v", got)
	}
}

func TestAgentContinueFromAssistantTailUsesSteering(t *testing.T) {
	var mu sync.Mutex
	var requestUsers [][]string
	streamFn := func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		var users []string
		for _, message := range transcript.Messages {
			if user, ok := message.(model.UserMessage); ok {
				if text, ok := user.StringContent(); ok {
					users = append(users, text)
				}
			}
		}
		mu.Lock()
		requestUsers = append(requestUsers, users)
		mu.Unlock()
		return doneStream(testAssistantText("Processed"))(ctx, m, transcript, options)
	}
	agent := NewAgent(AgentOptions{SteeringMode: model.QueueOneAtATime, StreamFn: streamFn})
	agent.SetMessages([]model.AgentMessage{model.NewUserText("Initial", 1), testAssistantText("Initial response")})
	agent.Steer(model.NewUserText("Steering 1", 2))
	agent.Steer(model.NewUserText("Steering 2", 3))
	if err := agent.Continue(context.Background()); err != nil {
		t.Fatalf("continue: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requestUsers) != 2 {
		t.Fatalf("requests = %d, want 2: %v", len(requestUsers), requestUsers)
	}
	if !containsString(requestUsers[0], "Steering 1") || containsString(requestUsers[0], "Steering 2") {
		t.Fatalf("request 0 = %v", requestUsers[0])
	}
	if !containsString(requestUsers[1], "Steering 2") {
		t.Fatalf("request 1 = %v", requestUsers[1])
	}
}

func containsString(values []string, want string) bool {
	return slices.Contains(values, want)
}

func TestAgentForwardsThinkingLevel(t *testing.T) {
	var mu sync.Mutex
	var seen model.ThinkingLevel
	streamFn := func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		mu.Lock()
		seen = options.Reasoning
		mu.Unlock()
		return doneStream(testAssistantText("ok"))(ctx, m, transcript, options)
	}
	agent := NewAgent(AgentOptions{InitialState: &model.AgentState{ThinkingLevel: model.ThinkingHigh}, StreamFn: streamFn})
	if err := agent.Prompt(context.Background(), "hello"); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if seen != model.ThinkingHigh {
		t.Fatalf("reasoning = %q, want high", seen)
	}
}

func TestAgentForwardsSessionID(t *testing.T) {
	var mu sync.Mutex
	var seen string
	streamFn := func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		mu.Lock()
		seen = options.SessionID
		mu.Unlock()
		return doneStream(testAssistantText("ok"))(ctx, m, transcript, options)
	}
	agent := NewAgent(AgentOptions{StreamFn: streamFn})
	agent.SetSessionID("session-abc")
	if err := agent.Prompt(context.Background(), "hello"); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	mu.Lock()
	if seen != "session-abc" {
		t.Fatalf("session id = %q", seen)
	}
	mu.Unlock()
	if agent.SessionID() != "session-abc" {
		t.Fatalf("SessionID() = %q", agent.SessionID())
	}
}

func TestAgentRejectsConcurrentPrompt(t *testing.T) {
	release := make(chan struct{})
	agent := NewAgent(AgentOptions{StreamFn: func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		stream := wire.NewAssistantMessageEventStream()
		go func() {
			<-release
			stream.Push(model.AssistantMessageEvent{Type: model.EventDone, Reason: model.StopStop, Message: testAssistantText("done")})
		}()
		return stream, nil
	}})
	firstDone := make(chan error, 1)
	go func() { firstDone <- agent.Prompt(context.Background(), "first") }()
	time.Sleep(20 * time.Millisecond)
	if err := agent.Prompt(context.Background(), "second"); err == nil {
		t.Fatalf("second prompt succeeded, want error")
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first prompt: %v", err)
	}
}

func TestAgentRejectsResetWhileProcessing(t *testing.T) {
	release := make(chan struct{})
	agent := NewAgent(AgentOptions{StreamFn: func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		stream := wire.NewAssistantMessageEventStream()
		go func() {
			<-release
			stream.Push(model.AssistantMessageEvent{Type: model.EventDone, Reason: model.StopStop, Message: testAssistantText("Done")})
		}()
		return stream, nil
	}})
	promptDone := make(chan error, 1)
	go func() { promptDone <- agent.Prompt(context.Background(), "Hello") }()
	time.Sleep(20 * time.Millisecond)
	if err := agent.Reset(); err == nil {
		t.Fatalf("reset succeeded while processing")
	}
	if !agent.State().IsStreaming {
		t.Fatalf("reset corrupted streaming state")
	}
	close(release)
	if err := <-promptDone; err != nil {
		t.Fatalf("prompt: %v", err)
	}
}
