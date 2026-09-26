package agent

import (
	"context"
	"errors"
	"maps"
	"slices"
	"sync"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// This file ports packages/agent/src/agent.ts: a stateful wrapper around the
// low-level loop that owns the transcript, emits lifecycle events, executes
// tools, and exposes steering/follow-up queues. Context cancellation replaces
// pi's AbortSignal.

var defaultModel = &model.Model{
	ID: "unknown", Name: "unknown", Api: "unknown", Provider: "unknown",
	Input: []string{}, ContextWindow: 0, MaxTokens: 0,
}

// Listener receives agent events with the active run's cancellation context.
type Listener func(ctx context.Context, event model.AgentEvent) error

type pendingQueue struct {
	mode     model.QueueMode
	messages []model.AgentMessage
}

func (q *pendingQueue) enqueue(message model.AgentMessage) { q.messages = append(q.messages, message) }
func (q *pendingQueue) hasItems() bool                     { return len(q.messages) > 0 }
func (q *pendingQueue) clear()                             { q.messages = nil }

func (q *pendingQueue) peek() []model.AgentMessage {
	if q.mode == model.QueueAll {
		return slices.Clone(q.messages)
	}
	if len(q.messages) == 0 {
		return nil
	}
	return []model.AgentMessage{q.messages[0]}
}

func (q *pendingQueue) drain() []model.AgentMessage {
	drained := q.peek()
	q.messages = q.messages[len(drained):]
	return drained
}

// AgentOptions configures a new Agent.
type AgentOptions struct {
	InitialState     *model.AgentState
	ConvertToLlm     func(messages []model.AgentMessage) []model.Message
	TransformContext func(ctx context.Context, messages []model.AgentMessage) []model.AgentMessage
	StreamFn         StreamFunction
	GetApiKey        func(provider string) string
	// SimpleStreamOptions carries the provider request options forwarded to
	// every stream call.
	model.SimpleStreamOptions

	BeforeToolCall  func(ctx context.Context, c model.BeforeToolCallContext) *model.BeforeToolCallResult
	AfterToolCall   func(ctx context.Context, c model.AfterToolCallContext) *model.AfterToolCallResult
	FinishTurn      model.FinishTurnFunc
	PrepareRequest  model.PrepareRequestFunc
	PrepareNextTurn func(c model.AgentTurnContext) *model.AgentLoopTurnUpdate
	SteeringMode    model.QueueMode
	FollowUpMode    model.QueueMode
	ToolExecution   model.ToolExecutionMode
}

type activeRun struct {
	cancel context.CancelFunc
	ctx    context.Context
	done   chan struct{}
}

// Agent is a stateful wrapper around the low-level agent loop.
type Agent struct {
	mu        sync.Mutex
	state     model.AgentState
	listeners []Listener

	steeringQueue pendingQueue
	followUpQueue pendingQueue

	convertToLlm     func(messages []model.AgentMessage) []model.Message
	transformContext func(ctx context.Context, messages []model.AgentMessage) []model.AgentMessage
	streamFn         StreamFunction
	getApiKey        func(provider string) string
	simpleOptions    model.SimpleStreamOptions

	beforeToolCall  func(ctx context.Context, c model.BeforeToolCallContext) *model.BeforeToolCallResult
	afterToolCall   func(ctx context.Context, c model.AfterToolCallContext) *model.AfterToolCallResult
	finishTurn      model.FinishTurnFunc
	prepareRequest  model.PrepareRequestFunc
	prepareNextTurn func(c model.AgentTurnContext) *model.AgentLoopTurnUpdate

	toolExecution model.ToolExecutionMode
	active        *activeRun
}

// NewAgent constructs an Agent from options. A nil StreamFn leaves the agent
// without a provider stream; a run then fails with a synthesized error turn.
func NewAgent(opts AgentOptions) *Agent {
	state := model.AgentState{
		Model:            defaultModel,
		ThinkingLevel:    model.ThinkingOff,
		PendingToolCalls: map[string]bool{},
	}
	if opts.InitialState != nil {
		initial := opts.InitialState
		if initial.Model != nil {
			state.Model = initial.Model
		}
		if initial.ThinkingLevel != "" {
			state.ThinkingLevel = initial.ThinkingLevel
		}
		state.Tools = append([]model.AgentTool(nil), initial.Tools...)
		state.Messages = append([]model.AgentMessage(nil), initial.Messages...)
		declarations := make([]model.Tool, len(state.Tools))
		for i, tool := range state.Tools {
			declarations[i] = model.ToToolDeclaration(tool.AsTool())
		}
		initialMessage, ok := model.CreateInitialSystemMessage(initial.SystemPrompt, declarations)
		if ok && (len(state.Messages) == 0 || state.Messages[0].MessageRole() != model.RoleSystem) {
			state.Messages = append([]model.AgentMessage{initialMessage}, state.Messages...)
		}
	}

	agent := &Agent{
		state:            state,
		convertToLlm:     opts.ConvertToLlm,
		transformContext: opts.TransformContext,
		streamFn:         opts.StreamFn,
		getApiKey:        opts.GetApiKey,
		simpleOptions:    opts.SimpleStreamOptions,
		beforeToolCall:   opts.BeforeToolCall,
		afterToolCall:    opts.AfterToolCall,
		finishTurn:       opts.FinishTurn,
		prepareRequest:   opts.PrepareRequest,
		prepareNextTurn:  opts.PrepareNextTurn,
		toolExecution:    opts.ToolExecution,
	}
	if agent.convertToLlm == nil {
		agent.convertToLlm = defaultConvertToLlm
	}
	if agent.toolExecution == "" {
		agent.toolExecution = model.ToolParallel
	}
	if agent.simpleOptions.Transport == "" {
		agent.simpleOptions.Transport = model.TransportAuto
	}
	agent.steeringQueue.mode = orMode(opts.SteeringMode, model.QueueOneAtATime)
	agent.followUpQueue.mode = orMode(opts.FollowUpMode, model.QueueOneAtATime)
	return agent
}

func orMode(mode, fallback model.QueueMode) model.QueueMode {
	if mode == "" {
		return fallback
	}
	return mode
}

// Subscribe registers an event listener; the returned function unsubscribes.
func (a *Agent) Subscribe(listener Listener) func() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.listeners = append(a.listeners, listener)
	index := len(a.listeners) - 1
	return func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		if index < len(a.listeners) {
			a.listeners[index] = nil
		}
	}
}

