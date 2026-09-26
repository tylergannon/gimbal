package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/wire"
)

// testModel builds a model with explicit compat, the shape the Diffusion
// Router's config layer is expected to produce.
func testModel(provider, baseURL, compat string) *model.Model {
	m := &model.Model{
		Type:          model.ModelTypeChat,
		ID:            "test-model",
		Name:          "Test Model",
		Api:           model.APIOpenAICompletions,
		Provider:      provider,
		BaseURL:       baseURL,
		Reasoning:     false,
		Input:         []string{"text"},
		ContextWindow: 128000,
		MaxTokens:     8192,
	}
	if compat != "" {
		m.Compat = json.RawMessage(compat)
	}
	return m
}

func userTranscript(text string) model.TranscriptContext {
	return model.TranscriptContext{
		Messages: []model.Message{
			model.UserMessage{Content: model.ContentList{model.TextContent{Text: text}}},
		},
	}
}

// streamBody renders chunks as a sequence of SSE data events followed by
// [DONE].
func streamBody(chunks []any) string {
	var b strings.Builder
	for _, chunk := range chunks {
		raw, _ := json.Marshal(chunk)
		b.WriteString("data: ")
		b.Write(raw)
		b.WriteString("\n\n")
	}
	b.WriteString("data: [DONE]\n\n")
	return b.String()
}

// newSSEServer serves one streamed response per request and records each
// decoded request body.
func newSSEServer(t *testing.T, chunks []any, requests *[]map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if requests != nil {
			*requests = append(*requests, body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, streamBody(chunks))
	}))
}

func TestStreamDisablesSDKRetries(t *testing.T) {
	var requests []map[string]any
	server := newSSEServer(t, []any{
		map[string]any{"id": "c1", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "ok"}}}},
		map[string]any{"id": "c1", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}}},
	}, &requests)
	defer server.Close()

	m := testModel("local-vllm", server.URL, "")
	s := Stream(context.Background(), m, userTranscript("hi"), &Options{APIKey: "test"})
	result := s.Result()
	if result.StopReason != model.StopStop {
		t.Fatalf("stopReason = %q, want stop (%s)", result.StopReason, result.ErrorMessage)
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(requests))
	}
}

func TestStreamHonorsProviderRetries(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requests.Add(1)
		switch n {
		case 1:
			w.Header().Set("retry-after-ms", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = fmt.Fprint(w, `{"error":{"message":"rate limited"}}`)
		case 2:
			w.Header().Set("retry-after-ms", "1")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, `{"error":{"message":"server error"}}`)
		default:
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprint(w, streamBody([]any{
				map[string]any{"id": "c1", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "ok"}}}},
				map[string]any{"id": "c1", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}}},
			}))
		}
	}))
	defer server.Close()

	maxDelay := 60000
	m := testModel("local-vllm", server.URL, "")
	s := Stream(context.Background(), m, userTranscript("hi"), &Options{
		APIKey:          "test",
		MaxRetries:      2,
		MaxRetryDelayMs: &maxDelay})
	result := s.Result()
	if result.StopReason != model.StopStop {
		t.Fatalf("stopReason = %q, want stop (%s)", result.StopReason, result.ErrorMessage)
	}
	if got := requests.Load(); got != 3 {
		t.Fatalf("requests = %d, want 3", got)
	}
}

func TestStreamFailsFastOnLongRetryDelay(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("retry-after", "277403")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = fmt.Fprint(w, `{"error":{"message":"rate limited"}}`)
	}))
	defer server.Close()

	maxDelay := 1000
	m := testModel("local-vllm", server.URL, "")
	s := Stream(context.Background(), m, userTranscript("hi"), &Options{
		APIKey:          "test",
		MaxRetries:      2,
		MaxRetryDelayMs: &maxDelay})
	result := s.Result()
	if result.StopReason != model.StopError {
		t.Fatalf("stopReason = %q, want error", result.StopReason)
	}
	if !strings.Contains(result.ErrorMessage, "Server requested 277403s retry delay (max: 1s)") {
		t.Fatalf("errorMessage = %q", result.ErrorMessage)
	}
	if !strings.Contains(result.ErrorMessage, "rate limited") {
		t.Fatalf("errorMessage missing rate limited: %q", result.ErrorMessage)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func stopChunks() []any {
	return []any{
		map[string]any{"id": "c1", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}}},
	}
}

