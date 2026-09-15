package observation

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
)

// The seven tables. A name is both the file's stem beside the run log and the
// key the snapshot holds that table under, so a `row` frame names one thing.
const (
	tableRun        = "run"
	tableScopes     = "scopes"
	tableSessions   = "sessions"
	tableTurns      = "turns"
	tableTurnUsage  = "turn_usage"
	tableModelCalls = "model_calls"
	tableCommands   = "commands"
)

// tables is every table, in the order the seven files are written.
var tables = []string{tableRun, tableScopes, tableSessions, tableTurns, tableTurnUsage, tableModelCalls, tableCommands}

// tableRowsLocked is one table as its file holds it: a JSON array in a fixed
// order, so the same state always writes the same bytes.
func (s *Store) tableRowsLocked(table string) any {
	switch table {
	case tableRun:
		return []RunRow{s.run}
	case tableScopes:
		out := make([]ScopeRow, 0, len(s.scopes))
		for _, row := range s.scopes {
			out = append(out, *row)
		}
		slices.SortFunc(out, func(a, b ScopeRow) int { return cmp.Compare(a.Key, b.Key) })
		return out
	case tableSessions:
		out := make([]SessionRow, 0, len(s.sessions))
		for _, row := range s.sessions {
			out = append(out, *row)
		}
		slices.SortFunc(out, func(a, b SessionRow) int {
			return cmp.Or(cmp.Compare(a.Created, b.Created), cmp.Compare(a.ID, b.ID))
		})
		return out
	case tableTurns:
		out := make([]TurnRow, 0, len(s.turns))
		for _, row := range s.turns {
			out = append(out, *row)
		}
		slices.SortFunc(out, func(a, b TurnRow) int {
			return cmp.Or(cmp.Compare(a.Started, b.Started), cmp.Compare(a.ID, b.ID))
		})
		return out
	case tableTurnUsage:
		out := make([]TurnUsageRow, 0, len(s.turnUsage))
		for _, turn := range sortedKeys(s.turnUsage) {
			byModel := s.turnUsage[turn]
			for _, model := range sortedKeys(byModel) {
				out = append(out, TurnUsageRow{Run: s.run.ID, Turn: turn, Model: model, Usage: byModel[model]})
			}
		}
		return out
	case tableModelCalls:
		out := make([]ModelCallRow, 0, len(s.modelCalls))
		for _, turn := range sortedKeys(s.modelCalls) {
			// Calls are appended in step order, which is the order they are
			// written in.
			out = append(out, s.modelCalls[turn]...)
		}
		return out
	case tableCommands:
		out := make([]CommandRow, 0, len(s.commands))
		for _, row := range s.commands {
			out = append(out, *row)
		}
		slices.SortFunc(out, func(a, b CommandRow) int {
			return cmp.Or(cmp.Compare(a.Started, b.Started), cmp.Compare(a.ID, b.ID))
		})
		return out
	}
	panic(fmt.Errorf("observation: no table named %s", table))
}

// sortedKeys is one map's keys in string order.
func sortedKeys[V any](in map[string]V) []string { return slices.Sorted(maps.Keys(in)) }

// writeTablesLocked rewrites each named table whole. It runs under the store
// lock, before the fold publishes, so what a reader finds on disk is a state
// the page was also told about. A run with no directory writes nothing, and
// neither does a replay until it has finished reading.
func (s *Store) writeTablesLocked(changed []string) error {
	if s.dir == "" || s.noWrite {
		return nil
	}
	for _, table := range tables {
		if !slices.Contains(changed, table) {
			continue
		}
		if err := writeAtomic(filepath.Join(s.dir, table+".json"), s.tableRowsLocked(table)); err != nil {
			return fmt.Errorf("observation: write %s.json: %w", table, err)
		}
	}
	return nil
}

// writeAllTablesLocked rewrites all seven, which is what Open does for an empty
// run and what a replay does for a directory missing any of them.
func (s *Store) writeAllTablesLocked() error { return s.writeTablesLocked(tables) }

// missingTable reports the first of the seven files a directory does not hold.
func missingTable(dir string) string {
	for _, table := range tables {
		if _, err := os.Stat(filepath.Join(dir, table+".json")); err != nil {
			return table
		}
	}
	return ""
}

// writeAtomic encodes one table and replaces its file in one rename, so a
// reader never sees a half-written array.
func writeAtomic(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	name := temp.Name()
	if _, err := temp.Write(raw); err != nil {
		_ = temp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}