// State returns a snapshot of the agent state. Slices share backing storage and
// should be treated read-only. SystemPrompt is replayed from the transcript.
func (a *Agent) State() model.AgentState {
	a.mu.Lock()
	defer a.mu.Unlock()
	state := a.state
	state.SystemPrompt = model.GetCurrentSystemPrompt(state.Messages)
	return state
}

// SetModel sets the active model for future turns.
func (a *Agent) SetModel(m *model.Model) { a.mu.Lock(); a.state.Model = m; a.mu.Unlock() }

// SetThinkingLevel sets the reasoning level for future turns.
func (a *Agent) SetThinkingLevel(level model.ThinkingLevel) {
	a.mu.Lock()
	a.state.ThinkingLevel = level
	a.mu.Unlock()
}

// SetTools replaces the available tools (copied).
func (a *Agent) SetTools(tools []model.AgentTool) {
	a.mu.Lock()
	a.state.Tools = append([]model.AgentTool(nil), tools...)
	a.mu.Unlock()
}

// SetMessages replaces the transcript (copied).
func (a *Agent) SetMessages(messages []model.AgentMessage) {
	a.mu.Lock()
	a.state.Messages = append([]model.AgentMessage(nil), messages...)
	a.mu.Unlock()
}

// SetSessionID sets the session identifier forwarded to provider requests.
func (a *Agent) SetSessionID(id string) { a.mu.Lock(); a.simpleOptions.SessionID = id; a.mu.Unlock() }

// SessionID returns the session identifier forwarded to provider requests.
func (a *Agent) SessionID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.simpleOptions.SessionID
}

// SetSteeringMode controls how queued steering messages drain.
func (a *Agent) SetSteeringMode(mode model.QueueMode) {
	a.mu.Lock()
	a.steeringQueue.mode = mode
	a.mu.Unlock()
}

// SetFollowUpMode controls how queued follow-up messages drain.
func (a *Agent) SetFollowUpMode(mode model.QueueMode) {
	a.mu.Lock()
	a.followUpQueue.mode = mode
	a.mu.Unlock()
}

