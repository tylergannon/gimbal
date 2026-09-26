package openai

import (
	"bytes"
	"encoding/json"
)

// reasoning_details replay handling, ported from openai-completions.ts
// (upstream d6af72e1). A detail is provider-opaque: its shape is validated and
// its members are carried whole so a sequence replays unmodified.

var openAICompletionsReasoningFields = map[string]bool{
	"reasoning":         true,
	"reasoning_content": true,
	"reasoning_text":    true,
}

// isOpenAICompletionsReasoningField reports whether a thinking signature names
// a raw reasoning field on the assistant message.
func isOpenAICompletionsReasoningField(field string) bool {
	return openAICompletionsReasoningFields[field]
}

func isJSONStringRaw(raw json.RawMessage) (string, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '"' {
		return "", false
	}
	var s string
	if json.Unmarshal(trimmed, &s) != nil {
		return "", false
	}
	return s, true
}

func isJSONNullRaw(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func isJSONNumberRaw(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return false
	}
	if c := trimmed[0]; c != '-' && (c < '0' || c > '9') {
		return false
	}
	var n json.Number
	return json.Unmarshal(trimmed, &n) == nil
}

func reasoningDetailFields(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(trimmed, &fields) != nil {
		return nil, false
	}
	return fields, true
}

func hasValidCommonReasoningDetailFields(fields map[string]json.RawMessage) bool {
	if id, ok := fields["id"]; ok && !isJSONNullRaw(id) {
		if _, ok := isJSONStringRaw(id); !ok {
			return false
		}
	}
	if format, ok := fields["format"]; ok {
		if _, ok := isJSONStringRaw(format); !ok {
			return false
		}
	}
	if index, ok := fields["index"]; ok && !isJSONNumberRaw(index) {
		return false
	}
	return true
}

// isOpenAIReasoningDetail reports whether raw is one of the three accepted
// reasoning-detail shapes.
func isOpenAIReasoningDetail(raw json.RawMessage) bool {
	fields, ok := reasoningDetailFields(raw)
	if !ok || !hasValidCommonReasoningDetailFields(fields) {
		return false
	}
	detailType, _ := isJSONStringRaw(fields["type"])
	switch detailType {
	case "reasoning.summary":
		_, ok := isJSONStringRaw(fields["summary"])
		return ok
	case "reasoning.encrypted":
		_, ok := isJSONStringRaw(fields["data"])
		return ok
	case "reasoning.text":
		if _, ok := isJSONStringRaw(fields["text"]); !ok {
			return false
		}
		signature, ok := fields["signature"]
		if !ok || isJSONNullRaw(signature) {
			return true
		}
		_, ok = isJSONStringRaw(signature)
		return ok
	default:
		return false
	}
}

// parseOpenAIReasoningDetails reads the reasoning-detail sequence serialized in
// a thinking block's signature, or nil when it is not a non-empty array of
// valid details.
func parseOpenAIReasoningDetails(signature string) []json.RawMessage {
	if signature == "" {
		return nil
	}
	var details []json.RawMessage
	if json.Unmarshal([]byte(signature), &details) != nil || len(details) == 0 {
		return nil
	}
	for _, detail := range details {
		if !isOpenAIReasoningDetail(detail) {
			return nil
		}
	}
	return details
}

// parseLegacyEncryptedReasoningDetail reads the pre-sequence session shape: one
// encrypted detail stored on a tool call's thought signature.
func parseLegacyEncryptedReasoningDetail(signature string) (json.RawMessage, bool) {
	if signature == "" {
		return nil, false
	}
	raw := json.RawMessage(signature)
	fields, ok := reasoningDetailFields(raw)
	if !ok || !isOpenAIReasoningDetail(raw) {
		return nil, false
	}
	if detailType, _ := isJSONStringRaw(fields["type"]); detailType != "reasoning.encrypted" {
		return nil, false
	}
	if id, ok := isJSONStringRaw(fields["id"]); !ok || id == "" {
		return nil, false
	}
	if data, _ := isJSONStringRaw(fields["data"]); data == "" {
		return nil, false
	}
	return raw, true
}

// marshalOpenAIReasoningDetails serializes a sequence into a thinking
// signature.
func marshalOpenAIReasoningDetails(details []json.RawMessage) string {
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, detail := range details {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.Write(detail)
	}
	buf.WriteByte(']')
	return buf.String()
}

// appendOpenAIReasoningDetail appends a detail, merging it into the previous
// entry when both are text or both are summary (consecutive deltas form one
// logical entry).
func appendOpenAIReasoningDetail(details []json.RawMessage, detail json.RawMessage) []json.RawMessage {
	if len(details) > 0 {
		if merged, ok := mergeOpenAIReasoningDetail(details[len(details)-1], detail); ok {
			details[len(details)-1] = merged
			return details
		}
	}
	return append(details, detail)
}

func decodeReasoningDetailObject(raw json.RawMessage) (map[string]any, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, false
	}
	var object map[string]any
	if json.Unmarshal(trimmed, &object) != nil {
		return nil, false
	}
	return object, true
}

func mergeOpenAIReasoningDetail(last, detail json.RawMessage) (json.RawMessage, bool) {
	target, ok := decodeReasoningDetailObject(last)
	if !ok {
		return nil, false
	}
	source, ok := decodeReasoningDetailObject(detail)
	if !ok {
		return nil, false
	}
	detailType, _ := target["type"].(string)
	sourceType, _ := source["type"].(string)
	if detailType != sourceType {
		return nil, false
	}
	switch detailType {
	case "reasoning.text":
		text, _ := target["text"].(string)
		extra, _ := source["text"].(string)
		target["text"] = text + extra
		if isFalsyJSONValue(target["signature"]) {
			target["signature"] = source["signature"]
		}
	case "reasoning.summary":
		summary, _ := target["summary"].(string)
		extra, _ := source["summary"].(string)
		target["summary"] = summary + extra
	default:
		return nil, false
	}
	fillMissingReasoningDetailField(target, source, "id", true)
	fillMissingReasoningDetailField(target, source, "format", false)
	fillMissingReasoningDetailField(target, source, "index", true)
	merged, err := json.Marshal(target)
	if err != nil {
		return nil, false
	}
	return merged, true
}

func fillMissingReasoningDetailField(target, source map[string]any, key string, nullishOnly bool) {
	existing, present := target[key]
	if nullishOnly {
		if present && existing != nil {
			return
		}
	} else if present && !isFalsyJSONValue(existing) {
		return
	}
	if value, ok := source[key]; ok {
		target[key] = value
	}
}

func isFalsyJSONValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case bool:
		return !v
	case float64:
		return v == 0
	case string:
		return v == ""
	default:
		return false
	}
}