func TestToolChoiceForwarded(t *testing.T) {
	var requests []map[string]any
	server := newSSEServer(t, stopChunks(), &requests)
	defer server.Close()

	m := testModel("local-vllm", server.URL, "")
	tool := model.Tool{Name: "ping", Description: "Ping tool", Parameters: model.Object(model.Prop("ok", model.Boolean()))}
	transcript := model.TranscriptContext{Messages: []model.Message{
		model.SystemMessage{Content: model.ContentList{model.TextContent{Text: "Follow"}}, ToolsAdded: []model.Tool{tool}},
		model.UserMessage{Content: model.ContentList{model.TextContent{Text: "Call ping"}}},
	}}
	s := Stream(context.Background(), m, transcript, &Options{
		APIKey:     "test",
		ToolChoice: "required",
	})
	_ = s.Result()
	if len(requests) != 1 {
		t.Fatalf("requests = %d", len(requests))
	}
	if requests[0]["tool_choice"] != "required" {
		t.Fatalf("tool_choice = %v, want required", requests[0]["tool_choice"])
	}
	tools, _ := requests[0]["tools"].([]any)
	if len(tools) == 0 {
		t.Fatalf("tools missing")
	}
}

func TestToolChoiceWithoutTools(t *testing.T) {
	var requests []map[string]any
	server := newSSEServer(t, stopChunks(), &requests)
	defer server.Close()

	m := testModel("local-vllm", server.URL, "")
	s := Stream(context.Background(), m, userTranscript("Summarize"), &Options{
		APIKey:     "test",
		ToolChoice: "none",
	})
	_ = s.Result()
	if requests[0]["tool_choice"] != "none" {
		t.Fatalf("tool_choice = %v, want none", requests[0]["tool_choice"])
	}
	if _, present := requests[0]["tools"]; present {
		t.Fatalf("tools should be absent")
	}
}

func functionOf(t *testing.T, params map[string]any) map[string]any {
	t.Helper()
	tools, _ := params["tools"].([]any)
	if len(tools) == 0 {
		t.Fatalf("no tools in params")
	}
	entry, _ := tools[0].(map[string]any)
	function, _ := entry["function"].(map[string]any)
	return function
}

func TestStrictModeOmittedWhenDisabled(t *testing.T) {
	var requests []map[string]any
	server := newSSEServer(t, stopChunks(), &requests)
	defer server.Close()

	m := testModel("local-vllm", server.URL, `{"supportsStrictMode":false}`)
	tool := model.Tool{
		Name: "ping", Description: "Ping tool",
		Parameters:          model.Object(model.Prop("ok", model.Boolean())),
		ConstrainedSampling: &model.ConstrainedSamplingConfig{Type: model.ConstrainedSamplingJSONSchema, Strict: model.ConstrainedSamplingPrefer},
	}
	transcript := model.TranscriptContext{Messages: []model.Message{
		model.SystemMessage{Content: model.ContentList{model.TextContent{Text: "Follow"}}, ToolsAdded: []model.Tool{tool}},
		model.UserMessage{Content: model.ContentList{model.TextContent{Text: "Call"}}},
	}}
	_ = Stream(context.Background(), m, transcript, &Options{APIKey: "test"}).Result()
	function := functionOf(t, requests[0])
	if _, present := function["strict"]; present {
		t.Fatalf("strict should be absent: %v", function["strict"])
	}
}

