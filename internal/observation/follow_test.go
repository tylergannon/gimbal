package observation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestExternalReaderAdvancesWithoutWriting(t *testing.T) {
	state := t.TempDir()
	dir := filepath.Join(state, "runs", "r")
	writer, err := Open(nil, "r", "example", dir, func() error { return os.MkdirAll(dir, 0755) })
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = writer.Close() }()
	lock, err := os.OpenFile(filepath.Join(dir, "runner.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Close() }()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	reader := NewRegistry(state)
	first, err := reader.Snapshot("r")
	if err != nil {
		t.Fatal(err)
	}
	store, ok := reader.Live("r")
	if !ok {
		t.Fatal("external live run missing")
	}
	_, _, sub, err := store.Join(first.Stream, first.Position)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	if err := writer.Lifecycle([]byte(`{"seq":1,"time":"2026-10-10T10:00:00Z","scope":"","event":{"kind":"run_ended","name":"example"}}`)); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, deltaFile))
	final, err := reader.Snapshot("r")
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(dir, deltaFile))
	if string(before) != string(after) {
		t.Fatal("reader changed writer's journal")
	}
	if final.Position <= first.Position || final.Run.Status != StatusCompleted {
		t.Fatalf("did not advance: %+v", final.Run)
	}
	select {
	case d := <-sub.Deltas():
		if d.Position != first.Position+1 {
			t.Fatal("cursor skipped")
		}
	default:
		t.Fatal("no streamed delta")
	}
}

func TestExternalReaderSettlesOnlyReleasedWriter(t *testing.T) {
	for _, validJSON := range []bool{false, true} {
		t.Run(fmt.Sprint(validJSON), func(t *testing.T) {
			state := t.TempDir()
			dir := filepath.Join(state, "runs", "lost")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			log := []byte("{\"seq\":1,\"time\":\"2026-10-10T10:00:00Z\",\"scope\":\"\",\"event\":{\"kind\":\"run_started\",\"name\":\"lost\"}}\n")
			if err := os.WriteFile(filepath.Join(dir, "run.jsonl"), log, 0600); err != nil {
				t.Fatal(err)
			}
			lock, err := os.OpenFile(filepath.Join(dir, "runner.lock"), os.O_CREATE|os.O_RDWR, 0600)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = lock.Close() }()
			if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
				t.Fatal(err)
			}
			writer, err := Open(nil, "lost", "lost", dir)
			if err != nil {
				t.Fatal(err)
			}
			_ = writer.Close()
			reader := NewRegistry(state)
			first, err := reader.Snapshot("lost")
			if err != nil || first.Run.Status != StatusRunning {
				t.Fatalf("live writer: %+v, %v", first.Run, err)
			}
			// There is deliberately no controller socket. Its absence is not death.
			if err := lock.Close(); err != nil {
				t.Fatal(err)
			}
			// The stopped writer may have left an incomplete durable transaction.
			tail := []byte(`{"stream":`)
			if validJSON {
				tail, _ = json.Marshal(Delta{Stream: first.Stream, Position: first.Position + 1})
			}
			if err := os.WriteFile(filepath.Join(dir, deltaFile), tail, 0600); err != nil {
				t.Fatal(err)
			}
			final, err := reader.Snapshot("lost")
			if err != nil || final.Run.Status != StatusFailed || final.Run.Error != "run process exited before completing; session cleanup is unconfirmed" {
				t.Fatalf("dead writer: %+v, %v", final.Run, err)
			}
		})
	}
}
