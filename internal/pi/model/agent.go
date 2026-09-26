package model

import "context"

// AgentMessage is a message in the agent transcript. The four message types
// satisfy Message; apps may add custom UI-only types that also implement
// Message and are filtered out by ConvertToLlm.
type AgentMessage = Message

// ToolExecutionMode controls how a batch of tool calls is executed.
type ToolExecutionMode string

const (
	// ToolSequential prepares, executes and finalizes each call before the next.
	ToolSequential ToolExecutionMode = "sequential"
	// ToolParallel prepares calls sequentially, then runs allowed tools
	// concurrently.
	ToolParallel ToolExecutionMode = "parallel"
	// ToolDefault defers to the loop-level default.
	ToolDefault ToolExecutionMode = ""
)

// QueueMode controls how many queued messages drain at a drain point.
type QueueMode string

const (
	QueueAll        QueueMode = "all"
	QueueOneAtATime QueueMode = "one-at-a-time"
)

// AgentToolCall is a tool call content block emitted by an assistant message.
type AgentToolCall = ToolCall

// BeforeToolCallResult is returned from BeforeToolCall. Returning Block true
// prevents the tool from executing.
type BeforeToolCallResult struct {
	Block  bool
	Reason string
	// Terminate hints that the agent should stop after the current tool batch
	// when this call is blocked.
	Terminate bool
}

// AfterToolCallResult overrides parts of a finalized tool result. A nil field
// keeps the original value; there is no deep merge.
type AfterToolCallResult struct {
	Content    ContentList
	HasContent bool
	Details    any
	HasDetails bool
	IsError    *bool
	Usage      *Usage
	Terminate  *bool
}

// BeforeToolCallContext is passed to BeforeToolCall.
type BeforeToolCallContext struct {
	AssistantMessage *AssistantMessage
	ToolCall         AgentToolCall
	Args             map[string]any
	Context          *AgentContext
}

// AfterToolCallContext is passed to AfterToolCall.
type AfterToolCallContext struct {
	AssistantMessage *AssistantMessage
	ToolCall         AgentToolCall
	Args             map[string]any
	Result           AgentToolResult
	IsError          bool
	Context          *AgentContext
}

// AgentTurnContext describes a turn that has just completed.
type AgentTurnContext struct {
	Message     *AssistantMessage
	ToolResults []ToolResultMessage
	Context     *AgentContext
	NewMessages []AgentMessage
}

// AgentTurnDecision is returned by FinishTurn. The zero value preserves normal
// scheduling.
type AgentTurnDecision string

const (
	TurnContinue AgentTurnDecision = "continue"
	TurnEnd      AgentTurnDecision = "end"
)

// FinishTurnFunc is called after a completed assistant turn and all of its
// tool-result messages, but before turn_end.
type FinishTurnFunc func(ctx context.Context, turn AgentTurnContext) AgentTurnDecision

// AgentLoopTurnUpdate replaces runtime state before the next provider request.
type AgentLoopTurnUpdate struct {
	Context       *AgentContext
	Messages      []AgentMessage
	Model         *Model
	ThinkingLevel *ThinkingLevel
}

// PrepareRequestContext is the runtime state available immediately before a
// conversational provider request.
type PrepareRequestContext struct {
	Context       *AgentContext
	Model         *Model
	ThinkingLevel ThinkingLevel
}

// AgentRequestUpdate replaces runtime state for the provider request being
// prepared.
type AgentRequestUpdate struct {
	Context       *AgentContext
	Model         *Model
	ThinkingLevel *ThinkingLevel
}

// PrepareRequestFunc is called immediately before every conversational
// provider request, including the first.
type PrepareRequestFunc func(ctx context.Context, request PrepareRequestContext) *AgentRequestUpdate

// AgentLoopConfig configures a single agent loop run.
type AgentLoopConfig struct {
	Model     *Model
	Reasoning ThinkingLevel

	SimpleStreamOptions

	ToolExecution ToolExecutionMode

	// ConvertToLlm maps the agent transcript to provider messages before each
	// call.
	ConvertToLlm func(messages []AgentMessage) []Message
	// TransformContext optionally rewrites the transcript before ConvertToLlm.
	TransformContext func(ctx context.Context, messages []AgentMessage) []AgentMessage
	// GetApiKey resolves an API key per call.
	GetApiKey func(provider string) string

	BeforeToolCall func(ctx context.Context, c BeforeToolCallContext) *BeforeToolCallResult
	AfterToolCall  func(ctx context.Context, c AfterToolCallContext) *AfterToolCallResult
	// FinishTurn is called after the assistant message and all tool-result
	// messages have been emitted, immediately before turn_end.
	FinishTurn FinishTurnFunc
	// PrepareRequest is called immediately before every conversational provider
	// request, including the first.
	PrepareRequest PrepareRequestFunc
	// PrepareNextTurn is called after turn_end only when the loop will
	// continue, immediately before the next turn starts.
	PrepareNextTurn func(c AgentTurnContext) *AgentLoopTurnUpdate
	// GetSteeringMessages returns steering messages to inject mid-run.
	GetSteeringMessages func() []AgentMessage
	// GetFollowUpMessages returns follow-up messages to process after the agent
	// would otherwise stop.
	GetFollowUpMessages func() []AgentMessage
}