func TestStrictModePreservedWhenSupported(t *testing.T) {
	var requests []map[string]any
	server := newSSEServer(t, stopChunks(), &requests)
	defer server.Close()

	m := testModel("local-vllm", server.URL, `{"supportsStrictMode":true}`)
	tool := model.Tool{
		Name: "ping", Description: "Ping tool",
		Parameters: model.Object(
			model.Prop("required", model.String()),
			model.Opt("optional", model.String()),
		),
		ConstrainedSampling: &model.ConstrainedSamplingConfig{Type: model.ConstrainedSamplingJSONSchema, Strict: model.ConstrainedSamplingPrefer},
	}
	transcript := model.TranscriptContext{Messages: []model.Message{
		model.SystemMessage{Content: model.ContentList{model.TextContent{Text: "Follow"}}, ToolsAdded: []model.Tool{tool}},
		model.UserMessage{Content: model.ContentList{model.TextContent{Text: "Call"}}},
	}}
	_ = Stream(context.Background(), m, transcript, &Options{APIKey: "test"}).Result()
	function := functionOf(t, requests[0])
	if function["strict"] != true {
		t.Fatalf("strict = %v, want true", function["strict"])
	}
	parameters, _ := function["parameters"].(map[string]any)
	required, _ := parameters["required"].([]any)
	if len(required) != 2 || required[0] != "required" || required[1] != "optional" {
		t.Fatalf("required = %v", required)
	}
}

func TestToolCallDeltasCoalesceByIndex(t *testing.T) {
	chunks := []any{
		map[string]any{
			"id": "c",
			"choices": []any{
				map[string]any{"index": 0, "delta": map[string]any{"tool_calls": []any{
					map[string]any{"index": 0, "id": "functions.read:0", "type": "function", "function": map[string]any{"name": "read", "arguments": ""}},
				}}},
			},
		},
		map[string]any{
			"id": "c",
			"choices": []any{
				map[string]any{"index": 0, "delta": map[string]any{"tool_calls": []any{
					map[string]any{"index": 0, "id": "chatcmpl-tool-a", "type": "function", "function": map[string]any{"name": nil, "arguments": `{"path":"README`}},
				}}},
			},
		},
		map[string]any{
			"id": "c",
			"choices": []any{
				map[string]any{"index": 0, "delta": map[string]any{"tool_calls": []any{
					map[string]any{"index": 0, "id": "chatcmpl-tool-b", "type": "function", "function": map[string]any{"name": nil, "arguments": `.md"}`}},
				}}, "finish_reason": "tool_calls"},
			},
		},
	}
	server := newSSEServer(t, chunks, nil)
	defer server.Close()

	m := testModel("local-vllm", server.URL, "")
	s := Stream(context.Background(), m, userTranscript("Read"), &Options{APIKey: "test"})
	result := s.Result()
	if result.StopReason != model.StopToolUse {
		t.Fatalf("stopReason = %q", result.StopReason)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content = %d blocks", len(result.Content))
	}
	call, ok := result.Content[0].(model.ToolCall)
	if !ok {
		t.Fatalf("content[0] = %T", result.Content[0])
	}
	if call.ID != "functions.read:0" || call.Name != "read" {
		t.Fatalf("call = %+v", call)
	}
	if call.Arguments["path"] != "README.md" {
		t.Fatalf("arguments = %v", call.Arguments)
	}
}

