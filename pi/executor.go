package pi

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/tylergannon/gimbal/internal/executor"
	"github.com/tylergannon/gimbal/internal/pi/history"
	"github.com/tylergannon/gimbal/internal/pi/session"
)

type savedSession struct {
	ID, NativeID, Model, Workdir, Root, History string
	HasHistory                                  bool
}

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
		// A native fork starts beside its parent. Give its persisted history
		// a separately owned root before the parent can be explicitly closed.
		original := s.session.SessionFile()
		parentRoot := filepath.Dir(filepath.Dir(original))
		var err error
		root, err = os.MkdirTemp("", "gimbal-pi-")
		if err != nil {
			return err
		}
		keep := false
		defer func() {
			if !keep {
				_ = os.RemoveAll(root)
			}
		}()
		if err = os.CopyFS(filepath.Join(root, "agent"), os.DirFS(filepath.Join(parentRoot, "agent"))); err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Join(root, "sessions"), 0700); err != nil {
			return err
		}
		target := filepath.Join(root, "sessions", filepath.Base(original))
		source, err := os.Open(original)
		if err != nil {
			return err
		}
		defer func() { _ = source.Close() }()
		file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(file, source)
		if err = errors.Join(copyErr, file.Close()); err != nil {
			return err
		}
		if err = s.session.SessionManager().SetSessionFile(target); err != nil {
			return err
		}
		s.root = root
		keep = true
		_ = os.Remove(original)
	}
	return executor.Write(path, savedSession{id, s.nativeID, s.model, s.workdir, root, s.session.SessionFile(), executor.Exists(s.session.SessionFile())})
}

// LoadSession reopens Pi's Go history and agent without replaying a prompt.
func (a *Adapter) LoadSession(ctx context.Context, path string, closing bool) (string, error) {
	var saved savedSession
	if err := executor.Read(path, &saved); err != nil {
		return "", err
	}
	if closing {
		a.mu.Lock()
		a.sessions[saved.ID] = &sessionState{id: saved.ID, root: saved.Root}
		a.mu.Unlock()
		return saved.ID, nil
	}
	key := a.config.APIKey()
	selected, err := a.routerCatalogModel(ctx, modelID(saved.Model), key)
	if err != nil {
		return "", err
	}
	var manager *history.SessionManager
	if !saved.HasHistory && !executor.Exists(saved.History) {
		manager, err = history.Create(saved.Workdir, filepath.Dir(saved.History), &history.NewSessionOptions{ID: &saved.NativeID})
	} else {
		manager, err = history.Open(saved.History, filepath.Dir(saved.History), saved.Workdir)
	}
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