// Steer queues a message to inject after the current assistant turn finishes.
func (a *Agent) Steer(message model.AgentMessage) {
	a.mu.Lock()
	a.steeringQueue.enqueue(message)
	a.mu.Unlock()
}

// FollowUp queues a message to run after the agent would otherwise stop.
func (a *Agent) FollowUp(message model.AgentMessage) {
	a.mu.Lock()
	a.followUpQueue.enqueue(message)
	a.mu.Unlock()
}

// ClearSteeringQueue removes all queued steering messages.
func (a *Agent) ClearSteeringQueue() { a.mu.Lock(); a.steeringQueue.clear(); a.mu.Unlock() }

// ClearFollowUpQueue removes all queued follow-up messages.
func (a *Agent) ClearFollowUpQueue() { a.mu.Lock(); a.followUpQueue.clear(); a.mu.Unlock() }

// ClearAllQueues removes all queued messages.
func (a *Agent) ClearAllQueues() {
	a.mu.Lock()
	a.steeringQueue.clear()
	a.followUpQueue.clear()
	a.mu.Unlock()
}

// HasQueuedMessages reports whether either queue has pending messages.
func (a *Agent) HasQueuedMessages() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.steeringQueue.hasItems() || a.followUpQueue.hasItems()
}

// PeekQueuedMessages previews the messages selected for the next turn without
// consuming them.
func (a *Agent) PeekQueuedMessages() []model.AgentMessage {
	a.mu.Lock()
	defer a.mu.Unlock()
	if steering := a.steeringQueue.peek(); len(steering) > 0 {
		return steering
	}
	return a.followUpQueue.peek()
}

// Abort cancels the current run, if any.
func (a *Agent) Abort() {
	a.mu.Lock()
	run := a.active
	a.mu.Unlock()
	if run != nil {
		run.cancel()
	}
}

// WaitForIdle blocks until the current run and its listeners finish.
func (a *Agent) WaitForIdle() {
	a.mu.Lock()
	run := a.active
	a.mu.Unlock()
	if run != nil {
		<-run.done
	}
}

// Reset clears conversation state, runtime state and queued messages while
// retaining the replayed prompt/tool baseline. It refuses while a run is
// active.
func (a *Agent) Reset() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.active != nil {
		return errors.New("Agent is already processing. Wait for completion before resetting.") //nolint:staticcheck // pi's exact error message
	}
	baseline, ok := model.GetCurrentSystemMessage(a.state.Messages)
	a.state.Messages = nil
	if ok {
		a.state.Messages = []model.AgentMessage{baseline}
	}
	a.state.IsStreaming = false
	a.state.StreamingMessage = nil
	a.state.PendingToolCalls = map[string]bool{}
	a.state.ErrorMessage = ""
	a.steeringQueue.clear()
	a.followUpQueue.clear()
	return nil
}

// Prompt starts a new run from text. Blocks until the run completes.
func (a *Agent) Prompt(ctx context.Context, text string, images ...model.ImageContent) error {
	content := model.ContentList{model.TextContent{Text: text}}
	for _, img := range images {
		content = append(content, img)
	}
	return a.PromptMessages(ctx, []model.AgentMessage{model.UserMessage{Content: content, Timestamp: timeNowMillis()}})
}

// PromptMessages starts a new run from explicit messages.
func (a *Agent) PromptMessages(ctx context.Context, messages []model.AgentMessage) error {
	a.mu.Lock()
	if a.active != nil {
		a.mu.Unlock()
		return errors.New("Agent is already processing a prompt. Use Steer() or FollowUp() to queue messages, or wait for completion.") //nolint:staticcheck // pi's exact error message
	}
	a.mu.Unlock()
	return a.runPromptMessages(ctx, messages, false)
}

