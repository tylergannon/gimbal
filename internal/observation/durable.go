package observation

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"

	"github.com/tylergannon/gimble/internal/sessionstate"
)

const (
	snapshotFile  = "observation.json"
	deltaFile     = "observation-deltas.jsonl"
	snapshotEvery = 64
)

type durableSnapshot struct {
	Snapshot RunSnapshot         `json:"snapshot"`
	Calls    map[string]openCall `json:"open_calls"`
	Offset   int64               `json:"delta_offset"`
}

// appendDeltaLocked makes the reduced transaction durable before it becomes
// visible to subscribers. Sync is deliberate: Position means this exact
// transaction can be recovered after a process failure.
func (s *Store) appendDeltaLocked(delta Delta) error {
	if s.dir == "" || s.noWrite {
		return nil
	}
	f, err := os.OpenFile(filepath.Join(s.dir, deltaFile), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(delta)
	if err == nil {
		_, err = f.Write(append(raw, '\n'))
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("observation: append durable delta: %w", err)
	}
	s.journalEnd += int64(len(raw) + 1)
	return nil
}

func (s *Store) saveSnapshotLocked() error {
	if s.dir == "" || s.noWrite {
		return nil
	}
	state := durableSnapshot{Snapshot: s.snapshotLocked(), Calls: s.calls, Offset: s.journalEnd}
	if err := writeAtomic(filepath.Join(s.dir, snapshotFile), state); err != nil {
		return fmt.Errorf("observation: save reduced snapshot: %w", err)
	}
	s.sinceSave = 0
	return nil
}

func loadDurable(registry *Registry, id, dir string) (*Store, bool, error) {
	raw, err := os.ReadFile(filepath.Join(dir, snapshotFile))
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var saved durableSnapshot
	if err := json.Unmarshal(raw, &saved); err != nil {
		return nil, false, fmt.Errorf("observation: read reduced snapshot: %w", err)
	}
	s := newStore(registry, id, saved.Snapshot.Run.Name, dir)
	s.noWrite = true
	s.restoreLocked(saved.Snapshot)
	s.calls = saved.Calls
	s.journalEnd = saved.Offset
	f, err := os.Open(filepath.Join(dir, deltaFile))
	if os.IsNotExist(err) {
		s.noWrite = false
		s.closed = true
		return s, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Seek(saved.Offset, io.SeekStart); err != nil {
		return nil, false, err
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64<<10), maxLogLine)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		var delta Delta
		if err := json.Unmarshal(line, &delta); err != nil {
			break
		} // interrupted tail was never durable
		if delta.Stream != s.stream || delta.Position != s.position+1 {
			return nil, false, fmt.Errorf("observation: non-contiguous durable delta at %d", delta.Position)
		}
		s.applyDeltaLocked(delta)
		s.journalEnd += int64(len(line) + 1)
	}
	if err := scanner.Err(); err != nil {
		return nil, false, err
	}
	s.noWrite = false
	s.closed = true
	return s, true, nil
}

func (s *Store) restoreLocked(snapshot RunSnapshot) {
	s.stream, s.position, s.run = snapshot.Stream, snapshot.Position, snapshot.Run
	if s.stream == "" {
		s.stream = newStreamID()
	}
	for k, v := range snapshot.Scopes {
		row := cloneScope(v)
		s.scopes[k] = &row
	}
	for k, v := range snapshot.Sessions {
		row := v
		s.sessions[k] = &row
	}
	for k, v := range snapshot.Turns {
		row := v
		s.turns[k] = &row
	}
	for k, v := range snapshot.TurnUsage {
		copied := make(map[string]Usage, len(v))
		maps.Copy(copied, v)
		s.turnUsage[k] = copied
	}
	for k, v := range snapshot.ModelCalls {
		s.modelCalls[k] = append([]ModelCallRow(nil), v...)
	}
	for turn, value := range snapshot.Transcripts {
		s.transcripts[turn] = &transcript{projection: sessionstate.New(value.Snapshot.State), provenance: cloneProvenance(value.Provenance)}
	}
}

func (s *Store) applyDeltaLocked(delta Delta) {
	for _, frame := range delta.Frames {
		switch frame.Name {
		case FrameRow:
			var row rowFrame
			if json.Unmarshal(frame.Data, &row) != nil {
				continue
			}
			s.restoreRowLocked(row)
		case FrameEvent:
			var event eventFrame
			if json.Unmarshal(frame.Data, &event) != nil {
				continue
			}
			decoded, err := sessionstate.DecodeValue(event.Event)
			if err != nil {
				continue
			}
			obj, ok := decoded.(*sessionstate.Obj)
			if !ok {
				continue
			}
			script := s.transcriptLocked(event.Turn)
			script.projection.Apply(obj)
			ref, _ := decodeRef(event.NativeRef)
			s.foldProvenanceLocked(script, ref, event.NativeRef)
		}
	}
	s.position = delta.Position
}

func (s *Store) restoreRowLocked(frame rowFrame) {
	raw, _ := json.Marshal(frame.Row)
	switch frame.Table {
	case tableRun:
		var v RunRow
		if json.Unmarshal(raw, &v) == nil {
			s.run = v
		}
	case tableScopes:
		var v ScopeRow
		if json.Unmarshal(raw, &v) == nil {
			s.scopes[frame.Key] = &v
		}
	case tableSessions:
		var v SessionRow
		if json.Unmarshal(raw, &v) == nil {
			s.sessions[frame.Key] = &v
		}
	case tableTurns:
		var v TurnRow
		if json.Unmarshal(raw, &v) == nil {
			s.turns[frame.Key] = &v
		}
	case tableTurnUsage:
		var v map[string]Usage
		if json.Unmarshal(raw, &v) == nil {
			s.turnUsage[frame.Key] = v
		}
	case tableModelCalls:
		var v []ModelCallRow
		if json.Unmarshal(raw, &v) == nil {
			s.modelCalls[frame.Key] = v
		}
	}
}
