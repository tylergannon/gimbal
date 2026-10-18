package claude

import (
	"context"
	"github.com/tylergannon/gimbal/internal/executor"
)

type savedSession struct {
	ID, Model, Effort, Workdir, Parent string
	Fresh                              bool
}

// SaveSession persists native continuation information for the agent executor.
func (a *adapter) SaveSession(_ context.Context, id, path string) error {
	s, err := a.session(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return executor.Write(path, savedSession{id, s.model, s.effort, s.workdir, s.parent, s.fresh})
}

// LoadSession rebuilds adapter routing without starting or replaying a turn.
func (a *adapter) LoadSession(_ context.Context, path string) (string, error) {
	var saved savedSession
	if err := executor.Read(path, &saved); err != nil {
		return "", err
	}
	a.mu.Lock()
	a.sessions[saved.ID] = &session{model: saved.Model, effort: saved.Effort, workdir: saved.Workdir, parent: saved.Parent, fresh: saved.Fresh}
	a.mu.Unlock()
	return saved.ID, nil
}
func (a *adapter) DetachSession(ctx context.Context, id string) error { return a.Close(ctx, id) }
