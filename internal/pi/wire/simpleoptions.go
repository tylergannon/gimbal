package wire

import (
	"github.com/tylergannon/gimbal/internal/pi/model"
)

// Simple request option building, ported from pi
// packages/ai/src/api/simple-options.ts (upstream d6af72e1).

const (
	contextSafetyTokens = 4096
	minMaxTokens        = 1
)

// ClampMaxTokensToContext caps maxTokens so the estimated context plus a safety
// margin fits the model's context window.
func ClampMaxTokensToContext(m *model.Model, context model.TranscriptContext, maxTokens int) int {
	if m.ContextWindow <= 0 {
		return max(minMaxTokens, maxTokens)
	}
	available := m.ContextWindow - EstimateContextTokens(context).Tokens - contextSafetyTokens
	return min(maxTokens, max(minMaxTokens, available))
}

// BuildBaseOptions assembles the stream options shared by the simple entry
// points, clamping maxTokens to the context window.
func BuildBaseOptions(m *model.Model, context model.TranscriptContext, options *model.SimpleStreamOptions, apiKey string) model.StreamOptions {
	maxTokens := m.MaxTokens
	var onProviderStreamEvent func(data any, m *model.Model) error
	var temperature *float64
	var env model.ProviderEnv
	if options != nil {
		if options.MaxTokens != nil {
			maxTokens = *options.MaxTokens
		}
		onProviderStreamEvent = options.OnProviderStreamEvent
		temperature = options.Temperature
		env = options.Env
	}
	clamped := ClampMaxTokensToContext(m, context, maxTokens)

	out := model.StreamOptions{
		OnProviderStreamEvent: onProviderStreamEvent,
		Temperature:           temperature,
		MaxTokens:             &clamped,
	}
	if options != nil {
		out.SamplingParams = options.SamplingParams
		out.Transport = options.Transport
		out.CacheRetention = options.CacheRetention
		out.SessionID = options.SessionID
		out.WebSocketConnectTimeoutMs = options.WebSocketConnectTimeoutMs
		out.Metadata = options.Metadata
		out.OnPayload = options.OnPayload
		out.OnResponse = options.OnResponse
		out.Headers = options.Headers
		out.TimeoutMs = options.TimeoutMs
		out.MaxRetries = options.MaxRetries
		out.MaxRetryDelayMs = options.MaxRetryDelayMs
		out.HTTPClient = options.HTTPClient
		out.Env = env
	}
	if apiKey != "" {
		out.APIKey = apiKey
	} else if options != nil {
		out.APIKey = options.APIKey
	}
	return out
}

// MinAnswerTokens are always left for the answer when a thinking budget shares
// the response ceiling.
const MinAnswerTokens = 1024

// DefaultThinkingBudgets are pi's default token budgets per thinking level.
var DefaultThinkingBudgets = model.ThinkingBudgets{
	Minimal: new(1024),
	Low:     new(2048),
	Medium:  new(8192),
	High:    new(16384),
}

//go:fix inline

// ClampReasoning maps xhigh and max down to high.
func ClampReasoning(effort model.ThinkingLevel) model.ThinkingLevel {
	if effort == model.ThinkingXHigh || effort == model.ThinkingMax {
		return model.ThinkingHigh
	}
	return effort
}

func budgetForLevel(budgets model.ThinkingBudgets, level model.ThinkingLevel) (int, bool) {
	var value *int
	switch level {
	case model.ThinkingMinimal:
		value = budgets.Minimal
	case model.ThinkingLow:
		value = budgets.Low
	case model.ThinkingMedium:
		value = budgets.Medium
	case model.ThinkingHigh:
		value = budgets.High
	default:
		return 0, false
	}
	if value == nil {
		return 0, false
	}
	return *value, true
}

// ThinkingBudgetForLevel returns the token budget for a reasoning level, with
// custom budgets overriding the defaults.
func ThinkingBudgetForLevel(reasoningLevel model.ThinkingLevel, customBudgets *model.ThinkingBudgets) int {
	level := ClampReasoning(reasoningLevel)
	merged := DefaultThinkingBudgets
	if customBudgets != nil {
		if customBudgets.Minimal != nil {
			merged.Minimal = customBudgets.Minimal
		}
		if customBudgets.Low != nil {
			merged.Low = customBudgets.Low
		}
		if customBudgets.Medium != nil {
			merged.Medium = customBudgets.Medium
		}
		if customBudgets.High != nil {
			merged.High = customBudgets.High
		}
	}
	value, _ := budgetForLevel(merged, level)
	return value
}

// ClampThinkingBudgetToAnswerRoom caps a thinking budget so at least
// MinAnswerTokens remain under a shared response ceiling.
func ClampThinkingBudgetToAnswerRoom(thinkingBudget, ceiling int) int {
	return min(thinkingBudget, max(0, ceiling-MinAnswerTokens))
}

// AdjustMaxTokensForThinking splits a response ceiling between thinking and the
// answer. A nil baseMaxTokens means no explicit caller cap.
func AdjustMaxTokensForThinking(baseMaxTokens *int, modelMaxTokens int, reasoningLevel model.ThinkingLevel, customBudgets *model.ThinkingBudgets) (maxTokens, thinkingBudget int) {
	thinkingBudget = ThinkingBudgetForLevel(reasoningLevel, customBudgets)
	if baseMaxTokens == nil {
		maxTokens = modelMaxTokens
	} else {
		maxTokens = min(*baseMaxTokens+thinkingBudget, modelMaxTokens)
	}
	if maxTokens <= thinkingBudget {
		thinkingBudget = ClampThinkingBudgetToAnswerRoom(thinkingBudget, maxTokens)
	}
	return maxTokens, thinkingBudget
}
