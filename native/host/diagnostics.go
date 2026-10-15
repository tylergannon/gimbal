package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// All desktop instances share a bounded log. The file lock also keeps export
// snapshots consistent with rotation by another running app instance.
type diagnosticLog struct {
	mu       sync.Mutex
	dir      string
	maxBytes int64
	backups  int
}

var desktopDiagnostics = sync.OnceValues(func() (*diagnosticLog, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, "Library", "Logs", "Gimbal")
	if runtime.GOOS != "darwin" {
		dir = filepath.Join(home, ".local", "state", "gimbal", "logs")
	}
	return &diagnosticLog{dir: dir, maxBytes: 2 << 20, backups: 3}, nil
})

func (d *diagnosticLog) locked(fn func() error) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := os.MkdirAll(d.dir, 0o700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(d.dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Close() }()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return err
	}
	defer func() { _ = unix.Flock(int(lock.Fd()), unix.LOCK_UN) }()
	return fn()
}

func (d *diagnosticLog) record(component, message string) error {
	// Bound individual writes as well as files. No transcript or audio is
	// supplied by the native callers; never include credentials or environment.
	if len(message) > 32<<10 {
		message = message[:32<<10] + " [truncated]"
	}
	var entry strings.Builder
	for _, line := range strings.Split(strings.TrimSpace(message), "\n") {
		fmt.Fprintf(&entry, "%s pid=%d [%s] %s\n", time.Now().UTC().Format(time.RFC3339Nano), os.Getpid(), component, line)
	}
	text := entry.String()
	if int64(len(text)) > d.maxBytes {
		text = text[:d.maxBytes]
	}
	return d.locked(func() error {
		path := filepath.Join(d.dir, "desktop.log")
		if info, err := os.Stat(path); err == nil && info.Size()+int64(len(text)) > d.maxBytes {
			for i := d.backups; i > 0; i-- {
				old := path
				if i > 1 {
					old = fmt.Sprintf("%s.%d", path, i-1)
				}
				if err := os.Rename(old, fmt.Sprintf("%s.%d", path, i)); err != nil && !os.IsNotExist(err) {
					return err
				}
			}
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		_, writeErr := io.WriteString(file, text)
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	})
}

func (d *diagnosticLog) snapshot() ([]byte, error) {
	var out bytes.Buffer
	fmt.Fprintf(&out, "Gimbal desktop diagnostics\nExported: %s\nSystem: %s/%s\nNo conversation or recording files are included.\n\n", time.Now().UTC().Format(time.RFC3339), runtime.GOOS, runtime.GOARCH)
	err := d.locked(func() error {
		for i := d.backups; i >= 0; i-- {
			name := "desktop.log"
			if i > 0 {
				name = fmt.Sprintf("%s.%d", name, i)
			}
			data, err := os.ReadFile(filepath.Join(d.dir, name))
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(&out, "--- %s ---\n", name)
			out.Write(data)
		}
		return nil
	})
	return out.Bytes(), err
}

func writeDiagnostic(component, message string) {
	d, err := desktopDiagnostics()
	if err == nil {
		err = d.record(component, message)
	}
	if err != nil {
		// Logging failure must not stop capture or server execution.
		fmt.Fprintf(os.Stderr, "Gimbal diagnostic log unavailable: %v\n", err)
	}
}

type serverDiagnostics struct{}

func (serverDiagnostics) Write(p []byte) (int, error) {
	writeDiagnostic("server", string(p))
	_, _ = os.Stderr.Write(p)
	return len(p), nil
}

func exportDiagnostics(destination string) error {
	d, err := desktopDiagnostics()
	if err != nil {
		return err
	}
	return d.export(destination)
}

func (d *diagnosticLog) export(destination string) error {
	// Do not allow an export to replace the active log or its lock/rotation.
	abs, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	if filepath.Dir(abs) == d.dir {
		return fmt.Errorf("choose a destination outside the Gimbal logs folder")
	}
	data, err := d.snapshot()
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(abs), ".gimbal-diagnostics-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(file.Name()) }()
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), abs)
}
