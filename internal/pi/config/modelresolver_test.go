package config

import (
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func resolverModels() []*model.Model {
	return []*model.Model{
		{
			ID: "claude-sonnet-4-5", Name: "Claude Sonnet 4.5", Api: model.APIAnthropicMessages,
			Provider: "anthropic", BaseURL: "https://api.anthropic.com", Reasoning: true,
			Input: []string{"text", "image"}, ContextWindow: 200000, MaxTokens: 8192,
		},
		{
			ID: "gpt-4o", Name: "GPT-4o", Api: model.APIAnthropicMessages,
			Provider: "openai", BaseURL: "https://api.openai.com",
			Input: []string{"text", "image"}, ContextWindow: 128000, MaxTokens: 4096,
		},
		{
			ID: "qwen/qwen3-coder:exacto", Name: "Qwen3 Coder Exacto", Api: model.APIAnthropicMessages,
			Provider: "openrouter", BaseURL: "https://openrouter.ai/api/v1", Reasoning: true,
			Input: []string{"text"}, ContextWindow: 128000, MaxTokens: 8192,
		},
		{
			ID: "openai/gpt-4o:extended", Name: "GPT-4o Extended", Api: model.APIAnthropicMessages,
			Provider: "openrouter", BaseURL: "https://openrouter.ai/api/v1",
			Input: []string{"text", "image"}, ContextWindow: 128000, MaxTokens: 4096,
		},
	}
}

func TestParseModelPatternSimple(t *testing.T) {
	models := resolverModels()
	result := ParseModelPattern("claude-sonnet-4-5", models, nil)
	if result.Model == nil || result.Model.ID != "claude-sonnet-4-5" || result.ThinkingLevel != "" || result.Warning != "" {
		t.Fatalf("result = %+v", result)
	}
	result = ParseModelPattern("sonnet", models, nil)
	if result.Model == nil || result.Model.ID != "claude-sonnet-4-5" {
		t.Fatalf("partial result = %+v", result)
	}
	result = ParseModelPattern("nonexistent", models, nil)
	if result.Model != nil || result.ThinkingLevel != "" || result.Warning != "" {
		t.Fatalf("missing result = %+v", result)
	}
}

func TestParseModelPatternThinkingLevels(t *testing.T) {
	models := resolverModels()
	for _, level := range []model.ThinkingLevel{model.ThinkingOff, model.ThinkingMinimal, model.ThinkingLow, model.ThinkingMedium, model.ThinkingHigh, model.ThinkingXHigh, model.ThinkingMax} {
		result := ParseModelPattern("sonnet:"+string(level), models, nil)
		if result.Model == nil || result.Model.ID != "claude-sonnet-4-5" || result.ThinkingLevel != level || result.Warning != "" {
			t.Fatalf("level %q -> %+v", level, result)
		}
	}
	result := ParseModelPattern("sonnet:random", models, nil)
	if result.Model == nil || result.ThinkingLevel != "" || result.Warning == "" {
		t.Fatalf("invalid level -> %+v", result)
	}
}

func TestParseModelPatternColonIDs(t *testing.T) {
	models := resolverModels()
	result := ParseModelPattern("qwen/qwen3-coder:exacto", models, nil)
	if result.Model == nil || result.Model.ID != "qwen/qwen3-coder:exacto" || result.ThinkingLevel != "" {
		t.Fatalf("exacto -> %+v", result)
	}
	result = ParseModelPattern("openrouter/qwen/qwen3-coder:exacto:high", models, nil)
	if result.Model == nil || result.Model.Provider != "openrouter" || result.ThinkingLevel != model.ThinkingHigh {
		t.Fatalf("exacto:high -> %+v", result)
	}
	result = ParseModelPattern("qwen/qwen3-coder:exacto:random", models, nil)
	if result.Model == nil || result.ThinkingLevel != "" || result.Warning == "" {
		t.Fatalf("exacto:random -> %+v", result)
	}
}

func TestResolveModelScopeFromModels(t *testing.T) {
	models := resolverModels()
	result := ResolveModelScopeFromModels([]string{"sonnet:high", "gpt-4o:invalid", "missing"}, models)
	if len(result.ScopedModels) != 2 {
		t.Fatalf("scoped = %+v", result.ScopedModels)
	}
	if result.ScopedModels[0].Model.ID != "claude-sonnet-4-5" || result.ScopedModels[0].ThinkingLevel != model.ThinkingHigh {
		t.Fatalf("scoped[0] = %+v", result.ScopedModels[0])
	}
	if len(result.Diagnostics) != 2 || result.Diagnostics[0].Code != "invalid-thinking-level" || result.Diagnostics[1].Code != "no-match" {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
}

func TestResolveCLIModel(t *testing.T) {
	models := resolverModels()
	result := ResolveCLIModel(ResolveCLIModelOptions{CLIModel: "openai/gpt-4o", Models: models})
	if result.Error != "" || result.Model == nil || result.Model.Provider != "openai" || result.Model.ID != "gpt-4o" {
		t.Fatalf("result = %+v", result)
	}
	result = ResolveCLIModel(ResolveCLIModelOptions{CLIModel: "openai/gpt-4o:extended", Models: models})
	if result.Error != "" || result.Model == nil || result.Model.Provider != "openrouter" {
		t.Fatalf("openrouter-style result = %+v", result)
	}
	result = ResolveCLIModel(ResolveCLIModelOptions{CLIProvider: "openai", CLIModel: "4o", Models: models})
	if result.Error != "" || result.Model == nil || result.Model.ID != "gpt-4o" {
		t.Fatalf("fuzzy result = %+v", result)
	}
	if result := ResolveCLIModel(ResolveCLIModelOptions{CLIModel: "x", Models: nil}); result.Error == "" {
		t.Fatal("expected no-models error")
	}
}

func TestResolveCLIModelFallbackStripsThinking(t *testing.T) {
	models := append(resolverModels(), &model.Model{
		ID: "some-base-model", Name: "Some Base Model", Api: model.APIAnthropicMessages,
		Provider: "neuralwatt", BaseURL: "https://api.neuralwatt.com",
		Input: []string{"text"}, ContextWindow: 128000, MaxTokens: 8192,
	})
	result := ResolveCLIModel(ResolveCLIModelOptions{CLIModel: "neuralwatt/zai-org/GLM-5.1-FP8:high", Models: models})
	if result.Error != "" {
		t.Fatalf("error = %s", result.Error)
	}
	if result.Model == nil || result.Model.ID != "zai-org/GLM-5.1-FP8" || !result.Model.Reasoning || result.ThinkingLevel != model.ThinkingHigh {
		t.Fatalf("fallback = %+v", result)
	}

	result = ResolveCLIModel(ResolveCLIModelOptions{CLIModel: "neuralwatt/zai-org/GLM-5.1-FP8:banana", Models: models})
	if result.Error != "" || result.Model == nil || result.Model.ID != "zai-org/GLM-5.1-FP8:banana" || result.ThinkingLevel != "" {
		t.Fatalf("invalid suffix fallback = %+v", result)
	}
}

func TestFindInitialModel(t *testing.T) {
	models := resolverModels()
	sonnet := models[0]
	opus := &model.Model{
		ID: "claude-opus-4-8", Name: "Claude Opus 4.8", Api: model.APIAnthropicMessages,
		Provider: "anthropic", BaseURL: "https://api.anthropic.com", Reasoning: true,
		Input: []string{"text"}, ContextWindow: 200000, MaxTokens: 8192,
	}

	result := FindInitialModel(FindInitialModelOptions{
		ScopedModels:    []ScopedModel{{Model: sonnet, ThinkingLevel: model.ThinkingHigh}},
		AvailableModels: []*model.Model{sonnet, opus},
	})
	if result.Model != sonnet || result.ThinkingLevel != model.ThinkingHigh {
		t.Fatalf("scoped result = %+v", result)
	}

	result = FindInitialModel(FindInitialModelOptions{
		DefaultProvider: "anthropic", DefaultModelID: "claude-opus-4-8",
		AvailableModels:   []*model.Model{sonnet, opus},
		GetModel:          func(provider, id string) *model.Model { return opus },
		HasConfiguredAuth: func(provider string) bool { return false },
	})
	if result.Model != sonnet {
		t.Fatalf("unauthenticated default result = %+v", result)
	}

	result = FindInitialModel(FindInitialModelOptions{
		CLIProvider: "openai", CLIModel: "gpt-4o",
		AvailableModels: models,
	})
	if result.Error != "" || result.Model == nil || result.Model.ID != "gpt-4o" {
		t.Fatalf("cli result = %+v", result)
	}
}

func TestRestoreModelFromSession(t *testing.T) {
	models := resolverModels()
	sonnet, opus := models[0], models[1]
	getModel := func(provider, id string) *model.Model {
		if provider == "anthropic" && id == "claude-sonnet-4-5" {
			return sonnet
		}
		return nil
	}
	restored, message := RestoreModelFromSession("anthropic", "claude-sonnet-4-5", opus, nil, getModel, func(string) bool { return true })
	if restored != sonnet || message != "" {
		t.Fatalf("restore = %+v, %q", restored, message)
	}
	restored, message = RestoreModelFromSession("anthropic", "gone", opus, nil, getModel, func(string) bool { return true })
	if restored != opus || message == "" {
		t.Fatalf("fallback = %+v, %q", restored, message)
	}
}
