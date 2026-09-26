package compact

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// Shared utilities for compaction and branch summarization, ported from
// packages/coding-agent/src/core/compaction/utils.ts (upstream d6af72e1).

// FileOperations tracks the files read, written and edited in a conversation.
type FileOperations struct {
	Read    map[string]struct{}
	Written map[string]struct{}
	Edited  map[string]struct{}
}

// NewFileOperations returns an empty FileOperations ready for use.
func NewFileOperations() *FileOperations {
	return &FileOperations{
		Read:    map[string]struct{}{},
		Written: map[string]struct{}{},
		Edited:  map[string]struct{}{},
	}
}

// ExtractFileOpsFromMessage collects paths from read/write/edit tool calls in an
// assistant message.
func ExtractFileOpsFromMessage(message model.AgentMessage, fileOps *FileOperations) {
	assistant, ok := assistantMessageOf(message)
	if !ok {
		return
	}
	for _, content := range assistant.Content {
		toolCall, ok := content.(model.ToolCall)
		if !ok {
			continue
		}
		path, _ := toolCall.Arguments["path"].(string)
		if path == "" {
			continue
		}
		switch toolCall.Name {
		case "read":
			fileOps.Read[path] = struct{}{}
		case "write":
			fileOps.Written[path] = struct{}{}
		case "edit":
			fileOps.Edited[path] = struct{}{}
		}
	}
}

// ComputeFileLists derives the read-only and modified file lists, each sorted.
// A file that was both read and modified counts as modified.
func ComputeFileLists(fileOps *FileOperations) (readFiles, modifiedFiles []string) {
	modified := map[string]struct{}{}
	for file := range fileOps.Edited {
		modified[file] = struct{}{}
	}
	for file := range fileOps.Written {
		modified[file] = struct{}{}
	}
	for file := range fileOps.Read {
		if _, ok := modified[file]; !ok {
			readFiles = append(readFiles, file)
		}
	}
	for file := range modified {
		modifiedFiles = append(modifiedFiles, file)
	}
	sort.Strings(readFiles)
	sort.Strings(modifiedFiles)
	return readFiles, modifiedFiles
}

// FormatFileOperations formats read/modified file lists as XML tags appended to
// a summary.
func FormatFileOperations(readFiles, modifiedFiles []string) string {
	var sections []string
	if len(readFiles) > 0 {
		sections = append(sections, "<read-files>\n"+strings.Join(readFiles, "\n")+"\n</read-files>")
	}
	if len(modifiedFiles) > 0 {
		sections = append(sections, "<modified-files>\n"+strings.Join(modifiedFiles, "\n")+"\n</modified-files>")
	}
	if len(sections) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(sections, "\n\n")
}

// toolResultMaxChars is the maximum characters of a tool result in a serialized
// summary.
const toolResultMaxChars = 2000

// truncateForSummary keeps the beginning of text and appends a truncation
// marker.
func truncateForSummary(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}
	truncatedChars := len(text) - maxChars
	return text[:maxChars] + "\n\n[... " + strconv.Itoa(truncatedChars) + " more characters truncated]"
}

// SerializeConversation serializes LLM messages to text for summarization so the
// model treats them as content to summarize rather than a conversation to
// continue. Tool results are truncated.
func SerializeConversation(messages []model.Message) string {
	var parts []string

	for _, message := range messages {
		switch typed := message.(type) {
		case model.UserMessage:
			if content := model.ContentText(typed.Content, ""); content != "" {
				parts = append(parts, "[User]: "+content)
			}
		case *model.UserMessage:
			if typed != nil {
				if content := model.ContentText(typed.Content, ""); content != "" {
					parts = append(parts, "[User]: "+content)
				}
			}
		case model.AssistantMessage:
			parts = append(parts, serializeAssistant(typed.Content)...)
		case *model.AssistantMessage:
			if typed != nil {
				parts = append(parts, serializeAssistant(typed.Content)...)
			}
		case model.ToolResultMessage:
			if content := model.ContentText(typed.Content, ""); content != "" {
				parts = append(parts, "[Tool result]: "+truncateForSummary(content, toolResultMaxChars))
			}
		case *model.ToolResultMessage:
			if typed != nil {
				if content := model.ContentText(typed.Content, ""); content != "" {
					parts = append(parts, "[Tool result]: "+truncateForSummary(content, toolResultMaxChars))
				}
			}
		}
	}

	return strings.Join(parts, "\n\n")
}

func serializeAssistant(content model.ContentList) []string {
	var thinkingParts, toolCalls []string
	hasText := false

	for _, block := range content {
		switch typed := block.(type) {
		case model.ThinkingContent:
			thinkingParts = append(thinkingParts, typed.Thinking)
		case model.ToolCall:
			toolCalls = append(toolCalls, typed.Name+"("+serializeArguments(typed.Arguments)+")")
		case model.TextContent:
			hasText = true
		}
	}

	var parts []string
	if len(thinkingParts) > 0 {
		parts = append(parts, "[Assistant thinking]: "+strings.Join(thinkingParts, "\n"))
	}
	if hasText {
		parts = append(parts, "[Assistant]: "+model.ContentText(content))
	}
	if len(toolCalls) > 0 {
		parts = append(parts, "[Assistant tool calls]: "+strings.Join(toolCalls, "; "))
	}
	return parts
}

func serializeArguments(arguments model.JsonObject) string {
	keys := make([]string, 0, len(arguments))
	for key := range arguments {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	entries := make([]string, 0, len(keys))
	for _, key := range keys {
		entries = append(entries, key+"="+safeJSONStringify(arguments[key]))
	}
	return strings.Join(entries, ", ")
}

func safeJSONStringify(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "[unserializable]"
	}
	return string(raw)
}

func assistantMessageOf(message model.Message) (model.AssistantMessage, bool) {
	switch typed := message.(type) {
	case model.AssistantMessage:
		return typed, true
	case *model.AssistantMessage:
		if typed != nil {
			return *typed, true
		}
	}
	return model.AssistantMessage{}, false
}

// SummarizationSystemPrompt is the dedicated system prompt for summarization
// requests.
const SummarizationSystemPrompt = `You are a context summarization assistant. Your task is to read a conversation between a user and an AI assistant, then produce a structured summary following the exact format specified.

Do NOT continue the conversation. Do NOT respond to any questions in the conversation. ONLY output the structured summary.`
