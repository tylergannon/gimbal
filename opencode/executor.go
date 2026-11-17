package opencode

import (
	"context"
	"fmt"

	"github.com/tylergannon/gimbal/internal/executor"
)

type savedSession struct{ ID, Model, Provider, Variant, Workdir, StateDir string }

// SaveSession retains the shared server's conversation identity and routing.
func (a *adapter) SaveSession(_ context.Context, id, path string) error {
	s, err := a.session(id)
	if err != nil {
		return err
	}
	stateDir, err := resolveStateDir(a.config.stateDir)
	if err != nil {
		return err
	}
	return executor.Write(path, savedSession{id, s.model, s.provider, s.variant, s.workdir, stateDir})
}

// LoadSession attaches to the existing native conversation without creating one.
func (a *adapter) LoadSession(ctx context.Context, path string, closing bool) (string, error) {
	var saved savedSession
	if err := executor.Read(path, &saved); err != nil {
		return "", err
	}
	client, err := a.config.connect(ctx, saved.Workdir, saved.StateDir)
	if err != nil {
		return "", fmt.Errorf("opencode: reconnect session: %w", err)
	}
	a.config.stateDir = saved.StateDir
	if err := a.ensureCapture(); err != nil {
		return "", err
	}
	if !closing {
		if err := a.ensureEvents(ctx, client); err != nil {
			return "", err
		}
	}
	a.mu.Lock()
	a.sessions[saved.ID] = &adapterSession{client: client, model: saved.Model, provider: saved.Provider, variant: saved.Variant, workdir: saved.Workdir}
	a.mu.Unlock()
	return saved.ID, nil
}

func (a *adapter) DetachSession(ctx context.Context, id string) error { return a.Close(ctx, id) }
