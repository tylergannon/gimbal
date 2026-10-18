package pi

import (
	"context"
	"github.com/tylergannon/gimbal/internal/executor"
	"github.com/tylergannon/gimbal/internal/pi/history"
	"github.com/tylergannon/gimbal/internal/pi/session"
	"path/filepath"
)

type savedSession struct{ ID, NativeID, Model, Workdir, Root, History string }

// SaveSession persists native continuation information for the agent executor.
func (a *Adapter) SaveSession(_ context.Context, id, path string) error {
	s, err := a.session(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	root := s.root
	if root == "" {
		root = filepath.Dir(filepath.Dir(s.session.SessionFile()))
	}
	return executor.Write(path, savedSession{id, s.nativeID, s.model, s.workdir, root, s.session.SessionFile()})
}

// LoadSession reopens Pi's Go history and agent without replaying a prompt.
func (a *Adapter) LoadSession(ctx context.Context, path string) (string, error) {
	var saved savedSession
	if err := executor.Read(path, &saved); err != nil {
		return "", err
	}
	key := a.config.APIKey()
	selected, err := a.routerCatalogModel(ctx, modelID(saved.Model), key)
	if err != nil {
		return "", err
	}
	manager, err := history.Open(saved.History, filepath.Dir(saved.History), saved.Workdir)
	if err != nil {
		return "", err
	}
	result, err := session.Create(ctx, session.CreateOptions{Cwd: saved.Workdir, AgentDir: filepath.Join(saved.Root, "agent"), SessionManager: manager, Model: selected, StreamFn: a.streamFn(key), APIKey: key})
	if err != nil {
		return "", err
	}
	s := &sessionState{id: saved.ID, nativeID: saved.NativeID, model: saved.Model, workdir: saved.Workdir, root: saved.Root, session: result.Session}
	a.mu.Lock()
	a.sessions[saved.ID] = s
	a.mu.Unlock()
	return saved.ID, nil
}

// DetachSession releases memory while retaining the native history for later turns.
func (a *Adapter) DetachSession(_ context.Context, id string) error {
	a.mu.Lock()
	s := a.sessions[id]
	delete(a.sessions, id)
	a.mu.Unlock()
	if s != nil {
		s.session.Dispose()
	}
	return nil
}
