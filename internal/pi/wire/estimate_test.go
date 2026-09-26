package wire

import (
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func estimateUsage(totalTokens int) model.Usage {
	return model.Usage{Input: totalTokens, TotalTokens: totalTokens}
}

func estimateAssistant(timestamp int64, totalTokens int) model.AssistantMessage {
	return model.AssistantMessage{
		Content:    model.ContentList{model.TextContent{Text: "kept"}},
		Api:        model.APIOpenAIResponses,
		Provider:   model.ProviderOpenAI,
		Model:      "test-model",
		Usage:      estimateUsage(totalTokens),
		StopReason: model.StopStop,
		Timestamp:  timestamp,
	}
}

func estimateModel() *model.Model {
	return &model.Model{
		ID:            "test-model",
		Api:           model.APIOpenAIResponses,
		Provider:      model.ProviderOpenAI,
		ContextWindow: 10_000,
		MaxTokens:     8_000,
	}
}

func TestEstimateContextTokensIgnoresStaleUsage(t *testing.T) {
	context := model.NormalizeContext(model.Context{
		SystemPrompt: "system",
		Messages: []model.Message{
			model.NewUserText("summary", 200),
			estimateAssistant(100, 9_500),
			model.NewUserText(strings.Repeat("x", 4_000), 300),
		},
	})

	got := EstimateContextTokens(context)
	if got.Tokens != 1_005 || got.UsageTokens != 0 || got.TrailingTokens != 1_005 || got.LastUsageIndex != nil {
		t.Fatalf("EstimateContextTokens = %+v, want tokens 1005, usage 0, trailing 1005, nil index", got)
	}

	options := BuildBaseOptions(estimateModel(), context, nil, "")
	if options.MaxTokens == nil || *options.MaxTokens != 4_899 {
		t.Fatalf("BuildBaseOptions maxTokens = %v, want 4899", options.MaxTokens)
	}
}

func TestEstimateContextTokensUsesFreshUsage(t *testing.T) {
	context := model.NormalizeContext(model.Context{
		Messages: []model.Message{
			model.NewUserText("summary", 200),
			estimateAssistant(100, 9_500),
			model.NewUserText("new prompt", 300),
			estimateAssistant(400, 2_000),
			model.NewUserText("tail", 500),
		},
	})

	got := EstimateContextTokens(context)
	if got.Tokens != 2_001 || got.UsageTokens != 2_000 || got.TrailingTokens != 1 {
		t.Fatalf("EstimateContextTokens = %+v, want tokens 2001, usage 2000, trailing 1", got)
	}
	if got.LastUsageIndex == nil || *got.LastUsageIndex != 3 {
		t.Fatalf("LastUsageIndex = %v, want 3", got.LastUsageIndex)
	}
}

func TestThinkingBudgets(t *testing.T) {
	if got := ThinkingBudgetForLevel(model.ThinkingMinimal, nil); got != 1024 {
		t.Errorf("minimal budget = %d, want 1024", got)
	}
	if got := ThinkingBudgetForLevel(model.ThinkingHigh, nil); got != 16384 {
		t.Errorf("high budget = %d, want 16384", got)
	}
	if got := ThinkingBudgetForLevel(model.ThinkingXHigh, nil); got != 16384 {
		t.Errorf("xhigh budget = %d, want high 16384", got)
	}
	custom := model.ThinkingBudgets{Minimal: new(7)}
	if got := ThinkingBudgetForLevel(model.ThinkingMinimal, &custom); got != 7 {
		t.Errorf("custom minimal budget = %d, want 7", got)
	}
	if got := ClampThinkingBudgetToAnswerRoom(5000, 3000); got != 1976 {
		t.Errorf("clamped budget = %d, want 1976", got)
	}
	base := 100
	maxTokens, thinking := AdjustMaxTokensForThinking(&base, 100000, model.ThinkingMedium, nil)
	if maxTokens != 8292 || thinking != 8192 {
		t.Errorf("AdjustMaxTokensForThinking = %d, %d, want 8292, 8192", maxTokens, thinking)
	}
}
