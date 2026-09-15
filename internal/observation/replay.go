package observation

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// maxLogLine bounds one log line. A turn_ended record carries the turn's
// whole result, so the scanner's default 64 KiB is not enough.
const maxLogLine = 8 << 20

// open reads one run directory into a fresh store. A finished run's facts are
// its six tables, so the normal path loads them and reads the session logs
// only for the transcript projections. A directory missing any table is
// rebuilt from its logs instead, and the tables are written as it goes.
//
// An unfinished log is served as far as it goes, with the run's status as the
// last record left it. Nothing is followed and no agent process is resumed.
func open(registry *Registry, id, dir string) (*Store, error) {
	if s, ok, err := loadDurable(registry, id, dir); ok || err != nil {
		return s, err
	}
	s := newStore(registry, id, "", dir)
	// Nothing on this path writes a table until it is asked to: loading
	// writes nothing at all, and a rebuild writes all six at the end.
	s.noWrite = true

	rebuild := missingTable(dir) != ""
	if rebuild {
		if err := replayLines(filepath.Join(dir, "run.jsonl"), func(line json.RawMessage) error {
			return s.Lifecycle(line)
		}); err != nil {
			if os.IsNotExist(err) {
				return nil, ErrNoRun
			}
			return nil, err
		}
	} else if err := s.loadTables(dir); err != nil {
		return nil, err
	}

	// Session ids carry their scope path, so a log nests as
	// sessions/lap.1/task.2/coder.1.jsonl. A session that never wrote one has
	// no file, which is not an error.
	//
	// These events are read for the transcripts. A fact they carry is already
	// in the tables: a turn that ended does not take usage from a step, and a
	// model call the file holds is rewritten rather than added.
	for _, session := range s.sessionsInCreatedOrder() {
		path := filepath.Join(dir, "sessions", filepath.FromSlash(session)+".jsonl")
		err := replayLines(path, func(line json.RawMessage) error {
			var rec struct {
				Scope     string          `json:"scope"`
				Session   string          `json:"session"`
				Turn      string          `json:"turn"`
				Event     json.RawMessage `json:"event"`
				NativeRef json.RawMessage `json:"native_ref"`
			}
			if err := json.Unmarshal(line, &rec); err != nil {
				return err
			}
			at := Placement{Scope: rec.Scope, Session: rec.Session, Turn: rec.Turn}
			return s.Event(at, rec.Event, rec.NativeRef)
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.noWrite = false
	if rebuild {
		if err := s.writeAllTablesLocked(); err != nil {
			return nil, err
		}
	}
	s.closed = true
	if err := s.saveSnapshotLocked(); err != nil {
		return nil, err
	}
	return s, nil
}

// loadTables fills the maps from the six files. This is what reading a
// finished run means: the tables are the facts and the roll-ups are computed
// from them, so the run log is only the transcript store and the source a
// rebuild reads.
func (s *Store) loadTables(dir string) error {
	run, err := loadTable[RunRow](dir, tableRun)
	if err != nil {
		return err
	}
	scopes, err := loadTable[ScopeRow](dir, tableScopes)
	if err != nil {
		return err
	}
	sessions, err := loadTable[SessionRow](dir, tableSessions)
	if err != nil {
		return err
	}
	turns, err := loadTable[TurnRow](dir, tableTurns)
	if err != nil {
		return err
	}
	usage, err := loadTable[TurnUsageRow](dir, tableTurnUsage)
	if err != nil {
		return err
	}
	calls, err := loadTable[ModelCallRow](dir, tableModelCalls)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.fromTables = true
	if len(run) > 0 {
		s.run = run[0]
	}
	for _, row := range scopes {
		if row.Values == nil {
			row.Values = map[string]json.RawMessage{}
		}
		s.scopes[row.Key] = &row
	}
	for _, row := range sessions {
		s.sessions[row.ID] = &row
	}
	for _, row := range turns {
		s.turns[row.ID] = &row
	}
	// The two tables that are per-turn collections fold back into the maps
	// the store holds them in.
	for _, row := range usage {
		byModel := s.turnUsage[row.Turn]
		if byModel == nil {
			byModel = map[string]Usage{}
			s.turnUsage[row.Turn] = byModel
		}
		byModel[row.Model] = row.Usage
	}
	for _, row := range calls {
		s.modelCalls[row.Turn] = append(s.modelCalls[row.Turn], row)
	}
	return nil
}

// loadTable decodes one table's file into its rows.
func loadTable[Row any](dir, table string) ([]Row, error) {
	raw, err := os.ReadFile(filepath.Join(dir, table+".json"))
	if err != nil {
		return nil, err
	}
	var rows []Row
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("observation: read %s.json: %w", table, err)
	}
	return rows, nil
}

// sessionsInCreatedOrder is every session the run log named, oldest first.
func (s *Store) sessionsInCreatedOrder() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, _ := s.tableRowsLocked(tableSessions).([]SessionRow)
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.ID)
	}
	return out
}

// replayLines hands each non-empty line of one log to fold. A line that does
// not decode is an error naming the file and the line.
func replayLines(path string, fold func(json.RawMessage) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64<<10), maxLogLine)
	for line := 1; scanner.Scan(); line++ {
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		if err := fold(slices.Clone(raw)); err != nil {
			return fmt.Errorf("observation: %s line %d: %w", path, line, err)
		}
	}
	return scanner.Err()
}
