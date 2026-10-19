package observation

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/tylergannon/gimbal/internal/runstate"
	"io"
	"os"
	"path/filepath"
	"time"
)

// RecordTerminated settles only a run whose writer has been stopped externally.
// The caller MUST first establish that the old process can no longer write.
// This records the orchestrator's failure, never synthetic session/scope closes.
func RecordTerminated(dir, reason string) error {
	if runstate.Recognized(dir) {
		return fmt.Errorf("observation: new-format terminalization requires the run-store writer")
	}
	return recordStopped(dir, reason, false)
}

// RecordCleanup retains an actual cleanup outcome after the workflow's writer
// has exited. Like RecordTerminated, the caller must hold its lifetime lock.
func RecordCleanup(dir string, err error) error {
	message := ""
	if err != nil {
		message = err.Error()
	}
	if runstate.Recognized(dir) {
		return fmt.Errorf("observation: new-format cleanup requires the run-store writer")
	}
	return recordStopped(dir, message, true)
}

func recordStopped(dir, reason string, cleanup bool, saved ...json.RawMessage) error {
	id := filepath.Base(dir)
	s, err := open(nil, id, dir)
	if err != nil {
		return err
	}
	if cleanup && s.run.Cancellation.Present && s.run.Cancellation.Value.CleanupError == reason && ((reason == "" && s.run.Cancellation.Value.Cleanup == "completed") || (reason != "" && s.run.Cancellation.Value.Cleanup == "unconfirmed")) {
		return nil
	}
	if !cleanup && s.run.Status != StatusRunning {
		return nil
	}
	// The writer is stopped. Remove an incomplete durable tail before appending
	// the terminal transaction, retaining every complete transaction we loaded.
	if err := os.Truncate(filepath.Join(dir, deltaFile), s.journalEnd); err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "run.jsonl"), os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	reader := bufio.NewReader(f)
	var offset int64
	var seq int64
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			var rec record
			if err = json.Unmarshal(line, &rec); err != nil {
				if readErr == io.EOF {
					break
				}
				return fmt.Errorf("read stopped run: %w", err)
			}
			seq = rec.Seq
			offset += int64(len(line))
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	// Discard only a torn final record. Earlier complete records are retained.
	if err = f.Truncate(offset); err != nil {
		return err
	}
	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	rec := struct {
		Seq   int64             `json:"seq"`
		Time  time.Time         `json:"time"`
		Scope string            `json:"scope"`
		Event map[string]string `json:"event"`
	}{seq + 1, time.Now().UTC(), "", map[string]string{"kind": "run_ended", "name": s.run.Name, "error": reason}}
	if cleanup {
		rec.Event = map[string]string{"kind": "cancellation_cleanup", "error": reason}
	}
	if len(saved) > 0 {
		projectionSeq := rec.Seq
		if err := json.Unmarshal(saved[0], &rec); err != nil {
			return err
		}
		rec.Seq = projectionSeq
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if _, err = f.Write(append(raw, '\n')); err != nil {
		return err
	}
	if !runstate.Recognized(dir) {
		if err = f.Sync(); err != nil {
			return err
		}
	}
	s.closed = false
	if err = s.Lifecycle(raw); err != nil {
		return err
	}
	return s.Close()
}
