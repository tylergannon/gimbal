package model

import "context"

// ContextUsage is the current context usage for an active model.
type ContextUsage struct {
	// Tokens is the estimated context tokens, or nil when unknown.
	Tokens *int `json:"tokens"`
	// ContextWindow is the model's context window.
	ContextWindow int `json:"contextWindow"`
	// Percent is the context usage as a percentage, or nil when Tokens is
	// unknown.
	Percent *float64 `json:"percent"`
}

// ToolDefinition is a tool definition registered by an app or extension. It is
// the definition-first form of an AgentTool.
type ToolDefinition struct {
	Name        string
	Label       string
	Description string
	// PromptSnippet is an optional one-line snippet for the system prompt.
	PromptSnippet string
	// PromptGuidelines are optional guideline bullets for the system prompt.
	PromptGuidelines []string
	Parameters       *Schema
	// ConstrainedSampling optionally asks the provider to constrain sampling.
	ConstrainedSampling *ConstrainedSamplingConfig
	// PrepareArguments is an optional shim applied before schema validation.
	PrepareArguments func(raw map[string]any) map[string]any
	ExecutionMode    ToolExecutionMode
	Execute          func(ctx context.Context, toolCallID string, params map[string]any, onUpdate ToolUpdateFunc) (AgentToolResult, error)
}

// SlashCommandInfo describes a registered slash command.
type SlashCommandInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	Arguments   string   `json:"arguments,omitempty"`
}

// ExtensionEvent is one event delivered to extension hooks. Apps implement it
// with the concrete event types below.
type ExtensionEvent interface {
	EventType() string
}

// Session event types.
type SessionStartEvent struct {
	Reason              string `json:"reason"`
	PreviousSessionFile string `json:"previousSessionFile,omitempty"`
}

func (SessionStartEvent) EventType() string { return "session_start" }

type SessionInfoChangedEvent struct {
	Name *string `json:"name"`
}

func (SessionInfoChangedEvent) EventType() string { return "session_info_changed" }

type SessionBeforeSwitchEvent struct {
	Reason            string `json:"reason"`
	TargetSessionFile string `json:"targetSessionFile,omitempty"`
}

func (SessionBeforeSwitchEvent) EventType() string { return "session_before_switch" }

type SessionBeforeForkEvent struct {
	EntryID  string `json:"entryId"`
	Position string `json:"position"`
}

func (SessionBeforeForkEvent) EventType() string { return "session_before_fork" }

type SessionBeforeCompactEvent struct {
	BranchEntries      []SessionEntry `json:"branchEntries"`
	CustomInstructions string         `json:"customInstructions,omitempty"`
	Reason             string         `json:"reason"`
	WillRetry          bool           `json:"willRetry"`
}

func (SessionBeforeCompactEvent) EventType() string { return "session_before_compact" }

type SessionCompactEvent struct {
	CompactionEntry CompactionEntry `json:"compactionEntry"`
	FromExtension   bool            `json:"fromExtension"`
	Reason          string          `json:"reason"`
	WillRetry       bool            `json:"willRetry"`
}

func (SessionCompactEvent) EventType() string { return "session_compact" }

type SessionCompactFailedEvent struct {
	Reason        string `json:"reason"`
	ErrorMessage  string `json:"errorMessage,omitempty"`
	Aborted       bool   `json:"aborted"`
	WillRetry     bool   `json:"willRetry"`
	FromExtension bool   `json:"fromExtension"`
}

func (SessionCompactFailedEvent) EventType() string { return "session_compact_failed" }

type SessionShutdownEvent struct {
	Reason            string `json:"reason"`
	TargetSessionFile string `json:"targetSessionFile,omitempty"`
}

func (SessionShutdownEvent) EventType() string { return "session_shutdown" }

type SessionBeforeTreeEvent struct {
	Preparation TreePreparation `json:"preparation"`
}

func (SessionBeforeTreeEvent) EventType() string { return "session_before_tree" }

type SessionTreeEvent struct {
	NewLeafID     *string             `json:"newLeafId"`
	OldLeafID     *string             `json:"oldLeafId"`
	SummaryEntry  *BranchSummaryEntry `json:"summaryEntry,omitempty"`
	FromExtension bool                `json:"fromExtension,omitempty"`
}

func (SessionTreeEvent) EventType() string { return "session_tree" }

