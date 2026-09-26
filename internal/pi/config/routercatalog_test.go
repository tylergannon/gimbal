package config

import (
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

const routerCatalogJSON = `{
  "object": "list",
  "data": [
    {"id": "chat-no-tools", "capabilities": {"openai_chat": true}},
    {"id": "anthropic-only", "capabilities": {"anthropic_messages": true, "tools": true}},
    {"id": "emb-granite", "object": "model", "capabilities": {"embeddings": true}},
    {"id": "glm-5.3", "object": "model", "context_window": 524288, "max_output_tokens": 131072,
     "capabilities": {"openai_chat": true, "anthropic_messages": true, "reasoning": true, "tools": true, "vision": true},
     "client_compat": {"pi": {"maxTokensField": "max_tokens", "supportsReasoningEffort": true, "thinkingFormat": "zai",
       "thinkingLevelMap": {"high": "high", "off": "none", "xhigh": "max"}}}},
    {"id": "flux2-klein", "object": "model", "capabilities": {"image_generation": true}}
  ]
}`

func TestParseRouterCatalog(t *testing.T) {
	catalog, err := ParseRouterCatalog([]byte(routerCatalogJSON), "https://router.test")
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Models) != 1 {
		t.Fatalf("models = %d; want 1", len(catalog.Models))
	}
	glm, ok := catalog.Get("glm-5.3")
	if !ok {
		t.Fatal("glm-5.3 not found")
	}
	if glm.Provider != ProviderDiffusion || glm.Api != model.APIOpenAICompletions || glm.BaseURL != "https://router.test" {
		t.Fatalf("glm = %+v", glm)
	}
	if glm.ContextWindow != 524288 || glm.MaxTokens != 131072 || !glm.Reasoning {
		t.Fatalf("glm metadata = %+v", glm)
	}
	if len(glm.Input) != 2 || glm.Input[1] != "image" {
		t.Fatalf("glm input = %+v", glm.Input)
	}
	if glm.ThinkingLevelMap == nil || glm.ThinkingLevelMap[model.ModelThinkingLevel("xhigh")] == nil || *glm.ThinkingLevelMap[model.ModelThinkingLevel("xhigh")] != "max" {
		t.Fatalf("thinking map = %+v", glm.ThinkingLevelMap)
	}
	if glm.Compat == nil {
		t.Fatal("compat was not preserved")
	}

	if got := catalog.DefaultChatModel(); got == nil || got.ID != "glm-5.3" {
		t.Fatalf("default chat model = %+v", got)
	}
	if len(catalog.ChatModels()) != 1 {
		t.Fatalf("chat models = %+v", catalog.ChatModels())
	}
	if _, ok := catalog.Get("flux2-klein"); ok {
		t.Fatal("image-only model admitted to the coding catalog")
	}
}
