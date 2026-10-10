package observation

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// refresh follows only complete durable transactions; it never writes the
// running process's observation. The same Store retains subscription cursors.
func (s *Store) refresh() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.run.Status == StatusRunning {
		// A failed socket cannot establish death. Only acquiring the writer's
		// lifetime lock permits settlement, while holding it against other readers.
		lock, err := os.OpenFile(filepath.Join(s.dir, "runner.lock"), os.O_RDWR, 0)
		if err == nil {
			if unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB) == nil {
				err = RecordTerminated(s.dir, "run process exited before completing; session cleanup is unconfirmed")
			}
			_ = lock.Close()
			if err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	f, err := os.Open(filepath.Join(s.dir, deltaFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if _, err = f.Seek(s.journalEnd, io.SeekStart); err != nil {
		return err
	}
	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		var delta Delta
		if err := json.Unmarshal(line, &delta); err != nil {
			return err
		}
		if delta.Stream != s.stream || delta.Position != s.position+1 {
			return fmt.Errorf("observation: non-contiguous durable delta at %d", delta.Position)
		}
		s.applyDeltaLocked(delta)
		s.journalEnd += int64(len(line))
		s.retainLocked(delta)
		s.publishDeltaLocked(delta)
	}
	if s.run.Status != StatusRunning {
		s.closed = true
		s.finishJoinSubscribersLocked()
	}
	return nil
}
