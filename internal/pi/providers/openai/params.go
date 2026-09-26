package openai

import (
	"maps"
	"slices"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/wire"
)

// Request parameter building, ported from openai-completions.ts buildParams
// (upstream d6af72e1).

// Options are the provider-native options for a Chat Completions request.
type Options struct {
	model.StreamOptions
	// ToolChoice selects whether the model may call tools. It is a string
	// ("auto" | "none" | "required") or an object.
	ToolChoice any
	// ReasoningEffort is the provider-native effort level; empty disables
	// reasoning.
	ReasoningEffort string
	// ThinkingBudgets are token budgets per thinking level.
	ThinkingBudgets *model.ThinkingBudgets
}

// resolveCacheRetention chooses the effective cache retention preference.
func resolveCacheRetention(retention model.CacheRetention, env model.ProviderEnv) model.CacheRetention {
	if retention != "" {
		return retention
	}
	if getProviderEnvValue("PI_CACHE_RETENTION", env) == "long" {
		return model.CacheLong
	}
	return model.CacheShort
}

// buildParams assembles the Chat Completions request body.
func buildParams(m *model.Model, transcript model.TranscriptContext, options *Options, c compat, cacheRetention model.CacheRetention, grammarProps map[string]string) (map[string]any, error) {
	supportsToolAdditions := c.SupportsMidConvoSystemMessages && c.SupportsMidConvoToolAdditions
	transcriptTools := model.ResolveTranscriptTools(transcript.Messages, supportsToolAdditions)

	messages, err := convertMessages(m, transcript, c, grammarProps)
	if err != nil {
		return nil, err
	}

	params := map[string]any{
		"model":    m.ID,
		"messages": messages,
		"stream":   true,
	}
	if c.SupportsUsageInStreaming {
		params["stream_options"] = map[string]any{"include_usage": true}
	}
	if c.SupportsStore {
		params["store"] = false
	}

	sessionID := ""
	if options != nil {
		sessionID = options.SessionID
	}
	if sessionID != "" &&
		((strings.Contains(m.BaseURL, "api.openai.com") && cacheRetention != model.CacheNone) ||
			(cacheRetention == model.CacheLong && c.SupportsLongCacheRetention)) {
		params["prompt_cache_key"] = clampPromptCacheKey(sessionID)
	}
	if cacheRetention == model.CacheLong && c.SupportsLongCacheRetention {
		params["prompt_cache_retention"] = "24h"
	}

	if options != nil && options.MaxTokens != nil && *options.MaxTokens > 0 {
		params[c.MaxTokensField] = *options.MaxTokens
	}
	if options != nil && options.Temperature != nil {
		params["temperature"] = *options.Temperature
	}

	if len(transcriptTools.RequestTools) > 0 {
		tools, err := convertTools(transcriptTools.RequestTools, c)
		if err != nil {
			return nil, err
		}
		params["tools"] = tools
		if c.ZaiToolStream {
			params["tool_stream"] = true
		}
	} else if hasToolHistory(transcript.Messages) {
		params["tools"] = []map[string]any{}
	}

	if control := compatCacheControl(c, cacheRetention); control != nil {
		applyAnthropicCacheControl(messages, params["tools"], control)
	}

	if options != nil && options.ToolChoice != nil {
		params["tool_choice"] = options.ToolChoice
	}

	if c.HasVLLMPriority {
		if c.VLLMPriority != nil {
			params["priority"] = *c.VLLMPriority
		} else {
			params["priority"] = nil
		}
	}

	budgetField := resolveThinkingTokenBudgetField(c)
	thinkingBudget := resolveClampedThinkingBudget(m, options, params)
	level := ""
	if options != nil {
		level = options.ReasoningEffort
	}
	applyReasoningFormat(params, m, c, level, thinkingBudget)
	if budgetField != "" && thinkingBudget != nil {
		params[budgetField] = *thinkingBudget
	}

	if c.HasOpenRouterRouting {
		params["provider"] = c.OpenRouterRouting
	}
	if len(c.VercelGatewayRouting.Only) > 0 || len(c.VercelGatewayRouting.Order) > 0 {
		gateway := map[string]any{}
		if len(c.VercelGatewayRouting.Only) > 0 {
			gateway["only"] = c.VercelGatewayRouting.Only
		}
		if len(c.VercelGatewayRouting.Order) > 0 {
			gateway["order"] = c.VercelGatewayRouting.Order
		}
		params["providerOptions"] = map[string]any{"gateway": gateway}
	}

	if options != nil && options.SamplingParams != nil {
		maps.Copy(params, options.SamplingParams)
	}
	return params, nil
}

// resolveThinkingTokenBudgetField resolves the top-level budget field name.
func resolveThinkingTokenBudgetField(c compat) string {
	if c.ThinkingTokenBudgetField != "" {
		return c.ThinkingTokenBudgetField
	}
	if c.SupportsThinkingTokenBudget {
		return "thinking_token_budget"
	}
	return ""
}

