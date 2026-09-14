package runlog

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadDrainsAvailableHistory(t *testing.T) {
	dir := t.TempDir()
	const count = 1000
	data := strings.Repeat("{\"event\":{\"kind\":\"run_started\"}}\n", count) + "{\"event\":{\"kind\":\"complete\"}}\n"
	if err := os.WriteFile(filepath.Join(dir, "run.jsonl"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	// A completed file must drain without the tail-following poll delay on
	// every record (which would take more than ten seconds for this input).
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	seen := 0
	err := Read[map[string]any](ctx, dir, func(map[string]any) error {
		seen++
		return nil
	})
	if err != nil || seen != count+1 {
		t.Fatalf("drain: records=%d, error=%v", seen, err)
	}
}

func TestReadWaitsForRunLogInExistingDirectory(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	seen := make(chan map[string]any, 2)
	done := make(chan error, 1)
	go func() {
		done <- Read[map[string]any](ctx, dir, func(record map[string]any) error {
			seen <- record
			return nil
		})
	}()

	select {
	case err := <-done:
		t.Fatalf("Read returned before run.jsonl existed: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	const data = "{\"event\":{\"kind\":\"run_started\"}}\n{\"event\":{\"kind\":\"complete\"}}\n"
	if err := os.WriteFile(filepath.Join(dir, "run.jsonl"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := len(seen); got != 2 {
		t.Fatalf("records = %d, want 2", got)
	}
}

func TestReadMissingRunDirectoryFailsAtOnce(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	err := Read[map[string]any](t.Context(), dir, func(map[string]any) error { return nil })
	if !os.IsNotExist(err) {
		t.Fatalf("Read error = %v, want not-exist error", err)
	}
}

func TestReadWaitingForRunLogStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	dir := t.TempDir()
	done := make(chan error, 1)
	go func() {
		done <- Read[map[string]any](ctx, dir, func(map[string]any) error { return nil })
	}()
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Read error = %v, want context cancellation", err)
	}
}
