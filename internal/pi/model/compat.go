package model

// ChatTemplateVar is the pi-controlled thinking value placeholder
// (`{"$var": "thinking.enabled"}` and friends).
type ChatTemplateVar struct {
	Var         string `json:"$var"`
	OmitWhenOff bool   `json:"omitWhenOff,omitempty"`
}

// ChatTemplateKwargValue is a value sent in chat_template_kwargs or
// chat_template_args.
type ChatTemplateKwargValue = any

// OpenAICompletionsCompat holds compatibility settings for OpenAI-compatible
// completions APIs.
type OpenAICompletionsCompat struct {
	SupportsStore                               *bool  `json:"supportsStore,omitempty"`
	SupportsDeveloperRole                       *bool  `json:"supportsDeveloperRole,omitempty"`
	SupportsReasoningEffort                     *bool  `json:"supportsReasoningEffort,omitempty"`
	SupportsUsageInStreaming                    *bool  `json:"supportsUsageInStreaming,omitempty"`
	SupportsFinishReason                        *bool  `json:"supportsFinishReason,omitempty"`
	MaxTokensField                              string `json:"maxTokensField,omitempty"`
	RequiresToolResultName                      *bool  `json:"requiresToolResultName,omitempty"`
	RequiresAssistantAfterToolResult            *bool  `json:"requiresAssistantAfterToolResult,omitempty"`
	RequiresThinkingAsText                      *bool  `json:"requiresThinkingAsText,omitempty"`
	RequiresReasoningContentOnAssistantMessages *bool  `json:"requiresReasoningContentOnAssistantMessages,omitempty"`
	// ThinkingFormat is one of "openai", "openrouter", "deepseek", "together",
	// "baseten", "zai", "qwen", "chat-template", "qwen-chat-template",
	// "string-thinking", "ant-ling".
	ThinkingFormat                 string                            `json:"thinkingFormat,omitempty"`
	ChatTemplateKwargs             map[string]ChatTemplateKwargValue `json:"chatTemplateKwargs,omitempty"`
	ChatTemplateArgs               map[string]ChatTemplateKwargValue `json:"chatTemplateArgs,omitempty"`
	OpenRouterRouting              *OpenRouterRouting                `json:"openRouterRouting,omitempty"`
	VercelGatewayRouting           *VercelGatewayRouting             `json:"vercelGatewayRouting,omitempty"`
	ZaiToolStream                  *bool                             `json:"zaiToolStream,omitempty"`
	ThinkingTokenBudgetField       ThinkingTokenBudgetField          `json:"thinkingTokenBudgetField,omitempty"`
	SupportsThinkingTokenBudget    *bool                             `json:"supportsThinkingTokenBudget,omitempty"`
	SupportsOpenAIGrammarTools     *bool                             `json:"supportsOpenAIGrammarTools,omitempty"`
	SupportsMidConvoSystemMessages *bool                             `json:"supportsMidConvoSystemMessages,omitempty"`
	SupportsMidConvoToolAdditions  *bool                             `json:"supportsMidConvoToolAdditions,omitempty"`
	SupportsStrictMode             *bool                             `json:"supportsStrictMode,omitempty"`
	CacheControlFormat             string                            `json:"cacheControlFormat,omitempty"`
	SendSessionAffinityHeaders     *bool                             `json:"sendSessionAffinityHeaders,omitempty"`
	SessionAffinityFormat          SessionAffinityFormat             `json:"sessionAffinityFormat,omitempty"`
	SupportsLongCacheRetention     *bool                             `json:"supportsLongCacheRetention,omitempty"`
	VLLMPriority                   *int                              `json:"vllmPriority,omitempty"`
}

