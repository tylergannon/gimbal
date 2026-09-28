package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// ProviderDiffusion is the provider id of the Diffusion Router.
const ProviderDiffusion model.ProviderId = "diffusion"

// DefaultDiffusionBaseURL is the router API base URL used when no base URL is
// configured. It can be overridden with DIFFUSION_BASE_URL.
const DefaultDiffusionBaseURL = "https://router.diffusion.io/v1"

type routerModelList struct {
	Data []routerModel `json:"data"`
}

type routerCapabilities map[string]bool

func (capabilities *routerCapabilities) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	switch {
	case bytes.Equal(data, []byte("null")):
		*capabilities = nil
		return nil
	case len(data) > 0 && data[0] == '{':
		return json.Unmarshal(data, (*map[string]bool)(capabilities))
	case len(data) > 0 && data[0] == '[':
		var names []string
		if err := json.Unmarshal(data, &names); err != nil {
			return err
		}
		*capabilities = make(routerCapabilities, len(names))
		for _, name := range names {
			(*capabilities)[name] = true
		}
		return nil
	default:
		return errors.New("unsupported capabilities shape")
	}
}

type routerModel struct {
	ID              string             `json:"id"`
	Object          string             `json:"object"`
	Created         int64              `json:"created"`
	OwnedBy         string             `json:"owned_by"`
	ContextWindow   int                `json:"context_window"`
	MaxOutputTokens int                `json:"max_output_tokens"`
	Capabilities    routerCapabilities `json:"capabilities"`
	ClientCompat    struct {
		Pi json.RawMessage `json:"pi"`
	} `json:"client_compat"`
}

type piClientCompat struct {
	MaxTokensField          string             `json:"maxTokensField"`
	SupportsReasoningEffort bool               `json:"supportsReasoningEffort"`
	ThinkingFormat          string             `json:"thinkingFormat"`
	ThinkingLevelMap        map[string]*string `json:"thinkingLevelMap"`
}

// RouterCatalog is a parsed Diffusion Router model list.
type RouterCatalog struct {
	Models []*model.Model
	byID   map[string]*model.Model
}

// LoadRouterCatalog reads an OpenAI-style /models list and converts every entry
// into a model.Model for the Diffusion provider. The base URL is applied to
// every model; an empty baseURL falls back to DIFFUSION_BASE_URL and then
// DefaultDiffusionBaseURL.
func LoadRouterCatalog(path, baseURL string) (*RouterCatalog, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseRouterCatalog(content, baseURL)
}

// ParseRouterCatalog parses router model-list bytes.
func ParseRouterCatalog(content []byte, baseURL string) (*RouterCatalog, error) {
	if baseURL == "" {
		baseURL = os.Getenv("DIFFUSION_BASE_URL")
	}
	if baseURL == "" {
		baseURL = DefaultDiffusionBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	var list routerModelList
	if err := json.Unmarshal(content, &list); err != nil {
		return nil, err
	}
	catalog := &RouterCatalog{byID: map[string]*model.Model{}}
	for _, entry := range list.Data {
		if entry.ID == "" {
			continue
		}
		if !entry.supported() {
			continue
		}
		converted := entry.toModel(baseURL)
		catalog.Models = append(catalog.Models, converted)
		catalog.byID[converted.ID] = converted
	}
	if len(catalog.Models) == 0 {
		return catalog, errors.New("router catalog has no models")
	}
	return catalog, nil
}

// supported reports whether the port knows how to run the model. Embedding and
// rerank models are outside the Diffusion Router chat scope.
func (entry routerModel) supported() bool {
	return (entry.Capabilities["openai_chat"] || entry.Capabilities["chat"]) && entry.Capabilities["tools"]
}

func (entry routerModel) toModel(baseURL string) *model.Model {
	converted := &model.Model{
		ID:            entry.ID,
		Name:          entry.ID,
		Provider:      ProviderDiffusion,
		BaseURL:       baseURL,
		Type:          model.ModelTypeChat,
		Input:         []string{"text"},
		ContextWindow: entry.ContextWindow,
		MaxTokens:     entry.MaxOutputTokens,
		Reasoning:     entry.Capabilities["reasoning"],
	}
	if entry.Capabilities["vision"] {
		converted.Input = append(converted.Input, "image")
	}
	converted.Api = model.APIOpenAICompletions

	if len(entry.ClientCompat.Pi) > 0 {
		converted.Compat = append(json.RawMessage(nil), entry.ClientCompat.Pi...)
		var compat piClientCompat
		if err := json.Unmarshal(entry.ClientCompat.Pi, &compat); err == nil && compat.ThinkingLevelMap != nil {
			converted.ThinkingLevelMap = make(model.ThinkingLevelMap, len(compat.ThinkingLevelMap))
			for level, value := range compat.ThinkingLevelMap {
				converted.ThinkingLevelMap[model.ModelThinkingLevel(level)] = value
			}
		}
	}
	return converted
}

// Get returns a model by id.
func (c *RouterCatalog) Get(id string) (*model.Model, bool) {
	if c == nil {
		return nil, false
	}
	converted, ok := c.byID[id]
	return converted, ok
}

// ChatModels returns the chat-capable models in catalog order.
func (c *RouterCatalog) ChatModels() []*model.Model {
	if c == nil {
		return nil
	}
	var out []*model.Model
	for _, entry := range c.Models {
		if entry.GetModelType() == model.ModelTypeChat {
			out = append(out, entry)
		}
	}
	return out
}

// DefaultChatModel returns the first chat-capable model in catalog order, or
// nil when the catalog has none. The router list order is the source of the
// default.
func (c *RouterCatalog) DefaultChatModel() *model.Model {
	chatModels := c.ChatModels()
	if len(chatModels) == 0 {
		return nil
	}
	return chatModels[0]
}