// AgentState is the public agent state.
type AgentState struct {
	// SystemPrompt is the current prompt, replayed from the transcript's system
	// messages.
	SystemPrompt  string
	Model         *Model
	ThinkingLevel ThinkingLevel
	Tools         []AgentTool
	Messages      []AgentMessage
	IsStreaming   bool
	// StreamingMessage is the partial assistant message for the current
	// streamed response, if any.
	StreamingMessage AgentMessage
	// PendingToolCalls are the tool call ids currently executing.
	PendingToolCalls map[string]bool
	ErrorMessage     string
}

// AgentToolResult is the final or partial result produced by a tool.
type AgentToolResult struct {
	Content   ContentList
	Details   any
	Usage     *Usage
	Terminate bool
}

// ToolUpdateFunc streams partial tool results during execution.
type ToolUpdateFunc func(partial AgentToolResult)

// AgentTool is a tool available to the agent: a tool definition plus a UI label
// and an execute function.
type AgentTool struct {
	Name        string
	Description string
	Parameters  *Schema
	// Label is a human-readable name for UI display.
	Label string
	// ExecutionMode optionally overrides the loop default for this tool.
	ExecutionMode ToolExecutionMode
	// PrepareArguments is an optional shim applied to raw arguments before
	// schema validation.
	PrepareArguments func(raw map[string]any) map[string]any
	// ConstrainedSampling optionally asks the provider to constrain sampling.
	ConstrainedSampling *ConstrainedSamplingConfig
	// Replay is the recovery policy for an effect whose outcome is unknown.
	Replay string
	// Execute runs the tool. Return an error on failure rather than encoding
	// errors in Content.
	Execute func(ctx context.Context, toolCallID string, params map[string]any, onUpdate ToolUpdateFunc) (AgentToolResult, error)
}

// AsTool returns the ai.Tool definition used for schema validation and the
// provider request.
func (t AgentTool) AsTool() Tool {
	return Tool{
		Name:                t.Name,
		Description:         t.Description,
		Parameters:          t.Parameters,
		ConstrainedSampling: t.ConstrainedSampling,
	}
}

// AgentContext is the snapshot passed into the low-level loop.
type AgentContext struct {
	Messages []AgentMessage
	Tools    []AgentTool
}

// Clone returns a deep copy of the context.
func (c AgentContext) Clone() AgentContext {
	out := c
	out.Messages = CloneMessages(c.Messages)
	out.Tools = append([]AgentTool(nil), c.Tools...)
	for i := range out.Tools {
		definition := c.Tools[i].AsTool().Clone()
		out.Tools[i].Parameters = definition.Parameters
		out.Tools[i].ConstrainedSampling = definition.ConstrainedSampling
	}
	return out
}

// AgentEventType is the discriminator for AgentEvent.
type AgentEventType string

const (
	EvAgentStart          AgentEventType = "agent_start"
	EvAgentEnd            AgentEventType = "agent_end"
	EvTurnStart           AgentEventType = "turn_start"
	EvTurnEnd             AgentEventType = "turn_end"
	EvMessageStart        AgentEventType = "message_start"
	EvMessageUpdate       AgentEventType = "message_update"
	EvMessageEnd          AgentEventType = "message_end"
	EvToolExecutionStart  AgentEventType = "tool_execution_start"
	EvToolExecutionUpdate AgentEventType = "tool_execution_update"
	EvToolExecutionEnd    AgentEventType = "tool_execution_end"
)

// AgentEvent is a lifecycle event emitted by the loop.
type AgentEvent struct {
	Type AgentEventType

	// AgentEnd: full new-message list for the run.
	Messages []AgentMessage
	// TurnEnd / Message*: the relevant message.
	Message AgentMessage
	// TurnEnd: tool results from the turn.
	ToolResults []ToolResultMessage
	// MessageUpdate: the underlying assistant stream event.
	AssistantMessageEvent *AssistantMessageEvent

	// Tool execution events.
	ToolCallID    string
	ToolName      string
	Args          map[string]any
	PartialResult any
	Result        any
	IsError       bool
}

// EventSink receives loop events. Returning an error aborts the loop.
type EventSink func(event AgentEvent) error
