package session

import (
	"context"

	"github.com/tylergannon/gimbal/internal/pi/agent"
	"github.com/tylergannon/gimbal/internal/pi/history"
	"github.com/tylergannon/gimbal/internal/pi/model"
)

// Fork creates an independent session with a deep snapshot of the current
// conversation. The parent session keeps running; the child has its own agent,
// queues, cancellation state, history and storage. An unfinished trailing
// assistant tool batch is excluded so the child starts from a complete prefix.
func (s *Session) Fork(ctx context.Context) (*Session, error) {
	projection := s.sessionManager.BuildSessionProjection()
	messages := completePrefix(projection.Messages)

	var manager *history.SessionManager
	var err error
	if s.sessionManager.IsPersisted() && s.sessionManager.GetSessionFile() != "" {
		sessionDir := s.sessionManager.GetSessionDir()
		manager, err = history.ForkFrom(s.sessionManager.GetSessionFile(), s.cwd, sessionDir, nil)
		if err != nil {
			return nil, err
		}
	} else {
		entries := model.CloneSessionEntries(s.sessionManager.GetBranch())
		header := s.sessionManager.GetHeader()
		var adopted *model.SessionHeader
		if header != nil {
			copyHeader := *header
			copyHeader.ID = ""
			adopted = &copyHeader
		}
		manager, err = history.InMemory(s.cwd, nil, adopted, entries)
		if err != nil {
			return nil, err
		}
	}

	childMessages := make([]model.AgentMessage, 0, len(messages))
	for _, message := range messages {
		childMessages = append(childMessages, model.CloneMessage(message))
	}

	template := s.template
	childAgent := agent.NewAgent(agent.AgentOptions{
		InitialState: &model.AgentState{
			Model:         s.Model(),
			ThinkingLevel: s.ThinkingLevel(),
			Messages:      childMessages,
		},
		StreamFn: template.StreamFn,
	})
	childAgent.SetSessionID(manager.GetSessionID())

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

// completePrefix drops a trailing assistant message whose tool calls have no
// recorded results.
func completePrefix(messages []model.AgentMessage) []model.AgentMessage {
	if len(messages) == 0 {
		return messages
	}
	last := messages[len(messages)-1]
	assistant, ok := assistantOf(last)
	if !ok || !assistantHasToolCalls(assistant) {
		return messages
	}
	return messages[:len(messages)-1]
}

func assistantHasToolCalls(message *model.AssistantMessage) bool {
	for _, block := range message.Content {
		if _, ok := block.(model.ToolCall); ok {
			return true
		}
	}
	return false
}