func TestMixedContentReasoningAndToolDeltas(t *testing.T) {
	chunks := []any{
		map[string]any{"id": "c", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{
			"content": "answer 1", "reasoning_content": "think 1",
			"tool_calls": []any{
				map[string]any{"index": 0, "id": "tc_read", "type": "function", "function": map[string]any{"name": "read", "arguments": `{"path":"README`}},
				map[string]any{"index": 1, "id": "tc_grep", "type": "function", "function": map[string]any{"name": "grep", "arguments": `{"pattern":"TODO`}},
			},
		}}}},
		map[string]any{"id": "c", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{
			"content": " answer 2", "reasoning_content": " think 2",
			"tool_calls": []any{
				map[string]any{"index": 0, "type": "function", "function": map[string]any{"arguments": `.md"}`}},
				map[string]any{"index": 1, "type": "function", "function": map[string]any{"arguments": `","path":"src"}`}},
			},
		}, "finish_reason": "tool_calls"}}},
	}
	server := newSSEServer(t, chunks, nil)
	defer server.Close()

	m := testModel("local-vllm", server.URL, "")
	s := Stream(context.Background(), m, userTranscript("Think"), &Options{APIKey: "test"})
	result := s.Result()
	if len(result.Content) != 4 {
		t.Fatalf("content = %d blocks, want 4: %#v", len(result.Content), result.Content)
	}
	text, _ := result.Content[0].(model.TextContent)
	if text.Text != "answer 1 answer 2" {
		t.Fatalf("text = %q", text.Text)
	}
	thinking, _ := result.Content[1].(model.ThinkingContent)
	if thinking.Thinking != "think 1 think 2" {
		t.Fatalf("thinking = %q", thinking.Thinking)
	}
	read, _ := result.Content[2].(model.ToolCall)
	if read.Arguments["path"] != "README.md" {
		t.Fatalf("read args = %v", read.Arguments)
	}
	grep, _ := result.Content[3].(model.ToolCall)
	if grep.Arguments["pattern"] != "TODO" || grep.Arguments["path"] != "src" {
		t.Fatalf("grep args = %v", grep.Arguments)
	}
}

func TestUsageParsing(t *testing.T) {
	chunks := []any{
		map[string]any{"id": "c", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "OK"}}}},
		map[string]any{"id": "c", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}},
			"usage": map[string]any{
				"prompt_tokens": 100, "completion_tokens": 5,
				"prompt_tokens_details":     map[string]any{"cached_tokens": 50, "cache_write_tokens": 30},
				"completion_tokens_details": map[string]any{"reasoning_tokens": 3},
			}},
	}
	server := newSSEServer(t, chunks, nil)
	defer server.Close()

	m := testModel("local-vllm", server.URL, "")
	result := Stream(context.Background(), m, userTranscript("hi"), &Options{APIKey: "test"}).Result()
	if result.Usage.Input != 20 || result.Usage.CacheRead != 50 || result.Usage.CacheWrite != 30 {
		t.Fatalf("usage = %+v", result.Usage)
	}
	if result.Usage.TotalTokens != 105 {
		t.Fatalf("total = %d", result.Usage.TotalTokens)
	}
	if result.Usage.Reasoning != 3 {
		t.Fatalf("reasoning = %d", result.Usage.Reasoning)
	}
}

func TestFinishReasonNetworkError(t *testing.T) {
	chunks := []any{
		map[string]any{"id": "c", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "partial"}, "finish_reason": nil}}},
		map[string]any{"id": "c", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "network_error"}}},
	}
	server := newSSEServer(t, chunks, nil)
	defer server.Close()

	m := testModel("local-vllm", server.URL, "")
	result := Stream(context.Background(), m, userTranscript("hi"), &Options{APIKey: "test"}).Result()
	if result.StopReason != model.StopError {
		t.Fatalf("stopReason = %q", result.StopReason)
	}
	if result.ErrorMessage != "Provider finish_reason: network_error" {
		t.Fatalf("errorMessage = %q", result.ErrorMessage)
	}
}

func TestStreamEndsWithoutFinishReason(t *testing.T) {
	chunks := []any{
		map[string]any{"id": "c", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "partial"}, "finish_reason": nil}}},
	}
	server := newSSEServer(t, chunks, nil)
	defer server.Close()

	m := testModel("local-vllm", server.URL, "")
	result := Stream(context.Background(), m, userTranscript("hi"), &Options{APIKey: "test"}).Result()
	if result.StopReason != model.StopError {
		t.Fatalf("stopReason = %q", result.StopReason)
	}
	if result.ErrorMessage != "Stream ended without finish_reason" {
		t.Fatalf("errorMessage = %q", result.ErrorMessage)
	}
}

