package model

import "time"

// Api identifies a wire protocol / API shape (pi KnownApi plus any custom
// string).
type Api = string

// Known Api values.
const (
	APIOpenAICompletions     Api = "openai-completions"
	APIMistralConversations  Api = "mistral-conversations"
	APIOpenAIResponses       Api = "openai-responses"
	APIAzureOpenAIResponses  Api = "azure-openai-responses"
	APIOpenAICodexResponses  Api = "openai-codex-responses"
	APIAnthropicMessages     Api = "anthropic-messages"
	APIBedrockConverseStream Api = "bedrock-converse-stream"
	APIGoogleGenerativeAI    Api = "google-generative-ai"
	APIGoogleVertex          Api = "google-vertex"
	APIPiMessages            Api = "pi-messages"
)

// ImageApi identifies an image-generation API.
type ImageApi = string

// KnownImageAPI is the one known image API.
const KnownImageAPI ImageApi = "openrouter-images"

// ClassifierApi identifies a classifier API.
type ClassifierApi = string

// Known classifier APIs.
const (
	ClassifierTypesafeSystemOne   ClassifierApi = "typesafe-system-one"
	ClassifierCloudflareSystemOne ClassifierApi = "cloudflare-workers-ai-system-one"
)

// ProviderId identifies a model provider (pi ProviderId).
type ProviderId = string

// Known provider ids.
const (
	ProviderAmazonBedrock        ProviderId = "amazon-bedrock"
	ProviderAntLing              ProviderId = "ant-ling"
	ProviderAnthropic            ProviderId = "anthropic"
	ProviderGoogle               ProviderId = "google"
	ProviderGoogleVertex         ProviderId = "google-vertex"
	ProviderOpenAI               ProviderId = "openai"
	ProviderAzureOpenAIResponses ProviderId = "azure-openai-responses"
	ProviderOpenAICodex          ProviderId = "openai-codex"
	ProviderRadius               ProviderId = "radius"
	ProviderTypesafe             ProviderId = "typesafe"
	ProviderDeepSeek             ProviderId = "deepseek"
	ProviderOpenRouter           ProviderId = "openrouter"
	ProviderMistral              ProviderId = "mistral"
)

// ToolChoice is the provider-neutral tool selection for simple requests. The
// empty value is pi's absent option.
type ToolChoice string

const (
	ToolChoiceAuto ToolChoice = "auto"
	ToolChoiceNone ToolChoice = "none"
)

// ThinkingLevel is a reasoning effort level understood by the unified API.
// "off" is the agent-runtime spelling that disables reasoning.
type ThinkingLevel string

const (
	ThinkingOff     ThinkingLevel = "off"
	ThinkingMinimal ThinkingLevel = "minimal"
	ThinkingLow     ThinkingLevel = "low"
	ThinkingMedium  ThinkingLevel = "medium"
	ThinkingHigh    ThinkingLevel = "high"
	ThinkingXHigh   ThinkingLevel = "xhigh"
	ThinkingMax     ThinkingLevel = "max"
)

// ModelThinkingLevel is pi's ModelThinkingLevel: ThinkingLevel plus "off".
type ModelThinkingLevel string

// ThinkingLevelMap maps pi thinking levels to provider/model-specific values.
// A nil pointer value marks a level as unsupported.
type ThinkingLevelMap map[ModelThinkingLevel]*string

// ThinkingBudgets holds token budgets per thinking level (token-based
// providers only).
type ThinkingBudgets struct {
	Minimal *int `json:"minimal,omitempty"`
	Low     *int `json:"low,omitempty"`
	Medium  *int `json:"medium,omitempty"`
	High    *int `json:"high,omitempty"`
}

// ThinkingTokenBudgetField names the top-level request field used to cap
// reasoning tokens on OpenAI-compatible servers.
type ThinkingTokenBudgetField string

const (
	ThinkingTokenBudgetVLLM     ThinkingTokenBudgetField = "thinking_token_budget"
	ThinkingTokenBudgetQwen     ThinkingTokenBudgetField = "thinking_budget"
	ThinkingTokenBudgetLlamaCPP ThinkingTokenBudgetField = "thinking_budget_tokens"
)

