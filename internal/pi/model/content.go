package model

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Content is a single content block. It is implemented by TextContent,
// ThinkingContent, ImageContent and ToolCall.
type Content interface {
	ContentType() string
}

// TextSignatureV1 is the structured form of TextContent.TextSignature.
type TextSignatureV1 struct {
	V     int    `json:"v"`
	ID    string `json:"id"`
	Phase string `json:"phase,omitempty"`
}

// TextContent is a text block.
type TextContent struct {
	Text string `json:"text"`
	// TextSignature carries provider message metadata, e.g. an OpenAI
	// responses message id (a legacy string) or a TextSignatureV1 JSON.
	TextSignature string `json:"textSignature,omitempty"`
}

func (TextContent) ContentType() string { return "text" }

// ThinkingContent is a reasoning/thinking block.
type ThinkingContent struct {
	Thinking string `json:"thinking"`
	// ThinkingSignature carries provider-specific opaque or serialized
	// reasoning replay data. Treat it as opaque and replay it unmodified.
	ThinkingSignature string `json:"thinkingSignature,omitempty"`
	// Redacted marks thinking content removed by safety filters; the opaque
	// encrypted payload is kept in ThinkingSignature for multi-turn continuity.
	Redacted bool `json:"redacted,omitempty"`
}

func (ThinkingContent) ContentType() string { return "thinking" }

// ImageContent is a base64-encoded image block.
type ImageContent struct {
	Data     string `json:"data"`
	MimeType string `json:"mimeType"`
}

func (ImageContent) ContentType() string { return "image" }

// JsonObject is a JSON object value.
type JsonObject = map[string]any

// ToolCall is a tool invocation requested by the assistant.
type ToolCall struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Arguments JsonObject `json:"arguments"`
	// ThoughtSignature is a Google-specific opaque signature for reusing
	// thought context.
	ThoughtSignature string `json:"thoughtSignature,omitempty"`
	// Namespace is the OpenAI Responses namespace for calls to dynamically
	// loaded or namespaced tools.
	Namespace string `json:"namespace,omitempty"`
}

func (ToolCall) ContentType() string { return "toolCall" }

// ContentList is a slice of heterogeneous content blocks with discriminated
// JSON. A nil block is a hole in the array and serializes to null.
type ContentList []Content

// MarshalJSON encodes each block with a "type" discriminator first.
func (cl ContentList) MarshalJSON() ([]byte, error) {
	if cl == nil {
		return []byte("[]"), nil
	}
	parts := make([]json.RawMessage, len(cl))
	for i, c := range cl {
		raw, err := marshalContent(c)
		if err != nil {
			return nil, err
		}
		parts[i] = raw
	}
	return json.Marshal(parts)
}

// UnmarshalJSON decodes a discriminated content array.
func (cl *ContentList) UnmarshalJSON(data []byte) error {
	var raws []json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return err
	}
	out := make(ContentList, 0, len(raws))
	for _, raw := range raws {
		c, err := unmarshalContent(raw)
		if err != nil {
			return err
		}
		out = append(out, c)
	}
	*cl = out
	return nil
}

// marshalContent serializes a content block with its "type" discriminator
// first and the block's own fields after it.
func marshalContent(c Content) ([]byte, error) {
	if c == nil {
		return []byte("null"), nil
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '{' || raw[len(raw)-1] != '}' {
		return nil, fmt.Errorf("model: %s content block did not serialize to a JSON object: %s", c.ContentType(), raw)
	}
	t, _ := json.Marshal(c.ContentType())
	var buf bytes.Buffer
	buf.WriteString(`{"type":`)
	buf.Write(t)
	if fields := bytes.TrimSpace(raw[1 : len(raw)-1]); len(fields) > 0 {
		buf.WriteByte(',')
		buf.Write(fields)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// unmarshalContent decodes a content block based on its "type" discriminator.
func unmarshalContent(data []byte) (Content, error) {
	if string(bytes.TrimSpace(data)) == "null" {
		return nil, nil
	}
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, err
	}
	switch head.Type {
	case "text":
		var c TextContent
		err := json.Unmarshal(data, &c)
		return c, err
	case "thinking":
		var c ThinkingContent
		err := json.Unmarshal(data, &c)
		return c, err
	case "image":
		var c ImageContent
		err := json.Unmarshal(data, &c)
		return c, err
	case "toolCall":
		var c ToolCall
		err := json.Unmarshal(data, &c)
		return c, err
	default:
		return nil, fmt.Errorf("model: unknown content type: %q", head.Type)
	}
}

// TextContentText returns the text of the first text block. It is a helper for
// code that expects plain text content.
func (cl ContentList) TextContentText() (string, bool) {
	for _, c := range cl {
		if t, ok := c.(TextContent); ok {
			return t.Text, true
		}
	}
	return "", false
}

// Clone returns a deep copy of the content list.
func (cl ContentList) Clone() ContentList {
	if cl == nil {
		return nil
	}
	out := make(ContentList, len(cl))
	for i, c := range cl {
		out[i] = CloneContent(c)
	}
	return out
}

// CloneContent returns a deep copy of a content block.
func CloneContent(c Content) Content {
	switch t := c.(type) {
	case nil:
		return nil
	case TextContent:
		return t
	case *TextContent:
		if t == nil {
			return nil
		}
		v := *t
		return v
	case ThinkingContent:
		return t
	case *ThinkingContent:
		if t == nil {
			return nil
		}
		v := *t
		return v
	case ImageContent:
		return t
	case *ImageContent:
		if t == nil {
			return nil
		}
		v := *t
		return v
	case ToolCall:
		return t.Clone()
	case *ToolCall:
		if t == nil {
			return nil
		}
		v := t.Clone()
		return v
	default:
		return c
	}
}

// Clone returns a deep copy of the tool call and its arguments.
func (t ToolCall) Clone() ToolCall {
	out := t
	out.Arguments = cloneMap(t.Arguments)
	return out
}

// cloneMap deep-copies a JSON-ish map through explicit recursion.
func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = cloneValue(v)
	}
	return out
}

// cloneValue deep-copies a JSON-ish value.
func cloneValue(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case map[string]any:
		return cloneMap(t)
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = cloneValue(e)
		}
		return out
	case json.RawMessage:
		return append(json.RawMessage(nil), t...)
	case []byte:
		return append([]byte(nil), t...)
	case string, bool, float64, int, int64, json.Number:
		return t
	default:
		return t
	}
}
