package session

import (
	"context"
	"slices"

	"github.com/tylergannon/gimbal/internal/pi/agent"
	"github.com/tylergannon/gimbal/internal/pi/history"
	"github.com/tylergannon/gimbal/internal/pi/model"
)

// Fork creates an independent session with a deep snapshot of the current
// conversation. The parent session keeps running; the child has its own agent,
// queues, cancellation state, history and storage. A trailing assistant tool
// batch whose results are incomplete is excluded so the child starts from a
// complete prefix, and the child inherits the parent agent's provider stream,
// request options and hooks.
func (s *Session) Fork(ctx context.Context) (*Session, error) {
	projection := s.sessionManager.BuildSessionProjection()
	prefix := completePrefix(projection.Entries)
	prefixMessages := projectedMessages(prefix)

	manager, err := s.forkManager()
	if err != nil {
		return nil, err
	}
	if len(prefix) < len(projection.Entries) {
		if len(prefix) == 0 {
			manager.ResetLeaf()
		} else if err := manager.Branch(prefix[len(prefix)-1].SourceEntry.Base().ID); err != nil {
			return nil, err
		}
	}

	options := s.agent.Options()
	options.InitialState = &model.AgentState{
		Model:         s.Model(),
		ThinkingLevel: s.ThinkingLevel(),
		Messages:      prefixMessages,
	}
	childAgent := agent.NewAgent(options)
	childAgent.SetSessionID(manager.GetSessionID())

	template := s.template
	child, err := New(Config{
		Agent:                  childAgent,
		SessionManager:         manager,
		Settings:               template.Settings,
		Cwd:                    s.cwd,
		ResourceLoader:         template.ResourceLoader,
		StreamFn:               template.StreamFn,
		APIKey:                 template.APIKey,
		Headers:                template.Headers,
		Env:                    template.Env,
		ScopedModels:           append([]ScopedModel(nil), s.ScopedModels()...),
		InitialActiveToolNames: s.GetActiveToolNames(),
		AllowedToolNames:       nil,
		ExcludedToolNames:      nil,
		BaseTools:              template.BaseTools,
		CustomTools:            template.CustomTools,
	})
	if err != nil {
		return nil, err
	}
	return child, nil
}

// forkManager builds the child history manager. A persisted parent yields a
// new persisted session file; an in-memory parent yields an in-memory manager.
// Both adopt the parent's active branch and never touch the parent's manager.
func (s *Session) forkManager() (*history.SessionManager, error) {
	if s.sessionManager.IsPersisted() && s.sessionManager.GetSessionFile() != "" {
		return history.ForkFrom(s.sessionManager.GetSessionFile(), s.cwd, s.sessionManager.GetSessionDir(), nil)
	}
	entries := model.CloneSessionEntries(s.sessionManager.GetBranch())
	header := s.sessionManager.GetHeader()
	var adopted *model.SessionHeader
	if header != nil {
		copyHeader := *header
		copyHeader.ID = ""
		adopted = &copyHeader
	}
	return history.InMemory(s.cwd, nil, adopted, entries)
}

// completePrefix drops a trailing assistant message whose tool calls do not all
// have results. Results that already arrived are dropped with it: the assistant
// tool-call message is not a usable prefix until every call is resolved.
func completePrefix(entries []model.ProjectedSessionEntry) []model.ProjectedSessionEntry {
	for i, entrie := range slices.Backward(entries) {
		messages := entrie.Messages
		for _, message := range slices.Backward(messages) {
			assistant, ok := assistantOf(message)
			if !ok || !assistantHasToolCalls(assistant) {
				continue
			}
			if toolCallsResolved(entries[i+1:], assistant) {
				return entries
			}
			return entries[:i]
		}
	}
	return entries
}

func projectedMessages(entries []model.ProjectedSessionEntry) []model.AgentMessage {
	var messages []model.AgentMessage
	for _, entry := range entries {
		for _, message := range entry.Messages {
			messages = append(messages, model.CloneMessage(message))
		}
	}
	return messages
}

func toolCallsResolved(later []model.ProjectedSessionEntry, assistant *model.AssistantMessage) bool {
	remaining := map[string]bool{}
	for _, block := range assistant.Content {
		if id, ok := toolCallID(block); ok {
			remaining[id] = true
		}
	}
	if len(remaining) == 0 {
		return true
	}
	for _, entry := range later {
		for _, message := range entry.Messages {
			if id, ok := toolResultID(message); ok {
				delete(remaining, id)
			}
		}
	}
	return len(remaining) == 0
}

func toolCallID(block model.Content) (string, bool) {
	switch call := block.(type) {
	case model.ToolCall:
		return call.ID, true
	case *model.ToolCall:
		if call != nil {
			return call.ID, true
		}
	}
	return "", false
}

func toolResultID(message model.AgentMessage) (string, bool) {
	switch result := message.(type) {
	case model.ToolResultMessage:
		return result.ToolCallID, true
	case *model.ToolResultMessage:
		if result != nil {
			return result.ToolCallID, true
		}
	}
	return "", false
}

func assistantHasToolCalls(message *model.AssistantMessage) bool {
	for _, block := range message.Content {
		if _, ok := toolCallID(block); ok {
			return true
		}
	}
	return false
}
