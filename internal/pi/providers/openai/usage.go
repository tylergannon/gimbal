package openai

import "github.com/tylergannon/gimbal/internal/pi/model"

// Chunk usage parsing and cost calculation, ported from openai-completions.ts
// parseChunkUsage and pi models.ts calculateCost (upstream d6af72e1).

// rawUsage is the usage object a Chat Completions chunk may carry.
type rawUsage struct {
	PromptTokens            *int                    `json:"prompt_tokens"`
	CompletionTokens        *int                    `json:"completion_tokens"`
	CachedTokens            *int                    `json:"cached_tokens"`
	PromptCacheHitTokens    *int                    `json:"prompt_cache_hit_tokens"`
	PromptTokensDetails     *usagePromptDetails     `json:"prompt_tokens_details"`
	CompletionTokensDetails *usageCompletionDetails `json:"completion_tokens_details"`
}

type usagePromptDetails struct {
	CachedTokens     *int `json:"cached_tokens"`
	CacheWriteTokens *int `json:"cache_write_tokens"`
}

type usageCompletionDetails struct {
	ReasoningTokens *int `json:"reasoning_tokens"`
}

func intOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

// parseChunkUsage converts a provider usage chunk into model.Usage and
// calculates its cost.
func parseChunkUsage(raw *rawUsage, m *model.Model) model.Usage {
	promptTokens := intOrZero(raw.PromptTokens)
	cacheReadTokens := 0
	if raw.PromptTokensDetails != nil && raw.PromptTokensDetails.CachedTokens != nil {
		cacheReadTokens = *raw.PromptTokensDetails.CachedTokens
	} else if raw.PromptCacheHitTokens != nil {
		cacheReadTokens = *raw.PromptCacheHitTokens
	} else if raw.CachedTokens != nil {
		cacheReadTokens = *raw.CachedTokens
	}
	cacheWriteTokens := 0
	if raw.PromptTokensDetails != nil {
		cacheWriteTokens = intOrZero(raw.PromptTokensDetails.CacheWriteTokens)
	}
	input := max(0, promptTokens-cacheReadTokens-cacheWriteTokens)
	outputTokens := intOrZero(raw.CompletionTokens)
	reasoningTokens := 0
	if raw.CompletionTokensDetails != nil {
		reasoningTokens = intOrZero(raw.CompletionTokensDetails.ReasoningTokens)
	}
	usage := model.Usage{
		Input:       input,
		Output:      outputTokens,
		CacheRead:   cacheReadTokens,
		CacheWrite:  cacheWriteTokens,
		Reasoning:   reasoningTokens,
		TotalTokens: input + outputTokens + cacheReadTokens + cacheWriteTokens,
	}
	calculateCost(m, &usage)
	return usage
}

// calculateCost fills in a usage value's cost from the model's rates, applying
// the highest matching request-wide tier.
func calculateCost(m *model.Model, usage *model.Usage) {
	inputTokens := usage.Input + usage.CacheRead + usage.CacheWrite
	rates := m.Cost.ModelCostRates
	matchedThreshold := -1
	for _, tier := range m.Cost.Tiers {
		if inputTokens > tier.InputTokensAbove && tier.InputTokensAbove > matchedThreshold {
			rates = tier.ModelCostRates
			matchedThreshold = tier.InputTokensAbove
		}
	}
	longWrite := usage.CacheWrite1h
	shortWrite := usage.CacheWrite - longWrite
	usage.Cost.Input = (rates.Input / 1_000_000) * float64(usage.Input)
	usage.Cost.Output = (rates.Output / 1_000_000) * float64(usage.Output)
	usage.Cost.CacheRead = (rates.CacheRead / 1_000_000) * float64(usage.CacheRead)
	usage.Cost.CacheWrite = (rates.CacheWrite*float64(shortWrite) + rates.Input*2*float64(longWrite)) / 1_000_000
	usage.Cost.Total = usage.Cost.Input + usage.Cost.Output + usage.Cost.CacheRead + usage.Cost.CacheWrite
}