// TreePreparation is preparation data for tree navigation.
type TreePreparation struct {
	TargetID            string         `json:"targetId"`
	OldLeafID           *string        `json:"oldLeafId"`
	CommonAncestorID    *string        `json:"commonAncestorId"`
	EntriesToSummarize  []SessionEntry `json:"entriesToSummarize"`
	UserWantsSummary    bool           `json:"userWantsSummary"`
	CustomInstructions  string         `json:"customInstructions,omitempty"`
	ReplaceInstructions bool           `json:"replaceInstructions,omitempty"`
	Label               *string        `json:"label,omitempty"`
}

// Agent event types.
type ContextEvent struct {
	Messages []AgentMessage `json:"messages"`
}

func (ContextEvent) EventType() string { return "context" }

type ContextWithSystemEvent struct {
	Messages []AgentMessage `json:"messages"`
}

func (ContextWithSystemEvent) EventType() string { return "context_with_system" }

type BeforeProviderRequestEvent struct {
	Payload any `json:"payload"`
}

func (BeforeProviderRequestEvent) EventType() string { return "before_provider_request" }

type BeforeProviderHeadersEvent struct {
	Headers ProviderHeaders `json:"headers"`
}

func (BeforeProviderHeadersEvent) EventType() string { return "before_provider_headers" }

type AfterProviderResponseEvent struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
}

func (AfterProviderResponseEvent) EventType() string { return "after_provider_response" }

type ProviderStreamEvent struct {
	Provider ProviderId `json:"provider"`
	Api      Api        `json:"api"`
	Model    string     `json:"model"`
	Data     any        `json:"data"`
}

func (ProviderStreamEvent) EventType() string { return "provider_stream_event" }

type BeforeAgentStartEvent struct {
	Prompt       string         `json:"prompt"`
	Images       []ImageContent `json:"images,omitempty"`
	SystemPrompt string         `json:"systemPrompt"`
}

func (BeforeAgentStartEvent) EventType() string { return "before_agent_start" }

type AgentStartEvent struct{}

func (AgentStartEvent) EventType() string { return "agent_start" }

type AgentEndEvent struct {
	Messages []AgentMessage `json:"messages"`
}

func (AgentEndEvent) EventType() string { return "agent_end" }

type TurnStartEvent struct {
	TurnIndex int   `json:"turnIndex"`
	Timestamp int64 `json:"timestamp"`
}

func (TurnStartEvent) EventType() string { return "turn_start" }

type TurnEndEvent struct {
	TurnIndex          int                 `json:"turnIndex"`
	Message            AgentMessage        `json:"message"`
	ToolResults        []ToolResultMessage `json:"toolResults"`
	MessageEntryID     string              `json:"messageEntryId"`
	ToolResultEntryIDs []string            `json:"toolResultEntryIds"`
}

func (TurnEndEvent) EventType() string { return "turn_end" }

type MessageStartEvent struct {
	Message AgentMessage `json:"message"`
}

func (MessageStartEvent) EventType() string { return "message_start" }

type MessageUpdateEvent struct {
	Message               AgentMessage          `json:"message"`
	AssistantMessageEvent AssistantMessageEvent `json:"assistantMessageEvent"`
}

func (MessageUpdateEvent) EventType() string { return "message_update" }

type MessageEndEvent struct {
	Message AgentMessage `json:"message"`
}

func (MessageEndEvent) EventType() string { return "message_end" }

type ToolExecutionStartEvent struct {
	ToolCallID string `json:"toolCallId"`
	ToolName   string `json:"toolName"`
	Args       any    `json:"args"`
}

func (ToolExecutionStartEvent) EventType() string { return "tool_execution_start" }

type ToolExecutionUpdateEvent struct {
	ToolCallID    string `json:"toolCallId"`
	ToolName      string `json:"toolName"`
	Args          any    `json:"args"`
	PartialResult any    `json:"partialResult"`
}

func (ToolExecutionUpdateEvent) EventType() string { return "tool_execution_update" }

type ToolExecutionEndEvent struct {
	ToolCallID string `json:"toolCallId"`
	ToolName   string `json:"toolName"`
	Result     any    `json:"result"`
	IsError    bool   `json:"isError"`
}

func (ToolExecutionEndEvent) EventType() string { return "tool_execution_end" }

// AgentActivityOutcome is the outcome of an agent run.
type AgentActivityOutcome string

const (
	OutcomeCompleted AgentActivityOutcome = "completed"
	OutcomeAborted   AgentActivityOutcome = "aborted"
	OutcomeError     AgentActivityOutcome = "error"
)