func TestStreamWithoutFinishReasonWhenCompatDisables(t *testing.T) {
	chunks := []any{
		map[string]any{"id": "c", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "complete"}, "finish_reason": nil}}},
	}
	server := newSSEServer(t, chunks, nil)
	defer server.Close()

	m := testModel("local-vllm", server.URL, `{"supportsFinishReason":false}`)
	result := Stream(context.Background(), m, userTranscript("hi"), &Options{APIKey: "test"}).Result()
	if result.StopReason != model.StopStop {
		t.Fatalf("stopReason = %q (%s)", result.StopReason, result.ErrorMessage)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content = %#v", result.Content)
	}
}

func TestZaiReasoningFormat(t *testing.T) {
	var requests []map[string]any
	server := newSSEServer(t, stopChunks(), &requests)
	defer server.Close()

	maxEffort := "max"
	m := testModel("zai", server.URL, `{"thinkingFormat":"zai","supportsReasoningEffort":true}`)
	m.Reasoning = true
	m.ThinkingLevelMap = model.ThinkingLevelMap{"high": nil, "max": &maxEffort}
	s := Stream(context.Background(), m, userTranscript("hi"), &Options{
		APIKey:          "test",
		ReasoningEffort: "high",
	})
	_ = s.Result()
	thinking, _ := requests[0]["thinking"].(map[string]any)
	if thinking["type"] != "enabled" || thinking["clear_thinking"] != false {
		t.Fatalf("thinking = %v", thinking)
	}
	if _, present := requests[0]["reasoning_effort"]; present {
		t.Fatalf("reasoning_effort should be omitted for a null mapping: %v", requests[0]["reasoning_effort"])
	}
}

func TestOpenRouterReasoningObject(t *testing.T) {
	var requests []map[string]any
	server := newSSEServer(t, stopChunks(), &requests)
	defer server.Close()

	m := testModel("openrouter", server.URL, `{"thinkingFormat":"openrouter"}`)
	m.Reasoning = true
	s := Stream(context.Background(), m, userTranscript("hi"), &Options{
		APIKey:          "test",
		ReasoningEffort: "high",
	})
	_ = s.Result()
	reasoning, _ := requests[0]["reasoning"].(map[string]any)
	if reasoning["effort"] != "high" {
		t.Fatalf("reasoning = %v", reasoning)
	}
	if _, present := requests[0]["reasoning_effort"]; present {
		t.Fatalf("reasoning_effort should be absent")
	}
}