// Continue continues from the current transcript. The last message must be a
// user or tool-result message, or queued messages must exist.
func (a *Agent) Continue(ctx context.Context) error {
	a.mu.Lock()
	if a.active != nil {
		a.mu.Unlock()
		return errors.New("Agent is already processing. Wait for completion before continuing.") //nolint:staticcheck // pi's exact error message
	}
	if !slices.ContainsFunc(a.state.Messages, func(m model.AgentMessage) bool { return m.MessageRole() != model.RoleSystem }) {
		a.mu.Unlock()
		return errors.New("No messages to continue from") //nolint:staticcheck // pi's exact error message
	}
	last := a.state.Messages[len(a.state.Messages)-1]
	a.mu.Unlock()

	if last.MessageRole() == model.RoleAssistant {
		var drained []model.AgentMessage
		skipInitialSteeringPoll := false
		run, err := a.claimRun(ctx, func() {
			if steering := a.steeringQueue.drain(); len(steering) > 0 {
				drained = steering
				skipInitialSteeringPoll = true
				return
			}
			drained = a.followUpQueue.drain()
		})
		if err != nil {
			return errors.New("Agent is already processing. Wait for completion before continuing.") //nolint:staticcheck // pi's exact error message
		}
		if len(drained) == 0 {
			a.releaseRun(run)
			return errors.New("Cannot continue from message role: assistant") //nolint:staticcheck // pi's exact error message
		}
		return a.executeClaimedRun(run, func(runCtx context.Context) error {
			_, err := RunAgentLoop(runCtx, drained, a.contextSnapshot(), a.loopConfig(skipInitialSteeringPoll), a.processEvent(runCtx), a.streamFn)
			return err
		})
	}
	return a.runContinuation(ctx)
}

func (a *Agent) runPromptMessages(parent context.Context, messages []model.AgentMessage, skipInitialSteeringPoll bool) error {
	return a.runWithLifecycle(parent, func(ctx context.Context) error {
		_, err := RunAgentLoop(ctx, messages, a.contextSnapshot(), a.loopConfig(skipInitialSteeringPoll), a.processEvent(ctx), a.streamFn)
		return err
	})
}

func (a *Agent) runContinuation(parent context.Context) error {
	return a.runWithLifecycle(parent, func(ctx context.Context) error {
		_, err := RunAgentLoopContinue(ctx, a.contextSnapshot(), a.loopConfig(false), a.processEvent(ctx), a.streamFn)
		return err
	})
}

// handleRunFailure synthesizes a terminal assistant message and emits the full
// failure sequence so the lifecycle is always complete.
func (a *Agent) handleRunFailure(ctx context.Context, message string, wasAborted bool) error {
	a.mu.Lock()
	m := a.state.Model
	a.mu.Unlock()

	stop := model.StopError
	if wasAborted {
		stop = model.StopAborted
	}
	failure := &model.AssistantMessage{
		Content:      model.ContentList{model.TextContent{Text: ""}},
		Api:          m.Api,
		Provider:     m.Provider,
		Model:        m.ID,
		StopReason:   stop,
		ErrorMessage: message,
		Timestamp:    timeNowMillis(),
	}
	emit := a.processEvent(ctx)
	for _, event := range []model.AgentEvent{
		{Type: model.EvMessageStart, Message: failure},
		{Type: model.EvMessageEnd, Message: failure},
		{Type: model.EvTurnEnd, Message: failure, ToolResults: []model.ToolResultMessage{}},
		{Type: model.EvAgentEnd, Messages: []model.AgentMessage{failure}},
	} {
		if err := emit(event); err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) contextSnapshot() model.AgentContext {
	a.mu.Lock()
	defer a.mu.Unlock()
	return model.AgentContext{
		Messages: append([]model.AgentMessage(nil), a.state.Messages...),
		Tools:    append([]model.AgentTool(nil), a.state.Tools...),
	}
}

func (a *Agent) loopConfig(skipInitialSteeringPoll bool) model.AgentLoopConfig {
	a.mu.Lock()
	m := a.state.Model
	reasoning := a.state.ThinkingLevel
	simpleOptions := a.simpleOptions
	a.mu.Unlock()

	skip := skipInitialSteeringPoll
	config := model.AgentLoopConfig{
		Model:               m,
		Reasoning:           reasoning,
		SimpleStreamOptions: simpleOptions,
		ToolExecution:       a.toolExecution,
		ConvertToLlm:        a.convertToLlm,
		TransformContext:    a.transformContext,
		GetApiKey:           a.getApiKey,
		BeforeToolCall:      a.beforeToolCall,
		AfterToolCall:       a.afterToolCall,
		FinishTurn:          a.finishTurn,
		PrepareRequest:      a.prepareRequest,
		PrepareNextTurn:     a.prepareNextTurn,
		GetSteeringMessages: func() []model.AgentMessage {
			a.mu.Lock()
			defer a.mu.Unlock()
			if skip {
				skip = false
				return nil
			}
			return a.steeringQueue.drain()
		},
		GetFollowUpMessages: func() []model.AgentMessage {
			a.mu.Lock()
			defer a.mu.Unlock()
			return a.followUpQueue.drain()
		},
	}
	if reasoning == model.ThinkingOff {
		config.Reasoning = ""
	}
	return config
}

func (a *Agent) runWithLifecycle(parent context.Context, executor func(ctx context.Context) error) error {
	run, err := a.claimRun(parent, nil)
	if err != nil {
		return err
	}
	return a.executeClaimedRun(run, executor)
}

// claimRun atomically claims the run slot. onClaimed runs under the same lock
// immediately after a successful claim.
func (a *Agent) claimRun(parent context.Context, onClaimed func()) (*activeRun, error) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	run := &activeRun{cancel: cancel, ctx: ctx, done: make(chan struct{})}

	a.mu.Lock()
	if a.active != nil {
		a.mu.Unlock()
		cancel()
		return nil, errors.New("Agent is already processing.") //nolint:staticcheck // pi's exact error message
	}
	a.active = run
	if onClaimed != nil {
		onClaimed()
	}
	a.mu.Unlock()
	return run, nil
}

