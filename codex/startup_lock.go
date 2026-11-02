package codex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var errStartupLockBusy = errors.New("codex: daemon startup lock busy")

// The lock file persists: unlinking it could split waiters across inodes.
func startupLockPath(socket string) (string, error) {
	path, err := filepath.Abs(socket)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filepath.Base(path)+".gimbal-start.lock"), nil
}

func lockStartup(ctx context.Context, socket string) (func(), error) {
	path, err := startupLockPath(socket)
	if err != nil {
		return nil, fmt.Errorf("codex: startup lock path: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("codex: open startup lock: %w", err)
	}
	for {
		if err := ctx.Err(); err != nil {
			_ = f.Close()
			return nil, err
		}
		err = tryStartupLock(f)
		if err == nil {
			return func() { unlockStartup(f); _ = f.Close() }, nil
		}
		if !errors.Is(err, errStartupLockBusy) {
			_ = f.Close()
			return nil, fmt.Errorf("codex: lock startup: %w", err)
		}
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func startDaemonLocked(ctx context.Context, socket string) (string, error) {
	unlock, err := lockStartup(ctx, socket)
	if err != nil {
		return "", err
	}
	defer unlock()
	selectedLock, err := startupLockPath(socket)
	if err != nil {
		return "", err
	}
	current, running, err := daemonStatus(ctx)
	if err != nil {
		return "", err
	}
	currentLock, err := startupLockPath(current)
	if err != nil {
		return "", err
	}
	if currentLock != selectedLock {
		return "", fmt.Errorf("codex: daemon endpoint changed while waiting for startup lock")
	}
	if running {
		return current, nil
	}
	raiseFileDescriptorLimit()
	if _, err := runCodex(ctx, "app-server", "daemon", "start"); err != nil {
		return "", fmt.Errorf("codex: start app-server daemon: %w", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		current, running, err = daemonStatus(ctx)
		if err != nil {
			return "", err
		}
		currentLock, err = startupLockPath(current)
		if err != nil {
			return "", err
		}
		if currentLock != selectedLock {
			return "", fmt.Errorf("codex: daemon endpoint changed during startup")
		}
		if running {
			return current, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("codex: app-server daemon did not come up within 10s")
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
