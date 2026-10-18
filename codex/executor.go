package codex

import (
	"context"
	"github.com/tylergannon/gimbal/internal/executor"
)

type savedSession struct{ ID, Model, Effort, Workdir string }

// SaveSession persists native continuation information for the agent executor.
func (a *adapter) SaveSession(_ context.Context, id, path string) error {
	s, err := a.session(id)
	if err != nil {
		return err
	}
	return executor.Write(path, savedSession{id, s.model, s.effort, s.workdir})
}

// LoadSession rebuilds adapter routing without starting or replaying a turn.
func (a *adapter) LoadSession(ctx context.Context, path string) (string, error) {
	var saved savedSession
	if err := executor.Read(path, &saved); err != nil {
		return "", err
	}
	err := a.ResumeSession(ctx, saved.ID, saved.Model, saved.Effort, saved.Workdir)
	return saved.ID, err
}
