package model

import (
	"bytes"
	"encoding/json"
	"strings"
)

// Transcript replay, ported from pi packages/ai/src/utils/transcript.ts. The
// helpers accept any message list and read only system messages.

// CreateInitialSystemMessage builds the leading system message for a prompt
// and tool set. It reports false when both are empty.
func CreateInitialSystemMessage(systemPrompt string, tools []Tool) (SystemMessage, bool) {
	hasSystemPrompt := len(systemPrompt) > 0
	hasTools := len(tools) > 0
	if !hasSystemPrompt && !hasTools {
		return SystemMessage{}, false
	}
	message := NewSystemText(systemPrompt, 0)
	if hasTools {
		message.ToolsAdded = tools
	}
	return message, true
}

// NormalizeContext folds Context.SystemPrompt and Context.Tools into a leading
// system message. It is the only producer of a TranscriptContext.
func NormalizeContext(context Context) TranscriptContext {
	initial, ok := CreateInitialSystemMessage(context.SystemPrompt, context.Tools)
	if !ok {
		return TranscriptContext{Messages: context.Messages}
	}
	messages := make([]Message, 0, len(context.Messages)+1)
	messages = append(messages, initial)
	messages = append(messages, context.Messages...)
	return TranscriptContext{Messages: messages}
}

// GetInitialSystemMessage returns the leading system message, if the
// transcript starts with one.
func GetInitialSystemMessage(messages []Message) (SystemMessage, bool) {
	if len(messages) == 0 {
		return SystemMessage{}, false
	}
	return systemMessageOf(messages[0])
}

// WithoutInitialSystemMessage drops the leading system message.
func WithoutInitialSystemMessage(messages []Message) []Message {
	if _, ok := GetInitialSystemMessage(messages); ok {
		return messages[1:]
	}
	return messages
}

func systemMessageOf(message Message) (SystemMessage, bool) {
	switch m := message.(type) {
	case SystemMessage:
		return m, true
	case *SystemMessage:
		if m != nil {
			return *m, true
		}
	}
	return SystemMessage{}, false
}

type toolMap struct {
	order  []string
	byName map[string]Tool
}

func newToolMap() *toolMap { return &toolMap{byName: map[string]Tool{}} }

func (m *toolMap) set(tool Tool) {
	if _, ok := m.byName[tool.Name]; !ok {
		m.order = append(m.order, tool.Name)
	}
	m.byName[tool.Name] = tool
}

func (m *toolMap) get(name string) (Tool, bool) {
	tool, ok := m.byName[name]
	return tool, ok
}