// OpenAIResponsesCompat holds compatibility settings for OpenAI Responses APIs.
type OpenAIResponsesCompat struct {
	SupportsDeveloperRole           *bool                 `json:"supportsDeveloperRole,omitempty"`
	SupportsMidConvoSystemMessages  *bool                 `json:"supportsMidConvoSystemMessages,omitempty"`
	SessionAffinityFormat           SessionAffinityFormat `json:"sessionAffinityFormat,omitempty"`
	SupportsLongCacheRetention      *bool                 `json:"supportsLongCacheRetention,omitempty"`
	SupportsStrictMode              *bool                 `json:"supportsStrictMode,omitempty"`
	SupportsOpenAIGrammarTools      *bool                 `json:"supportsOpenAIGrammarTools,omitempty"`
	SupportsAdditionalTools         *bool                 `json:"supportsAdditionalTools,omitempty"`
	SupportsToolSearch              *bool                 `json:"supportsToolSearch,omitempty"`
	SupportsExplicitPromptCacheMode *bool                 `json:"supportsExplicitPromptCacheMode,omitempty"`
	SupportsMaxOutputTokens         *bool                 `json:"supportsMaxOutputTokens,omitempty"`
}

// AnthropicAllowedFallbackModel is a fallback model Anthropic may use after a
// refusal.
type AnthropicAllowedFallbackModel struct {
	Provider ProviderId `json:"provider"`
	Model    string     `json:"model"`
	Cost     ModelCost  `json:"cost"`
}

// AnthropicMessagesCompat holds compatibility settings for Anthropic
// Messages-compatible APIs.
type AnthropicMessagesCompat struct {
	SupportsEagerToolInputStreaming *bool                           `json:"supportsEagerToolInputStreaming,omitempty"`
	SupportsLongCacheRetention      *bool                           `json:"supportsLongCacheRetention,omitempty"`
	SendSessionAffinityHeaders      *bool                           `json:"sendSessionAffinityHeaders,omitempty"`
	SessionAffinityFormat           string                          `json:"sessionAffinityFormat,omitempty"`
	SupportsCacheControlOnTools     *bool                           `json:"supportsCacheControlOnTools,omitempty"`
	SupportsTemperature             *bool                           `json:"supportsTemperature,omitempty"`
	ForceAdaptiveThinking           *bool                           `json:"forceAdaptiveThinking,omitempty"`
	AllowEmptySignature             *bool                           `json:"allowEmptySignature,omitempty"`
	SupportsStrictTools             *bool                           `json:"supportsStrictTools,omitempty"`
	SupportsMidConvoEffort          *bool                           `json:"supportsMidConvoEffort,omitempty"`
	SupportsMidConvoSystemMessages  *bool                           `json:"supportsMidConvoSystemMessages,omitempty"`
	SupportsMidConvoToolChanges     *bool                           `json:"supportsMidConvoToolChanges,omitempty"`
	AllowedFallbackModels           []AnthropicAllowedFallbackModel `json:"allowedFallbackModels,omitempty"`
}

// BedrockCompat holds compatibility settings for Amazon Bedrock models.
type BedrockCompat struct {
	SupportsStrictMode *bool `json:"supportsStrictMode,omitempty"`
}

// MistralConversationsCompat holds compatibility settings for the Mistral chat
// API.
type MistralConversationsCompat struct {
	SupportsMidConvoSystemMessages *bool `json:"supportsMidConvoSystemMessages,omitempty"`
}

// OpenRouterRouting holds OpenRouter provider routing preferences.
type OpenRouterRouting struct {
	AllowFallbacks         *bool          `json:"allow_fallbacks,omitempty"`
	RequireParameters      *bool          `json:"require_parameters,omitempty"`
	DataCollection         string         `json:"data_collection,omitempty"`
	ZDR                    *bool          `json:"zdr,omitempty"`
	EnforceDistillableText *bool          `json:"enforce_distillable_text,omitempty"`
	Order                  []string       `json:"order,omitempty"`
	Only                   []string       `json:"only,omitempty"`
	Ignore                 []string       `json:"ignore,omitempty"`
	Quantizations          []string       `json:"quantizations,omitempty"`
	Sort                   any            `json:"sort,omitempty"`
	MaxPrice               map[string]any `json:"max_price,omitempty"`
	PreferredMinThroughput any            `json:"preferred_min_throughput,omitempty"`
	PreferredMaxLatency    any            `json:"preferred_max_latency,omitempty"`
}

// VercelGatewayRouting holds Vercel AI Gateway routing preferences.
type VercelGatewayRouting struct {
	Only  []string `json:"only,omitempty"`
	Order []string `json:"order,omitempty"`
}