// resolveClampedThinkingBudget computes the thinking budget after reserving
// answer room under the response ceiling.
func resolveClampedThinkingBudget(m *model.Model, options *Options, params map[string]any) *int {
	if options == nil || options.ReasoningEffort == "" || !m.Reasoning {
		return nil
	}
	ceiling := m.MaxTokens
	if value, ok := params["max_tokens"].(int); ok {
		ceiling = value
	} else if value, ok := params["max_completion_tokens"].(int); ok {
		ceiling = value
	}
	budget := wire.ClampThinkingBudgetToAnswerRoom(
		wire.ThinkingBudgetForLevel(model.ThinkingLevel(options.ReasoningEffort), options.ThinkingBudgets), ceiling)
	if budget <= 0 {
		return nil
	}
	return &budget
}

// applyReasoningFormat sets the provider-specific reasoning fields.
func applyReasoningFormat(params map[string]any, m *model.Model, c compat, level string, thinkingBudget *int) {
	enabled := level != ""
	switch {
	case c.ThinkingFormat == "zai" && m.Reasoning:
		if enabled {
			params["thinking"] = map[string]any{"type": "enabled", "clear_thinking": false}
		} else {
			params["thinking"] = map[string]any{"type": "disabled"}
		}
		if enabled && c.SupportsReasoningEffort {
			if effort, ok := mappedEffortOrRaw(m, level); ok {
				params["reasoning_effort"] = effort
			}
		}
	case c.ThinkingFormat == "qwen" && m.Reasoning:
		params["enable_thinking"] = enabled
		if enabled && c.SupportsReasoningEffort {
			params["reasoning_effort"] = effortValue(m, level)
		}
	case c.ThinkingFormat == "qwen-chat-template" && m.Reasoning:
		params["chat_template_kwargs"] = map[string]any{"enable_thinking": enabled, "preserve_thinking": true}
	case c.ThinkingFormat == "chat-template" && m.Reasoning:
		if kwargs := buildChatTemplateValues(m, c.ChatTemplateKwargs, level, thinkingBudget); kwargs != nil {
			params["chat_template_kwargs"] = kwargs
		}
	case c.ThinkingFormat == "baseten" && m.Reasoning:
		if args := buildChatTemplateValues(m, c.ChatTemplateArgs, level, thinkingBudget); args != nil {
			params["chat_template_args"] = args
		}
		if c.SupportsReasoningEffort {
			if enabled {
				if effort, ok := mappedEffortOrRaw(m, level); ok {
					params["reasoning_effort"] = effort
				}
			} else if off, ok := offEffortValue(m); ok {
				params["reasoning_effort"] = off
			}
		}
	case c.ThinkingFormat == "deepseek" && m.Reasoning:
		if enabled {
			params["thinking"] = map[string]any{"type": "enabled"}
		} else if _, send := offEffortOrDefault(m, ""); send {
			params["thinking"] = map[string]any{"type": "disabled"}
		}
		if enabled && c.SupportsReasoningEffort {
			params["reasoning_effort"] = effortValue(m, level)
		}
	case c.ThinkingFormat == "openrouter" && m.Reasoning:
		if enabled {
			params["reasoning"] = map[string]any{"effort": effortValue(m, level)}
		} else if off, send := offEffortOrDefault(m, "none"); send {
			params["reasoning"] = map[string]any{"effort": off}
		}
	case c.ThinkingFormat == "ant-ling" && m.Reasoning && enabled:
		if v, ok := offOrMapped(m, level); ok {
			params["reasoning"] = map[string]any{"effort": v}
		}
	case c.ThinkingFormat == "together" && m.Reasoning:
		params["reasoning"] = map[string]any{"enabled": enabled}
		if enabled && c.SupportsReasoningEffort {
			params["reasoning_effort"] = effortValue(m, level)
		}
	case c.ThinkingFormat == "string-thinking" && m.Reasoning:
		if enabled {
			params["thinking"] = effortValue(m, level)
		} else if off, send := offEffortOrDefault(m, "none"); send {
			params["thinking"] = off
		}
	case enabled && m.Reasoning && c.SupportsReasoningEffort:
		params["reasoning_effort"] = effortValue(m, level)
	case !enabled && m.Reasoning && c.SupportsReasoningEffort:
		if off, ok := offEffortValue(m); ok {
			params["reasoning_effort"] = off
		}
	}
}

func effortValue(m *model.Model, level string) string {
	if value, ok := m.ThinkingLevelMap[model.ModelThinkingLevel(level)]; ok && value != nil {
		return *value
	}
	return level
}

func offEffortValue(m *model.Model) (string, bool) {
	if value, ok := m.ThinkingLevelMap[model.ModelThinkingLevel("off")]; ok && value != nil {
		return *value, true
	}
	return "", false
}

