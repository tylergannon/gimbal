package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tylergannon/gimbal/internal/agentexecutor"
	"github.com/tylergannon/gimbal/internal/executor"
)

// The CLI owns the retained native session. Each headless run owns only its
// current turn, so scope cleanup releases the local run without closing the
// native conversation. Explicit --close uses the executor's real Close.
type promptSessionAdapter struct {
	*agentexecutor.Adapter
	id string
}

func (a *promptSessionAdapter) CreateSession(context.Context, string, string, string) (string, error) {
	return a.id, nil
}

func (a *promptSessionAdapter) Close(context.Context, string) error { return nil }

func promptSessionRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gimbal", "run-prompt"), nil
}

func promptSessionDir(root, id string) string { return filepath.Join(root, "sessions", id) }

func readPromptSession(root, id string) (agentexecutor.Session, error) {
	var session agentexecutor.Session
	if err := executor.Validate(id, "session_"); err != nil {
		return session, fmt.Errorf("invalid session ID: %w", err)
	}
	if err := executor.Read(filepath.Join(promptSessionDir(root, id), "session.json"), &session); err != nil {
		if os.IsNotExist(err) {
			return session, fmt.Errorf("session %s not found; use the ID printed by run-prompt", id)
		}
		return session, fmt.Errorf("read session %s: %w", id, err)
	}
	if string(session.ID) != id {
		return session, fmt.Errorf("session %s has incompatible retained state", id)
	}
	return session, nil
}

func preparePromptSession(options runPromptOptions, workdirExplicit bool, caller promptCaller) (string, agentexecutor.Session, error) {
	root, err := promptSessionRoot()
	if err != nil {
		return "", agentexecutor.Session{}, err
	}
	if options.resume == "" {
		selection, err := resolvePromptModel(options, caller)
		if err != nil {
			return "", agentexecutor.Session{}, err
		}
		workdir, err := promptWorkdir(options.workdir)
		return root, agentexecutor.Session{ID: executor.NewSessionID(), Harness: selection.Harness, Model: selection.Model, Effort: selection.Effort, Workdir: workdir}, err
	}
	session, err := readPromptSession(root, options.resume)
	if err != nil {
		return "", session, err
	}
	dir := promptSessionDir(root, options.resume)
	if executor.Exists(filepath.Join(dir, "closed.json")) {
		return "", session, fmt.Errorf("session %s is closed; omit --resume to start a new session", options.resume)
	}
	lease, err := executor.Lock(dir)
	if err != nil {
		return "", session, err
	}
	if lease == nil {
		return "", session, fmt.Errorf("session %s already has an active turn; wait for it to finish before resuming", options.resume)
	}
	lease.Close() // The executor rechecks ownership atomically when accepting the turn.
	if !executor.Exists(filepath.Join(dir, "native.json")) {
		return "", session, fmt.Errorf("session %s has no retained native conversation; omit --resume to start a new session", options.resume)
	}
	if workdirExplicit {
		workdir, err := promptWorkdir(options.workdir)
		if err != nil {
			return "", session, err
		}
		if workdir != session.Workdir {
			return "", session, fmt.Errorf("--workdir conflicts with session workspace %s; omit it when resuming", session.Workdir)
		}
	}
	if _, err := promptWorkdir(session.Workdir); err != nil {
		return "", session, err
	}
	if options.modelVersion != "" && options.model == "" {
		return "", session, errors.New("--model-version requires --model when resuming; omit both to keep the saved model")
	}
	if options.model != "" {
		// An omitted effort retains the saved value even when the model alias
		// normally defaults to a different effort.
		selectionOptions := options
		if selectionOptions.effort == "" {
			selectionOptions.effort = session.Effort
		}
		selection, err := resolvePromptModel(selectionOptions, promptCallerNone)
		if err != nil {
			return "", session, err
		}
		if selection.Model != session.Model || selection.Harness != session.Harness {
			return "", session, fmt.Errorf("--model conflicts with session model %s (%s); omit it when resuming", session.Model, session.Harness)
		}
	}
	if options.effort != "" && options.effort != session.Effort {
		return "", session, fmt.Errorf("--effort conflicts with session effort %q; omit it when resuming", session.Effort)
	}
	return root, session, nil
}

func closePromptSession(id string, stderr io.Writer) error {
	root, err := promptSessionRoot()
	if err != nil {
		return err
	}
	if _, err = readPromptSession(root, id); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err = (&agentexecutor.Adapter{Root: root}).Close(ctx, id); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stderr, "Closed session %s\n", id)
	return err
}
