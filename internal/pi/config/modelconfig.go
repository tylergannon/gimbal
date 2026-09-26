package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// ModelsJSONModel is one custom model definition in models.json.
type ModelsJSONModel struct {
	ID               string                  `json:"id"`
	Name             string                  `json:"name,omitempty"`
	Api              string                  `json:"api,omitempty"`
	BaseURL          string                  `json:"baseUrl,omitempty"`
	Reasoning        *bool                   `json:"reasoning,omitempty"`
	ThinkingLevelMap model.ThinkingLevelMap  `json:"thinkingLevelMap,omitempty"`
	Input            []string                `json:"input,omitempty"`
	InputLimits      *model.ModelInputLimits `json:"inputLimits,omitempty"`
	Cost             *model.ModelCost        `json:"cost,omitempty"`
	PromptCache      *model.ModelPromptCache `json:"promptCache,omitempty"`
	ContextWindow    *int                    `json:"contextWindow,omitempty"`
	MaxTokens        *int                    `json:"maxTokens,omitempty"`
	SamplingParams   map[string]any          `json:"samplingParams,omitempty"`
	Headers          map[string]string       `json:"headers,omitempty"`
	Compat           json.RawMessage         `json:"compat,omitempty"`
}

// ModelsJSONModelOverride is a modelOverrides entry in models.json.
type ModelsJSONModelOverride struct {
	Name             string                  `json:"name,omitempty"`
	Reasoning        *bool                   `json:"reasoning,omitempty"`
	ThinkingLevelMap model.ThinkingLevelMap  `json:"thinkingLevelMap,omitempty"`
	Input            []string                `json:"input,omitempty"`
	InputLimits      *model.ModelInputLimits `json:"inputLimits,omitempty"`
	Cost             *model.ModelCost        `json:"cost,omitempty"`
	PromptCache      *model.ModelPromptCache `json:"promptCache,omitempty"`
	ContextWindow    *int                    `json:"contextWindow,omitempty"`
	MaxTokens        *int                    `json:"maxTokens,omitempty"`
	SamplingParams   map[string]any          `json:"samplingParams,omitempty"`
	Headers          map[string]string       `json:"headers,omitempty"`
	Compat           json.RawMessage         `json:"compat,omitempty"`
}

// ModelsJSONProvider is one provider block in models.json.
type ModelsJSONProvider struct {
	Name           string                             `json:"name,omitempty"`
	BaseURL        string                             `json:"baseUrl,omitempty"`
	APIKey         string                             `json:"apiKey,omitempty"`
	Api            string                             `json:"api,omitempty"`
	OAuth          string                             `json:"oauth,omitempty"`
	Headers        map[string]string                  `json:"headers,omitempty"`
	Compat         json.RawMessage                    `json:"compat,omitempty"`
	AuthHeader     *bool                              `json:"authHeader,omitempty"`
	Models         []ModelsJSONModel                  `json:"models,omitempty"`
	ModelOverrides map[string]ModelsJSONModelOverride `json:"modelOverrides,omitempty"`
}

type modelsJSONConfig struct {
	Providers map[string]ModelsJSONProvider `json:"providers"`
}

// ModelConfig is one immutable load of models.json. The zero value is an empty
// configuration.
type ModelConfig struct {
	providers map[string]ModelsJSONProvider
	err       string
}

// LoadModelConfig reads and validates models.json. A missing path or file
// yields an empty configuration; a parse or schema error is recorded and
// returned by GetError.
func LoadModelConfig(modelsJSONPath string) *ModelConfig {
	config := &ModelConfig{providers: map[string]ModelsJSONProvider{}}
	if modelsJSONPath == "" {
		return config
	}
	path := NormalizePath(modelsJSONPath)
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return config
		}
		config.err = fmt.Sprintf("Failed to load models.json: %s\n\nFile: %s", err, path)
		return config
	}

	var parsed modelsJSONConfig
	decoder := json.NewDecoder(strings.NewReader(StripJSONComments(StripBOM(string(content)))))
	if err := decoder.Decode(&parsed); err != nil {
		config.err = fmt.Sprintf("Failed to parse models.json: %s\n\nFile: %s", err, path)
		return config
	}
	if parsed.Providers == nil {
		return config
	}
	for providerID, provider := range parsed.Providers {
		if providerID == "" {
			config.err = fmt.Sprintf("Invalid models.json schema:\n  - providers: provider id is required\n\nFile: %s", path)
			return config
		}
		config.providers[providerID] = provider
	}
	return config
}

// NewModelConfig builds an in-memory configuration from providers.
func NewModelConfig(providers map[string]ModelsJSONProvider) *ModelConfig {
	if providers == nil {
		providers = map[string]ModelsJSONProvider{}
	}
	return &ModelConfig{providers: providers}
}

// GetProvider returns a provider block, or nil when absent.
func (c *ModelConfig) GetProvider(providerID string) *ModelsJSONProvider {
	if c == nil {
		return nil
	}
	provider, ok := c.providers[providerID]
	if !ok {
		return nil
	}
	return &provider
}

// GetProviderIDs returns the configured provider ids.
func (c *ModelConfig) GetProviderIDs() []string {
	if c == nil {
		return nil
	}
	ids := make([]string, 0, len(c.providers))
	for providerID := range c.providers {
		ids = append(ids, providerID)
	}
	return ids
}

// GetError returns the load or validation error, or "" when the config is valid.
func (c *ModelConfig) GetError() string {
	if c == nil {
		return ""
	}
	return c.err
}