func TestReasoningReplayAsReasoningContent(t *testing.T) {
	m := testModel("zai", "http://localhost", "")
	m.Reasoning = true
	assistant := model.AssistantMessage{
		Api:      model.APIOpenAICompletions,
		Provider: "zai",
		Model:    "test-model",
		Content: model.ContentList{
			model.ThinkingContent{Thinking: "prior reasoning", ThinkingSignature: "reasoning_content"},
			model.ToolCall{ID: "call_1", Name: "read", Arguments: map[string]any{"path": "README.md"}},
		},
		StopReason: model.StopToolUse,
	}
	transcript := model.TranscriptContext{Messages: []model.Message{
		model.UserMessage{Content: model.ContentList{model.TextContent{Text: "Read"}}},
		assistant,
		model.ToolResultMessage{ToolCallID: "call_1", ToolName: "read", Content: model.ContentList{model.TextContent{Text: "contents"}}},
		model.UserMessage{Content: model.ContentList{model.TextContent{Text: "Continue"}}},
	}}
	messages, err := convertMessages(m, transcript, getCompat(m), map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	var replayed map[string]any
	for _, message := range messages {
		if message["role"] == "assistant" {
			replayed = message
			break
		}
	}
	if replayed["reasoning_content"] != "prior reasoning" {
		t.Fatalf("replayed = %#v", replayed)
	}
}

func TestOpenCodeGoReasoningSignatureNormalized(t *testing.T) {
	m := testModel("opencode-go", "https://opencode.ai/zen/go/v1", "")
	m.Reasoning = true
	assistant := model.AssistantMessage{
		Api:      model.APIOpenAICompletions,
		Provider: "opencode-go",
		Model:    "test-model",
		Content: model.ContentList{
			model.ThinkingContent{Thinking: "think", ThinkingSignature: "reasoning"},
			model.ToolCall{ID: "call_1", Name: "read", Arguments: map[string]any{"path": "README.md"}},
		},
		StopReason: model.StopStop,
	}
	transcript := model.TranscriptContext{Messages: []model.Message{assistant}}
	messages, err := convertMessages(m, transcript, getCompat(m), map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if messages[0]["reasoning_content"] != "think" {
		t.Fatalf("messages[0] = %#v", messages[0])
	}
	if _, present := messages[0]["reasoning"]; present {
		t.Fatalf("reasoning should not be present")
	}
}

func TestReasoningDetailsReplay(t *testing.T) {
	m := testModel("openrouter", "https://openrouter.ai/api/v1", "")
	details := `[{"type":"reasoning.text","text":"step","signature":"abc"}]`
	assistant := model.AssistantMessage{
		Api:      model.APIOpenAICompletions,
		Provider: "openrouter",
		Model:    "test-model",
		Content: model.ContentList{
			model.TextContent{Text: "step"},
			model.ThinkingContent{Thinking: "step", ThinkingSignature: details},
		},
		StopReason: model.StopStop,
	}
	messages, err := convertMessages(m, model.TranscriptContext{Messages: []model.Message{assistant}}, getCompat(m), map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := messages[0]["reasoning_details"].([]json.RawMessage)
	if !ok || len(raw) != 1 {
		t.Fatalf("reasoning_details = %#v", messages[0]["reasoning_details"])
	}
	if _, present := messages[0]["reasoning_text"]; present {
		t.Fatalf("raw reasoning field should not be set alongside reasoning_details")
	}
}

func TestChunkErrorSurfaces(t *testing.T) {
	chunks := []any{
		map[string]any{"error": map[string]any{"message": "boom"}},
	}
	server := newSSEServer(t, chunks, nil)
	defer server.Close()

	m := testModel("local-vllm", server.URL, "")
	result := Stream(context.Background(), m, userTranscript("hi"), &Options{APIKey: "test"}).Result()
	if result.StopReason != model.StopError || result.ErrorMessage != "boom" {
		t.Fatalf("result = %+v", result)
	}
}

func TestGrammarToolDelta(t *testing.T) {
	var requests []map[string]any
	chunks := []any{
		map[string]any{
			"id": "c",
			"choices": []any{
				map[string]any{"index": 0, "delta": map[string]any{"tool_calls": []any{
					map[string]any{"index": 0, "id": "call_1", "type": "custom", "custom": map[string]any{"name": "parse", "input": "ab"}},
				}}},
			},
		},
		map[string]any{
			"id": "c",
			"choices": []any{
				map[string]any{"index": 0, "delta": map[string]any{"tool_calls": []any{
					map[string]any{"index": 0, "type": "custom", "custom": map[string]any{"input": "c"}},
				}}, "finish_reason": "tool_calls"},
			},
		},
	}
	server := newSSEServer(t, chunks, &requests)
	defer server.Close()

	m := testModel("local-vllm", server.URL, `{"supportsOpenAIGrammarTools":true}`)
	tool := model.Tool{
		Name: "parse", Description: "Parse", Parameters: model.Object(model.Prop("input", model.String())),
		ConstrainedSampling: &model.ConstrainedSamplingConfig{
			Type:     model.ConstrainedSamplingGrammar,
			Variants: model.GrammarVariants{OpenAILark: "start: /[a-z]+/"},
		},
	}
	transcript := model.TranscriptContext{Messages: []model.Message{
		model.SystemMessage{Content: model.ContentList{model.TextContent{Text: "Follow"}}, ToolsAdded: []model.Tool{tool}},
		model.UserMessage{Content: model.ContentList{model.TextContent{Text: "Parse"}}},
	}}
	result := Stream(context.Background(), m, transcript, &Options{APIKey: "test"}).Result()
	if result.StopReason != model.StopToolUse {
		t.Fatalf("stopReason = %q (%s)", result.StopReason, result.ErrorMessage)
	}
	call, ok := result.Content[0].(model.ToolCall)
	if !ok {
		t.Fatalf("content[0] = %T", result.Content[0])
	}
	if call.Arguments["input"] != "abc" {
		t.Fatalf("arguments = %v", call.Arguments)
	}
}

func TestStreamSimpleForwardsOptions(t *testing.T) {
	var requests []map[string]any
	server := newSSEServer(t, stopChunks(), &requests)
	defer server.Close()

	m := testModel("local-vllm", server.URL, "")
	maxTokens := 123
	options := &model.SimpleStreamOptions{
		APIKey:     "test",
		ToolChoice: model.ToolChoiceNone,
	}
	options.MaxTokens = &maxTokens
	channel, err := StreamSimple(context.Background(), m, userTranscript("hi"), options)
	if err != nil {
		t.Fatal(err)
	}
	result := channel.(*wire.AssistantMessageEventStream).Result()
	if result.StopReason != model.StopStop {
		t.Fatalf("stopReason = %q (%s)", result.StopReason, result.ErrorMessage)
	}
	if requests[0]["max_completion_tokens"] != float64(123) {
		t.Fatalf("max_completion_tokens = %v", requests[0]["max_completion_tokens"])
	}
	if requests[0]["tool_choice"] != "none" {
		t.Fatalf("tool_choice = %v", requests[0]["tool_choice"])
	}
}

func TestOrphanedToolCallGetsSyntheticResult(t *testing.T) {
	m := testModel("local-vllm", "http://localhost", "")
	assistant := model.AssistantMessage{
		Api:      model.APIOpenAICompletions,
		Provider: "local-vllm",
		Model:    "test-model",
		Content: model.ContentList{
			model.ToolCall{ID: "call_1", Name: "read", Arguments: map[string]any{"path": "a"}},
		},
		StopReason: model.StopToolUse,
	}
	messages, err := convertMessages(m, model.TranscriptContext{Messages: []model.Message{
		model.UserMessage{Content: model.ContentList{model.TextContent{Text: "Read"}}},
		assistant,
	}}, getCompat(m), map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, message := range messages {
		if message["role"] != "tool" {
			continue
		}
		found = true
		if message["content"] != "No result provided" {
			t.Fatalf("synthetic content = %v", message["content"])
		}
	}
	if !found {
		t.Fatalf("no synthetic tool result: %#v", messages)
	}
}

func TestStreamUsesDiffusionAPIKey(t *testing.T) {
	t.Setenv("DIFFUSION_API_KEY", "secret-from-env")
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, streamBody(stopChunks()))
	}))
	defer server.Close()

	m := testModel("local-vllm", server.URL, "")
	result := Stream(context.Background(), m, userTranscript("hi"), &Options{}).Result()
	if result.StopReason != model.StopStop {
		t.Fatalf("stopReason = %q (%s)", result.StopReason, result.ErrorMessage)
	}
	if authorization != "Bearer secret-from-env" {
		t.Fatalf("authorization = %q", authorization)
	}
}

func TestStreamMissingAPIKeyFails(t *testing.T) {
	t.Setenv("DIFFUSION_API_KEY", "")
	m := testModel("local-vllm", "http://localhost:1", "")
	result := Stream(context.Background(), m, userTranscript("hi"), &Options{}).Result()
	if result.StopReason != model.StopError {
		t.Fatalf("stopReason = %q", result.StopReason)
	}
	if !strings.Contains(result.ErrorMessage, "No API key for provider") {
		t.Fatalf("errorMessage = %q", result.ErrorMessage)
	}
}