// CacheRetention is the prompt cache retention preference.
type CacheRetention string

const (
	CacheNone  CacheRetention = "none"
	CacheShort CacheRetention = "short"
	CacheLong  CacheRetention = "long"
)

// ModelPromptCache is the best-effort prompt cache lifetime in seconds for
// each retention tier. A nil tier means the lifetime is unknown.
type ModelPromptCache struct {
	Short *float64 `json:"short,omitempty"`
	Long  *float64 `json:"long,omitempty"`
}

// Transport is the preferred transport for providers that support several.
type Transport string

const (
	TransportSSE             Transport = "sse"
	TransportWebSocket       Transport = "websocket"
	TransportWebSocketCached Transport = "websocket-cached"
	TransportAuto            Transport = "auto"
)

// ProviderHeaders are custom HTTP headers for provider requests. A nil map
// entry is a deletion marker that suppresses a provider default.
type ProviderHeaders map[string]*string

// HeaderValue returns a present header value for a ProviderHeaders entry.
//
//go:fix inline
func HeaderValue(v string) *string { return new(v) }

// ProviderEnv holds provider-scoped environment overrides.
type ProviderEnv map[string]string

// ProviderResponse is the HTTP response summary passed to OnResponse.
type ProviderResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
}

// StopReason describes why an assistant turn ended.
type StopReason string

const (
	// StopPending marks a partial assistant message whose stream has not yet
	// reported a terminal reason.
	StopPending StopReason = "pending"
	StopStop    StopReason = "stop"
	StopLength  StopReason = "length"
	StopToolUse StopReason = "toolUse"
	StopError   StopReason = "error"
	StopAborted StopReason = "aborted"
	// StopDeferred marks a submission the provider accepted but has not
	// finished; the message carries a DeferredHandle instead of content.
	StopDeferred StopReason = "deferred"
)

// DeferredHandle is a durable reference to a response a provider accepted and
// is producing asynchronously.
type DeferredHandle struct {
	Provider    ProviderId `json:"provider"`
	ModelID     string     `json:"modelId"`
	Api         Api        `json:"api"`
	ID          string     `json:"id"`
	ExpiresAt   int64      `json:"expiresAt,omitempty"`
	PollAfterMs int64      `json:"pollAfterMs,omitempty"`
	// Data is provider conversion data needed to reconstruct the final
	// assistant message.
	Data any `json:"data,omitempty"`
}

// DeferredWindow is how long a provider should keep working on a deferred
// submission.
type DeferredWindow string

const (
	DeferredWindow15m DeferredWindow = "15m"
	DeferredWindow1h  DeferredWindow = "1h"
	DeferredWindow24h DeferredWindow = "24h"
)

// DeferredRequest asks a capable provider to return a DeferredHandle and carry
// on with the request asynchronously. A non-nil request with an empty Window
// is pi's `deferred: true`.
type DeferredRequest struct {
	Window DeferredWindow
}

// DeferredFetchOptions are the options for redeeming a DeferredHandle.
type DeferredFetchOptions struct {
	ProviderRequestOptions
	// Wait bounds the provider's long poll for a terminal response. nil leaves
	// the wait to the provider.
	Wait *time.Duration
}

// DeferredCancelOptions are the options for best-effort cancellation of a
// deferred response.
type DeferredCancelOptions = ProviderRequestOptions

// SessionAffinityFormat selects the session-affinity header shape.
type SessionAffinityFormat string

const (
	SessionAffinityOpenAI          SessionAffinityFormat = "openai"
	SessionAffinityOpenAINosession SessionAffinityFormat = "openai-nosession"
	SessionAffinityOpenRouter      SessionAffinityFormat = "openrouter"
)

// Role identifies a message author.
type Role string

const (
	RoleSystem     Role = "system"
	RoleUser       Role = "user"
	RoleAssistant  Role = "assistant"
	RoleToolResult Role = "toolResult"
)