// releaseRun abandons a claimed run that never executed.
func (a *Agent) releaseRun(run *activeRun) {
	a.mu.Lock()
	a.active = nil
	a.mu.Unlock()
	run.cancel()
	close(run.done)
}

func (a *Agent) executeClaimedRun(run *activeRun, executor func(ctx context.Context) error) error {
	ctx := run.ctx

	a.mu.Lock()
	a.state.IsStreaming = true
	a.state.StreamingMessage = nil
	a.state.ErrorMessage = ""
	a.mu.Unlock()

	var runErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				message := panicMessage(r)
				if ep, ok := r.(emitPanic); ok {
					message = ep.err.Error()
				}
				runErr = a.handleRunFailure(ctx, message, ctx.Err() != nil)
			}
		}()
		if err := executor(ctx); err != nil {
			runErr = a.handleRunFailure(ctx, err.Error(), ctx.Err() != nil)
		}
	}()

	a.mu.Lock()
	a.state.IsStreaming = false
	a.state.StreamingMessage = nil
	a.state.PendingToolCalls = map[string]bool{}
	a.active = nil
	a.mu.Unlock()
	run.cancel()
	close(run.done)
	return runErr
}

// processEvent reduces internal state for a loop event, then notifies listeners.
func (a *Agent) processEvent(ctx context.Context) model.EventSink {
	return func(event model.AgentEvent) error {
		a.mu.Lock()
		switch event.Type {
		case model.EvMessageStart, model.EvMessageUpdate:
			a.state.StreamingMessage = event.Message
		case model.EvMessageEnd:
			a.state.StreamingMessage = nil
			a.state.Messages = append(a.state.Messages, event.Message)
		case model.EvToolExecutionStart:
			next := make(map[string]bool, len(a.state.PendingToolCalls)+1)
			maps.Copy(next, a.state.PendingToolCalls)
			next[event.ToolCallID] = true
			a.state.PendingToolCalls = next
		case model.EvToolExecutionEnd:
			next := make(map[string]bool, len(a.state.PendingToolCalls))
			for k, v := range a.state.PendingToolCalls {
				if k != event.ToolCallID {
					next[k] = v
				}
			}
			a.state.PendingToolCalls = next
		case model.EvTurnEnd:
			if assistant, ok := asAssistant(event.Message); ok && assistant.ErrorMessage != "" {
				a.state.ErrorMessage = assistant.ErrorMessage
			}
		case model.EvAgentEnd:
			a.state.StreamingMessage = nil
		}
		listeners := append([]Listener(nil), a.listeners...)
		a.mu.Unlock()

		for _, listener := range listeners {
			if listener == nil {
				continue
			}
			if err := listener(ctx, event); err != nil {
				return err
			}
		}
		return nil
	}
}

func asAssistant(message model.AgentMessage) (*model.AssistantMessage, bool) {
	switch v := message.(type) {
	case *model.AssistantMessage:
		return v, true
	case model.AssistantMessage:
		return &v, true
	default:
		return nil, false
	}
}
