package wire

import (
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func overflowErrorMessage(errorMessage, provider string) *model.AssistantMessage {
	return &model.AssistantMessage{
		Api:          model.APIOpenAICompletions,
		Provider:     model.ProviderId(provider),
		Model:        "qwen3.5:35b",
		StopReason:   model.StopError,
		ErrorMessage: errorMessage,
	}
}

func lengthStopMessage(input, cacheRead, output int, provider string) *model.AssistantMessage {
	return &model.AssistantMessage{
		Api:        model.APIOpenAICompletions,
		Provider:   model.ProviderId(provider),
		Model:      "test-model",
		StopReason: model.StopLength,
		Usage: model.Usage{
			Input:       input,
			Output:      output,
			CacheRead:   cacheRead,
			TotalTokens: input + cacheRead + output,
		},
	}
}

func TestIsContextOverflowDetectsProviderErrors(t *testing.T) {
	cases := []struct {
		message  string
		provider string
		window   int
	}{
		{"400 `prompt too long; exceeded max context length by 100918 tokens`", "ollama", 32768},
		{`400 {"code":"1261","message":"Prompt too long"}`, "zai", 1048576},
		{"400 The input (516368 tokens) is longer than the model's context length (262144 tokens).", "together", 262144},
		{"Error: 503 litellm.ServiceUnavailableError: Requested token count exceeds the model's maximum context length of 131072 tokens.", "litellm", 131072},
		{"Error: 400 Input length (265330) exceeds model's maximum context length (262144).", "openai", 262144},
		{"Provider returned error: Input length 131393 exceeds the maximum allowed input length of 131040 tokens.", "openrouter", 131072},
		{"400 Prompt has 256468 tokens, but the configured context size is 256000 tokens", "ds4", 256000},
		{"Prompt has 5,958,968 tokens, but the configured context size is 256,000 tokens", "ds4", 256000},
	}
	for _, c := range cases {
		if !IsContextOverflow(overflowErrorMessage(c.message, c.provider), c.window) {
			t.Errorf("IsContextOverflow(%q) = false, want true", c.message)
		}
	}
}

func TestIsContextOverflowRejectsNonOverflowErrors(t *testing.T) {
	cases := []struct {
		message  string
		provider string
	}{
		{"500 `model runner crashed unexpectedly`", "ollama"},
		{"Throttling error: Too many tokens, please wait before trying again.", "amazon-bedrock"},
		{"Service unavailable: The service is temporarily unavailable.", "amazon-bedrock"},
		{"Rate limit exceeded, please retry after 30 seconds.", "openai"},
		{"Too many requests. Please slow down.", "openai"},
	}
	for _, c := range cases {
		if IsContextOverflow(overflowErrorMessage(c.message, c.provider), 200000) {
			t.Errorf("IsContextOverflow(%q) = true, want false", c.message)
		}
	}
}

func TestIsContextOverflowCerebrasBodyless(t *testing.T) {
	for _, message := range []string{"400 status code (no body)", "413 status code (no body)"} {
		if !IsContextOverflow(overflowErrorMessage(message, "cerebras"), 131072) {
			t.Errorf("IsContextOverflow(%q, cerebras) = false, want true", message)
		}
		if IsContextOverflow(overflowErrorMessage(message, "opencode-go"), 1000000) {
			t.Errorf("IsContextOverflow(%q, opencode-go) = true, want false", message)
		}
	}
}

func TestIsContextOverflowSilentAndLengthStops(t *testing.T) {
	silent := &model.AssistantMessage{
		StopReason: model.StopStop,
		Usage:      model.Usage{Input: 1048577},
	}
	if !IsContextOverflow(silent, 1048576) {
		t.Error("silent usage overflow must be detected")
	}

	xiaomi := lengthStopMessage(58, 1048512, 0, "xiaomi")
	if !IsContextOverflow(xiaomi, 1048576) {
		t.Error("Xiaomi-style length stop must be detected")
	}

	normal := lengthStopMessage(1000, 0, 4096, "test-provider")
	if IsContextOverflow(normal, 200000) {
		t.Error("a normal length stop must not be overflow")
	}

	farBelow := lengthStopMessage(100, 0, 0, "test-provider")
	if IsContextOverflow(farBelow, 200000) {
		t.Error("a zero-output length stop far below the window must not be overflow")
	}
}

func TestIsRecoverableLength(t *testing.T) {
	below := lengthStopMessage(3, 253584, 16, "openai")
	if !IsRecoverableLength(below, 128000) {
		t.Error("a length stop below the desired output must be recoverable")
	}
	reached := lengthStopMessage(4062, 0, 1024, "test-provider")
	if IsRecoverableLength(reached, 1024) {
		t.Error("a length stop at the desired output must not be recoverable")
	}
	zeroOutput := lengthStopMessage(100, 0, 0, "test-provider")
	if !IsRecoverableLength(zeroOutput, 128000) {
		t.Error("a zero-output length stop must be recoverable")
	}
}
