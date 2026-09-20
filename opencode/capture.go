package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const captureDirName = "captures"

type captureEntry struct {
	Time      time.Time `json:"time"`
	Kind      string    `json:"kind"`
	RequestID string    `json:"request_id,omitempty"`
	Operation string    `json:"operation,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	Workdir   string    `json:"workdir,omitempty"`
	Raw       string    `json:"raw,omitempty"`
	Value     any       `json:"value,omitempty"`
	Error     string    `json:"error,omitempty"`
}

type rawCapture struct {
	mu   sync.Mutex
	path string
	file *os.File
	err  error
}

func (a *adapter) ensureCapture() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.capture != nil && a.capture.file != nil {
		return nil
	}
	dir, err := resolveStateDir(a.config.stateDir)
	if err != nil {
		return fmt.Errorf("opencode: resolve capture directory: %w", err)
	}
	captures := filepath.Join(dir, captureDirName)
	if err := os.MkdirAll(captures, 0o700); err != nil {
		return fmt.Errorf("opencode: create capture directory: %w", err)
	}
	if err := os.Chmod(captures, 0o700); err != nil {
		return fmt.Errorf("opencode: protect capture directory: %w", err)
	}
	id, err := captureID("capture")
	if err != nil {
		return err
	}
	path := filepath.Join(captures, id+".jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("opencode: create raw capture: %w", err)
	}
	a.capture = &rawCapture{path: path, file: file}
	return nil
}

func (a *adapter) captureRecord(entry captureEntry) {
	a.mu.Lock()
	capture := a.capture
	a.mu.Unlock()
	if capture == nil {
		return
	}
	capture.write(entry)
}

func (capture *rawCapture) write(entry captureEntry) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	if capture.file == nil || capture.err != nil {
		return
	}
	entry.Time = time.Now().UTC()
	if err := json.NewEncoder(capture.file).Encode(entry); err != nil {
		capture.err = fmt.Errorf("write OpenCode raw capture %s: %w", capture.path, err)
	}
}

func (a *adapter) closeCapture() {
	a.mu.Lock()
	capture := a.capture
	a.capture = nil
	a.mu.Unlock()
	if capture == nil {
		return
	}
	capture.mu.Lock()
	defer capture.mu.Unlock()
	if capture.file != nil {
		if err := capture.file.Close(); err != nil && capture.err == nil {
			capture.err = fmt.Errorf("close OpenCode raw capture %s: %w", capture.path, err)
		}
		capture.file = nil
	}
}