func (m *toolMap) delete(name string) {
	if _, ok := m.byName[name]; !ok {
		return
	}
	delete(m.byName, name)
	for i, n := range m.order {
		if n == name {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
}

func (m *toolMap) values() []Tool {
	out := make([]Tool, len(m.order))
	for i, name := range m.order {
		out[i] = m.byName[name]
	}
	return out
}

// GetCurrentTools resolves the tools available after applying every transcript
// delta in order.
func GetCurrentTools(messages []Message) []Tool {
	tools := newToolMap()
	for _, message := range messages {
		system, ok := systemMessageOf(message)
		if !ok {
			continue
		}
		for _, tool := range system.ToolsRemoved {
			tools.delete(tool.Name)
		}
		for _, tool := range system.ToolsAdded {
			tools.set(tool)
		}
	}
	return tools.values()
}

// GetCurrentSystemMessage replays every system message into one leading system
// message holding the current prompt and tools.
func GetCurrentSystemMessage(messages []Message) (SystemMessage, bool) {
	var content []string
	sections := SystemSections{}
	var timestamp int64
	found := false
	for _, message := range messages {
		system, ok := systemMessageOf(message)
		if !ok {
			continue
		}
		if !found {
			timestamp, found = system.Timestamp, true
		}
		if text := ContentText(system.Content); len(text) > 0 {
			content = append(content, text)
		}
		for _, section := range system.Sections.Entries() {
			if section.Value == nil {
				sections.Delete(section.Name)
			} else {
				sections.Set(section.Name, section.Value)
			}
		}
	}
	tools := GetCurrentTools(messages)
	if !found && len(tools) == 0 {
		return SystemMessage{}, false
	}
	current := NewSystemText(strings.Join(content, "\n\n"), timestamp)
	if sections.Len() > 0 {
		current.Sections = sections
	}
	if len(tools) > 0 {
		current.ToolsAdded = tools
	}
	return current, true
}

// GetCurrentSystemPrompt renders the current system prompt text after
// replaying every system message.
func GetCurrentSystemPrompt(messages []Message) string {
	if message, ok := GetCurrentSystemMessage(messages); ok {
		return GetSystemMessageText(message)
	}
	return ""
}

// CollapseSystemMessages rebuilds the transcript for APIs without
// mid-conversation system messages: the replayed system message leads, and
// every later system message is dropped.
func CollapseSystemMessages(context TranscriptContext) TranscriptContext {
	head, hasHead := GetCurrentSystemMessage(context.Messages)
	messages := make([]Message, 0, len(context.Messages)+1)
	if hasHead {
		messages = append(messages, head)
	}
	for _, message := range context.Messages {
		if message.MessageRole() != RoleSystem {
			messages = append(messages, message)
		}
	}
	return TranscriptContext{Messages: messages}
}

// ResolveTranscript keeps later system messages in place when the model accepts
// them, and collapses them otherwise.
func ResolveTranscript(context TranscriptContext, supportsMidConvoSystemMessages bool) TranscriptContext {
	if supportsMidConvoSystemMessages {
		return context
	}
	return CollapseSystemMessages(context)
}

// ToToolDeclaration strips a tool down to what it declares to the model: name,
// description, parameters and constrained sampling.
func ToToolDeclaration(tool Tool) Tool {
	declaration := Tool{Name: tool.Name, Description: tool.Description}
	declaration.Parameters = tool.Parameters.Clone()
	if tool.ConstrainedSampling != nil {
		sampling := *tool.ConstrainedSampling
		declaration.ConstrainedSampling = &sampling
	}
	return declaration
}

// DeclarationsEqual reports whether two tools declare the same interface to the
// model.
func DeclarationsEqual(left, right Tool) bool {
	l, err := json.Marshal(ToToolDeclaration(left))
	if err != nil {
		return false
	}
	r, err := json.Marshal(ToToolDeclaration(right))
	if err != nil {
		return false
	}
	return bytes.Equal(l, r)
}

// ToolStateChanges is the difference between two complete tool states.
type ToolStateChanges struct {
	ToolsAdded   []Tool
	ToolsRemoved []ToolReference
}

// GetToolStateChanges compares two complete tool states. A changed definition
// is a removal followed by an addition.
func GetToolStateChanges(previous, current []Tool) ToolStateChanges {
	previousTools := newToolMap()
	for _, tool := range previous {
		previousTools.set(tool)
	}
	currentTools := newToolMap()
	for _, tool := range current {
		currentTools.set(tool)
	}
	var changes ToolStateChanges
	for _, tool := range current {
		if previousTool, ok := previousTools.get(tool.Name); !ok || !DeclarationsEqual(previousTool, tool) {
			changes.ToolsAdded = append(changes.ToolsAdded, ToToolDeclaration(tool))
		}
	}
	for _, tool := range previous {
		if currentTool, ok := currentTools.get(tool.Name); !ok || !DeclarationsEqual(tool, currentTool) {
			changes.ToolsRemoved = append(changes.ToolsRemoved, ToolReference{Name: tool.Name})
		}
	}
	return changes
}

// GetDeclaredTools returns every definition referenced by transcript tool
// state, in first-declaration order.
func GetDeclaredTools(messages []Message) []Tool {
	definitions := newToolMap()
	for _, message := range messages {
		system, ok := systemMessageOf(message)
		if !ok {
			continue
		}
		for _, tool := range system.ToolsAdded {
			definitions.set(tool)
		}
	}
	return definitions.values()
}

// HasToolRedefinitions reports whether a tool name was declared twice with
// different definitions.
func HasToolRedefinitions(messages []Message) bool {
	declared := newToolMap()
	for _, message := range messages {
		system, ok := systemMessageOf(message)
		if !ok {
			continue
		}
		for _, tool := range system.ToolsAdded {
			if previous, ok := declared.get(tool.Name); ok && !DeclarationsEqual(previous, tool) {
				return true
			}
			declared.set(tool)
		}
	}
	return false
}

// HasNonAdditiveToolChanges reports whether tool history contains a removal or
// a same-name redeclaration that an addition-only transport cannot replay.
func HasNonAdditiveToolChanges(messages []Message) bool {
	declared := map[string]bool{}
	for _, message := range messages {
		system, ok := systemMessageOf(message)
		if !ok {
			continue
		}
		if len(system.ToolsRemoved) > 0 {
			return true
		}
		for _, tool := range system.ToolsAdded {
			if declared[tool.Name] {
				return true
			}
			declared[tool.Name] = true
		}
	}
	return false
}

// TranscriptTools splits tool declarations between the top-level request field
// and in-place additions.
type TranscriptTools struct {
	// RequestTools are the tools sent in the top-level request field.
	RequestTools []Tool
	// AnchorsAdditions reports whether later system messages carry their own
	// ToolsAdded as in-place additions.
	AnchorsAdditions bool
}

// ResolveTranscriptTools splits tool declarations between the top-level request
// field and in-place additions.
func ResolveTranscriptTools(messages []Message, supportsToolAdditions bool) TranscriptTools {
	anchorsAdditions := supportsToolAdditions && !HasNonAdditiveToolChanges(messages)
	if !anchorsAdditions {
		return TranscriptTools{RequestTools: GetCurrentTools(messages)}
	}
	var requestTools []Tool
	if initial, ok := GetInitialSystemMessage(messages); ok {
		requestTools = initial.ToolsAdded
	}
	return TranscriptTools{RequestTools: requestTools, AnchorsAdditions: true}
}
