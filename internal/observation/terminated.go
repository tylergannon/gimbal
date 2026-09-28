package observation

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// RecordTerminated settles only a run whose writer has been stopped externally.
// The caller MUST first establish that the old process can no longer write.
// This records the orchestrator's failure, never synthetic session/scope closes.
func RecordTerminated(dir, reason string) error {
	id := filepath.Base(dir)
	s, err := open(nil, id, dir)
	if err != nil {
		return err
	}
	if s.run.Status != StatusRunning {
		return nil
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
	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if _, err = f.Write(append(raw, '\n')); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	s.closed = false
	if err = s.Lifecycle(raw); err != nil {
		return err
	}
	return s.Close()
}
