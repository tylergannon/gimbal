package openai

import (
	"encoding/json"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// Compatibility profile resolution, ported from openai-completions.ts
// detectCompat / getCompat (upstream d6af72e1).

// compat is the resolved compatibility profile for an OpenAI-compatible
// chat-completions provider (pi ResolvedOpenAICompletionsCompat).
type compat struct {
	SupportsStore                               bool
	SupportsDeveloperRole                       bool
	SupportsReasoningEffort                     bool
	SupportsUsageInStreaming                    bool
	SupportsFinishReason                        bool
	MaxTokensField                              string
	ThinkingFormat                              string
	SupportsStrictMode                          bool
	SupportsOpenAIGrammarTools                  bool
	SupportsMidConvoSystemMessages              bool
	SupportsMidConvoToolAdditions               bool
	SupportsLongCacheRetention                  bool
	RequiresReasoningContentOnAssistantMessages bool
	RequiresToolResultName                      bool
	RequiresAssistantAfterToolResult            bool
	RequiresThinkingAsText                      bool
	ZaiToolStream                               bool
	ThinkingTokenBudgetField                    string
	SupportsThinkingTokenBudget                 bool
	SendSessionAffinityHeaders                  bool
	SessionAffinityFormat                       model.SessionAffinityFormat
	CacheControlFormat                          string
	OpenRouterRouting                           *model.OpenRouterRouting
	HasOpenRouterRouting                        bool
	VercelGatewayRouting                        model.VercelGatewayRouting
	ChatTemplateKwargs                          map[string]model.ChatTemplateKwargValue
	ChatTemplateArgs                            map[string]model.ChatTemplateKwargValue
	VLLMPriority                                *int
	HasVLLMPriority                             bool
}

// detectCompat infers compatibility settings from provider and baseUrl, the
// base used when model.compat does not set a value (pi detectCompat).
func detectCompat(m *model.Model) compat {
	provider := m.Provider
	baseURL := m.BaseURL
	has := func(s string) bool { return strings.Contains(baseURL, s) }

	isZai := provider == "zai" || provider == "zai-coding-cn" || has("api.z.ai") || has("open.bigmodel.cn")
	isTogether := provider == "together" || has("api.together.ai") || has("api.together.xyz")
	isMoonshot := provider == "moonshotai" || provider == "moonshotai-cn" || has("api.moonshot.")
	isOpenRouter := provider == "openrouter" || has("openrouter.ai")
	isCloudflareWorkersAI := provider == "cloudflare-workers-ai" || has("api.cloudflare.com")
	isCloudflareAiGateway := provider == "cloudflare-ai-gateway" || has("gateway.ai.cloudflare.com")
	isNvidia := provider == "nvidia" || has("integrate.api.nvidia.com")
	isAntLing := provider == "ant-ling" || has("api.ant-ling.com")
	isCerebras := provider == "cerebras" || has("cerebras.ai")
	isDeepSeek := provider == "deepseek" || strings.Contains(strings.ToLower(baseURL), "deepseek.com")

	isNonStandard := isNvidia || isCerebras ||
		provider == "xai" || has("api.x.ai") || isTogether || has("chutes.ai") ||
		isDeepSeek || isZai || isMoonshot || provider == "opencode" ||
		has("opencode.ai") || isCloudflareWorkersAI || isCloudflareAiGateway || isAntLing
	noLongCacheRetention := isTogether || isCloudflareWorkersAI || isCloudflareAiGateway || isNvidia || isAntLing
	useMaxTokens := has("chutes.ai") || isDeepSeek || isMoonshot || isCloudflareAiGateway ||
		isTogether || isNvidia || isAntLing || isZai

	isGrok := provider == "xai" || has("api.x.ai")
	isOpenRouterDeveloperRoleModel := isOpenRouter &&
		(strings.HasPrefix(m.ID, "anthropic/") || strings.HasPrefix(m.ID, "openai/"))

	cacheControlFormat := ""
	if provider == "openrouter" && strings.HasPrefix(m.ID, "anthropic/") {
		cacheControlFormat = "anthropic"
	}

	thinkingFormat := "openai"
	switch {
	case isDeepSeek:
		thinkingFormat = "deepseek"
	case isZai:
		thinkingFormat = "zai"
	case isTogether:
		thinkingFormat = "together"
	case isAntLing:
		thinkingFormat = "ant-ling"
	case isOpenRouter:
		thinkingFormat = "openrouter"
	}

	maxTokensField := "max_completion_tokens"
	if useMaxTokens {
		maxTokensField = "max_tokens"
	}

	affinity := model.SessionAffinityOpenAI
	if isOpenRouter {
		affinity = model.SessionAffinityOpenRouter
	}

	return compat{
		SupportsStore:                               !isNonStandard,
		SupportsDeveloperRole:                       isOpenRouterDeveloperRoleModel || (!isNonStandard && !isOpenRouter),
		SupportsReasoningEffort:                     !isGrok && !isZai && !isMoonshot && !isTogether && !isCloudflareAiGateway && !isNvidia && !isAntLing,
		SupportsUsageInStreaming:                    true,
		SupportsFinishReason:                        true,
		MaxTokensField:                              maxTokensField,
		ThinkingFormat:                              thinkingFormat,
		SupportsLongCacheRetention:                  !noLongCacheRetention,
		RequiresReasoningContentOnAssistantMessages: isDeepSeek,
		SendSessionAffinityHeaders:                  isOpenRouter,
		SessionAffinityFormat:                       affinity,
		CacheControlFormat:                          cacheControlFormat,
		ChatTemplateKwargs:                          map[string]model.ChatTemplateKwargValue{},
		ChatTemplateArgs:                            map[string]model.ChatTemplateKwargValue{},
	}
}

// getCompat applies explicit model.compat overrides one key at a time over the
// detected profile (pi getCompat).
func getCompat(m *model.Model) compat {
	c := detectCompat(m)
	if len(m.Compat) == 0 {
		return c
	}
	var o model.OpenAICompletionsCompat
	if err := json.Unmarshal(m.Compat, &o); err != nil {
		return c
	}
	if o.SupportsStore != nil {
		c.SupportsStore = *o.SupportsStore
	}
	if o.SupportsDeveloperRole != nil {
		c.SupportsDeveloperRole = *o.SupportsDeveloperRole
	}
	if o.SupportsReasoningEffort != nil {
		c.SupportsReasoningEffort = *o.SupportsReasoningEffort
	}
	if o.SupportsUsageInStreaming != nil {
		c.SupportsUsageInStreaming = *o.SupportsUsageInStreaming
	}
	if o.SupportsFinishReason != nil {
		c.SupportsFinishReason = *o.SupportsFinishReason
	}
	if o.MaxTokensField != "" {
		c.MaxTokensField = o.MaxTokensField
	}
	if o.ThinkingFormat != "" {
		c.ThinkingFormat = o.ThinkingFormat
	}
	if o.SupportsStrictMode != nil {
		c.SupportsStrictMode = *o.SupportsStrictMode
	}
	if o.SupportsOpenAIGrammarTools != nil {
		c.SupportsOpenAIGrammarTools = *o.SupportsOpenAIGrammarTools
	}
	if o.SupportsMidConvoSystemMessages != nil {
		c.SupportsMidConvoSystemMessages = *o.SupportsMidConvoSystemMessages
	}
	if o.SupportsMidConvoToolAdditions != nil {
		c.SupportsMidConvoToolAdditions = *o.SupportsMidConvoToolAdditions
	}
	if o.SupportsLongCacheRetention != nil {
		c.SupportsLongCacheRetention = *o.SupportsLongCacheRetention
	}
	if o.RequiresReasoningContentOnAssistantMessages != nil {
		c.RequiresReasoningContentOnAssistantMessages = *o.RequiresReasoningContentOnAssistantMessages
	}
	if o.RequiresToolResultName != nil {
		c.RequiresToolResultName = *o.RequiresToolResultName
	}
	if o.RequiresAssistantAfterToolResult != nil {
		c.RequiresAssistantAfterToolResult = *o.RequiresAssistantAfterToolResult
	}
	if o.RequiresThinkingAsText != nil {
		c.RequiresThinkingAsText = *o.RequiresThinkingAsText
	}
	if o.ZaiToolStream != nil {
		c.ZaiToolStream = *o.ZaiToolStream
	}
	if o.ThinkingTokenBudgetField != "" {
		c.ThinkingTokenBudgetField = string(o.ThinkingTokenBudgetField)
	}
	if o.SupportsThinkingTokenBudget != nil {
		c.SupportsThinkingTokenBudget = *o.SupportsThinkingTokenBudget
	}
	if o.SendSessionAffinityHeaders != nil {
		c.SendSessionAffinityHeaders = *o.SendSessionAffinityHeaders
	}
	if o.SessionAffinityFormat != "" {
		c.SessionAffinityFormat = o.SessionAffinityFormat
	}
	if o.CacheControlFormat != "" {
		c.CacheControlFormat = o.CacheControlFormat
	}
	if o.OpenRouterRouting != nil {
		c.OpenRouterRouting = o.OpenRouterRouting
		c.HasOpenRouterRouting = true
	}
	if o.VercelGatewayRouting != nil {
		c.VercelGatewayRouting = *o.VercelGatewayRouting
	}
	if o.ChatTemplateKwargs != nil {
		c.ChatTemplateKwargs = o.ChatTemplateKwargs
	}
	if o.ChatTemplateArgs != nil {
		c.ChatTemplateArgs = o.ChatTemplateArgs
	}

	// vllmPriority is read bare, so present-null is distinct from absent.
	var raw map[string]json.RawMessage
	if json.Unmarshal(m.Compat, &raw) == nil {
		if value, present := raw["vllmPriority"]; present {
			c.HasVLLMPriority = true
			var priority int
			if json.Unmarshal(value, &priority) == nil {
				c.VLLMPriority = &priority
			}
		}
	}
	return c
}
