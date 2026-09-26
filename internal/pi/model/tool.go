package model

import "encoding/json"

// GrammarFormat names an OpenAI constrained-sampling grammar variant.
type GrammarFormat string

const (
	GrammarOpenAILark  GrammarFormat = "openai_lark"
	GrammarOpenAIRegex GrammarFormat = "openai_regex"
)

// GrammarVariants holds provider-specific encodings of the same grammar.
type GrammarVariants struct {
	OpenAILark  string `json:"openai_lark,omitempty"`
	OpenAIRegex string `json:"openai_regex,omitempty"`
}

// ConstrainedSamplingType is the discriminant of ConstrainedSamplingConfig.
type ConstrainedSamplingType string

const (
	ConstrainedSamplingJSONSchema ConstrainedSamplingType = "json_schema"
	ConstrainedSamplingGrammar    ConstrainedSamplingType = "grammar"
)

// ConstrainedSamplingStrictness is how hard a json_schema config insists.
type ConstrainedSamplingStrictness string

const (
	ConstrainedSamplingPrefer  ConstrainedSamplingStrictness = "prefer"
	ConstrainedSamplingRequire ConstrainedSamplingStrictness = "require"
)

// ConstrainedSamplingConfig is an optional provider-side constrained-sampling
// config for a tool. The zero value marshals to JSON false, pi's "explicitly
// unconstrained" spelling.
type ConstrainedSamplingConfig struct {
	Type     ConstrainedSamplingType       `json:"type"`
	Strict   ConstrainedSamplingStrictness `json:"strict"`
	Variants GrammarVariants               `json:"variants"`
}

// MarshalJSON emits only the fields belonging to the configured Type.
func (c ConstrainedSamplingConfig) MarshalJSON() ([]byte, error) {
	switch c.Type {
	case ConstrainedSamplingJSONSchema:
		strict := c.Strict
		if strict == "" {
			strict = ConstrainedSamplingPrefer
		}
		return json.Marshal(struct {
			Type   ConstrainedSamplingType       `json:"type"`
			Strict ConstrainedSamplingStrictness `json:"strict"`
		}{c.Type, strict})
	case ConstrainedSamplingGrammar:
		return json.Marshal(struct {
			Type     ConstrainedSamplingType `json:"type"`
			Variants GrammarVariants         `json:"variants"`
		}{c.Type, c.Variants})
	case "":
		return []byte("false"), nil
	default:
		return json.Marshal(struct {
			Type     ConstrainedSamplingType       `json:"type"`
			Strict   ConstrainedSamplingStrictness `json:"strict,omitempty"`
			Variants GrammarVariants               `json:"variants"`
		}{c.Type, c.Strict, c.Variants})
	}
}

// UnmarshalJSON accepts pi's false spelling for "no constrained sampling".
func (c *ConstrainedSamplingConfig) UnmarshalJSON(data []byte) error {
	trimmed := string(trimSpace(data))
	if trimmed == "false" || trimmed == "null" {
		*c = ConstrainedSamplingConfig{}
		return nil
	}
	type alias ConstrainedSamplingConfig
	return json.Unmarshal(data, (*alias)(c))
}

// Tool is a tool definition exposed to the model.
type Tool struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Parameters  *Schema `json:"parameters"`
	// ConstrainedSampling optionally asks the provider to constrain sampling of
	// this tool's input. Nil (and the zero config) leaves sampling unconstrained.
	ConstrainedSampling *ConstrainedSamplingConfig `json:"constrainedSampling,omitempty"`
}

// Clone returns a deep copy of the tool.
func (t Tool) Clone() Tool {
	out := t
	out.Parameters = t.Parameters.Clone()
	if t.ConstrainedSampling != nil {
		v := *t.ConstrainedSampling
		out.ConstrainedSampling = &v
	}
	return out
}

// ToolReference names a tool without its definition.
type ToolReference struct {
	Name string `json:"name"`
}

// Context is the request input the public stream entry points accept.
type Context struct {
	SystemPrompt string    `json:"systemPrompt,omitempty"`
	Messages     []Message `json:"messages"`
	Tools        []Tool    `json:"tools,omitempty"`
}

// TranscriptContext is the normalized request context passed to providers and
// API implementations: the prompt and tool declarations are carried by the
// transcript's system messages. Only NormalizeContext produces one.
type TranscriptContext struct {
	Messages []Message `json:"messages"`
}

// MarshalJSON writes {"messages":[...]}; a nil transcript is the empty array.
func (c TranscriptContext) MarshalJSON() ([]byte, error) {
	messages := c.Messages
	if messages == nil {
		messages = []Message{}
	}
	return json.Marshal(struct {
		Messages []Message `json:"messages"`
	}{messages})
}

// UnmarshalJSON decodes a transcript context.
func (c *TranscriptContext) UnmarshalJSON(data []byte) error {
	var raw struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	messages, err := unmarshalMessages(raw.Messages)
	if err != nil {
		return err
	}
	c.Messages = messages
	return nil
}

func trimSpace(data []byte) []byte {
	start := 0
	for start < len(data) && isSpaceByte(data[start]) {
		start++
	}
	end := len(data)
	for end > start && isSpaceByte(data[end-1]) {
		end--
	}
	return data[start:end]
}

func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
