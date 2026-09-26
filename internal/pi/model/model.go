package model

import "encoding/json"

// ModelCostRates holds per-million-token pricing rates.
type ModelCostRates struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

// ModelCostTier extends ModelCostRates with an input threshold.
type ModelCostTier struct {
	ModelCostRates
	InputTokensAbove int `json:"inputTokensAbove"`
}

// ModelCost holds per-million-token pricing plus optional request-wide tiers.
type ModelCost struct {
	ModelCostRates
	// Tiers are request-wide pricing overrides. The highest matching input
	// threshold applies to the full request.
	Tiers []ModelCostTier `json:"tiers,omitempty"`
}

// ModelImageResizeOptions is the cache-safe resize profile applied to an image
// before it enters conversation history.
type ModelImageResizeOptions struct {
	MaxWidth    *int `json:"maxWidth,omitempty"`
	MaxHeight   *int `json:"maxHeight,omitempty"`
	MaxBytes    *int `json:"maxBytes,omitempty"`
	JPEGQuality *int `json:"jpegQuality,omitempty"`
}

// ModelImageInputLimits describes what a provider accepts for image input.
type ModelImageInputLimits struct {
	Resize        *ModelImageResizeOptions `json:"resize,omitempty"`
	MaxPerMessage *int                     `json:"maxPerMessage,omitempty"`
	MaxPerRequest *int                     `json:"maxPerRequest,omitempty"`
}

// ModelInputLimits carries a provider's input limits.
type ModelInputLimits struct {
	MaxRequestBytes *int                   `json:"maxRequestBytes,omitempty"`
	Images          *ModelImageInputLimits `json:"images,omitempty"`
}

// ModelType is what a catalog entry is for.
type ModelType string

const (
	ModelTypeChat       ModelType = "chat"
	ModelTypeImage      ModelType = "image"
	ModelTypeClassifier ModelType = "classifier"
)

// Model describes a concrete model in the unified model system. One struct
// stands in for pi's AnyModel: Type says which operation the model serves.
type Model struct {
	Type             ModelType         `json:"type,omitempty"`
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Api              Api               `json:"api"`
	Provider         ProviderId        `json:"provider"`
	BaseURL          string            `json:"baseUrl"`
	Reasoning        bool              `json:"reasoning"`
	ThinkingLevelMap ThinkingLevelMap  `json:"thinkingLevelMap,omitempty"`
	Input            []string          `json:"input"`
	InputLimits      *ModelInputLimits `json:"inputLimits,omitempty"`
	Cost             ModelCost         `json:"cost"`
	PromptCache      *ModelPromptCache `json:"promptCache,omitempty"`
	ContextWindow    int               `json:"contextWindow"`
	MaxTokens        int               `json:"maxTokens"`
	SamplingParams   map[string]any    `json:"samplingParams,omitempty"`
	Headers          ProviderHeaders   `json:"headers,omitempty"`
	Compat           json.RawMessage   `json:"compat,omitempty"`
	// Output is meaningful for image models; always includes "image".
	Output []string `json:"output,omitempty"`
}

// GetModelType returns the model's type, treating empty and "chat" alike.
func (m Model) GetModelType() ModelType {
	if m.Type == "" {
		return ModelTypeChat
	}
	return m.Type
}

// Clone returns a deep copy of the model.
func (m Model) Clone() Model {
	out := m
	out.Input = append([]string(nil), m.Input...)
	out.Output = append([]string(nil), m.Output...)
	if m.ThinkingLevelMap != nil {
		out.ThinkingLevelMap = make(ThinkingLevelMap, len(m.ThinkingLevelMap))
		for k, v := range m.ThinkingLevelMap {
			if v == nil {
				out.ThinkingLevelMap[k] = nil
				continue
			}
			s := *v
			out.ThinkingLevelMap[k] = &s
		}
	}
	if m.Headers != nil {
		out.Headers = make(ProviderHeaders, len(m.Headers))
		for k, v := range m.Headers {
			if v == nil {
				out.Headers[k] = nil
				continue
			}
			s := *v
			out.Headers[k] = &s
		}
	}
	out.SamplingParams = cloneMap(m.SamplingParams)
	out.Compat = append(json.RawMessage(nil), m.Compat...)
	if m.InputLimits != nil {
		v := *m.InputLimits
		out.InputLimits = &v
	}
	if m.PromptCache != nil {
		v := *m.PromptCache
		out.PromptCache = &v
	}
	return out
}

// ImageModel is an image-generation model.
type ImageModel struct {
	Model
}

// ClassifierModel is a structured classifier model.
type ClassifierModel struct {
	Model
}