func offEffortOrDefault(m *model.Model, fallback string) (string, bool) {
	if value, ok := m.ThinkingLevelMap[model.ModelThinkingLevel("off")]; ok {
		if value == nil {
			return "", false
		}
		return *value, true
	}
	return fallback, true
}

func mappedEffortOrRaw(m *model.Model, level string) (string, bool) {
	if value, ok := m.ThinkingLevelMap[model.ModelThinkingLevel(level)]; ok {
		if value == nil {
			return "", false
		}
		return *value, true
	}
	return level, true
}

func offOrMapped(m *model.Model, level string) (string, bool) {
	if value, ok := m.ThinkingLevelMap[model.ModelThinkingLevel(level)]; ok && value != nil {
		return *value, true
	}
	return "", false
}

// chatTemplateVar recognizes a {"$var": ...} placeholder.
func chatTemplateVar(value any) (model.ChatTemplateVar, bool) {
	switch v := value.(type) {
	case model.ChatTemplateVar:
		return v, true
	case map[string]any:
		name, ok := v["$var"].(string)
		if !ok {
			return model.ChatTemplateVar{}, false
		}
		out := model.ChatTemplateVar{Var: name}
		if omit, ok := v["omitWhenOff"].(bool); ok {
			out.OmitWhenOff = omit
		}
		return out, true
	default:
		return model.ChatTemplateVar{}, false
	}
}

// buildChatTemplateValues resolves configured chat-template kwargs.
func buildChatTemplateValues(m *model.Model, values map[string]model.ChatTemplateKwargValue, level string, thinkingBudget *int) map[string]any {
	if len(values) == 0 {
		return nil
	}
	resolved := map[string]any{}
	for key, value := range values {
		if out, ok := resolveChatTemplateKwargValue(m, value, level, thinkingBudget); ok {
			resolved[key] = out
		}
	}
	if len(resolved) == 0 {
		return nil
	}
	return resolved
}

func resolveChatTemplateKwargValue(m *model.Model, value any, level string, thinkingBudget *int) (any, bool) {
	variable, ok := chatTemplateVar(value)
	if !ok {
		return value, true
	}
	if level == "" && variable.OmitWhenOff {
		return nil, false
	}
	switch variable.Var {
	case "thinking.enabled":
		return level != "", true
	case "thinking.budget":
		if thinkingBudget == nil {
			return nil, false
		}
		return *thinkingBudget, true
	default:
		return chatTemplateEffortValue(m, level)
	}
}

func chatTemplateEffortValue(m *model.Model, level string) (any, bool) {
	var mapped *string
	present := false
	if level != "" {
		mapped, present = m.ThinkingLevelMap[model.ModelThinkingLevel(level)]
	} else {
		mapped, present = m.ThinkingLevelMap[model.ModelThinkingLevel("off")]
	}
	if !present {
		if level != "" {
			return level, true
		}
		return nil, false
	}
	if mapped == nil {
		return nil, false
	}
	return *mapped, true
}

// extendedThinkingLevels is pi's ordered reasoning-level ladder.
var extendedThinkingLevels = []model.ModelThinkingLevel{
	"off", "minimal", "low", "medium", "high", "xhigh", "max",
}

func getSupportedThinkingLevels(m *model.Model) []model.ModelThinkingLevel {
	if !m.Reasoning {
		return []model.ModelThinkingLevel{"off"}
	}
	var out []model.ModelThinkingLevel
	for _, level := range extendedThinkingLevels {
		mapped, present := m.ThinkingLevelMap[level]
		if present && mapped == nil {
			continue
		}
		if level == model.ModelThinkingLevel(model.ThinkingXHigh) || level == model.ModelThinkingLevel(model.ThinkingMax) {
			if !present {
				continue
			}
		}
		out = append(out, level)
	}
	return out
}

func containsThinkingLevel(levels []model.ModelThinkingLevel, level model.ModelThinkingLevel) bool {
	return slices.Contains(levels, level)
}

// clampThinkingLevel maps a requested level onto one the model supports.
func clampThinkingLevel(m *model.Model, level model.ModelThinkingLevel) model.ModelThinkingLevel {
	available := getSupportedThinkingLevels(m)
	if containsThinkingLevel(available, level) {
		return level
	}
	index := -1
	for i, candidate := range extendedThinkingLevels {
		if candidate == level {
			index = i
			break
		}
	}
	if index == -1 {
		if len(available) > 0 {
			return available[0]
		}
		return model.ModelThinkingLevel(model.ThinkingOff)
	}
	for i := index; i < len(extendedThinkingLevels); i++ {
		if containsThinkingLevel(available, extendedThinkingLevels[i]) {
			return extendedThinkingLevels[i]
		}
	}
	for i := index - 1; i >= 0; i-- {
		if containsThinkingLevel(available, extendedThinkingLevels[i]) {
			return extendedThinkingLevels[i]
		}
	}
	if len(available) > 0 {
		return available[0]
	}
	return model.ModelThinkingLevel(model.ThinkingOff)
}
