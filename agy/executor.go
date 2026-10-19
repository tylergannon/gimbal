package agy

import (
	"context"

	"github.com/tylergannon/gimbal/internal/executor"
)

type savedSession struct{ ID, Model, Effort, Workdir, ConversationID string }

// SaveSession retains Antigravity's native conversation identity.
func (a *adapter) SaveSession(_ context.Context, id, path string) error {
	s, err := a.session(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return executor.Write(path, savedSession{id, s.model, s.effort, s.workdir, s.conversationID})
}

// LoadSession rebuilds local routing without replaying a native prompt.
func (a *adapter) LoadSession(_ context.Context, path string, closing bool) (string, error) {
	var saved savedSession
	if err := executor.Read(path, &saved); err != nil {
		return "", err
	}
	a.mu.Lock()
	a.sessions[saved.ID] = &session{model: saved.Model, effort: saved.Effort, workdir: saved.Workdir, conversationID: saved.ConversationID}
	a.mu.Unlock()
	return saved.ID, nil
}
func (a *adapter) DetachSession(ctx context.Context, id string) error { return a.Close(ctx, id) }
