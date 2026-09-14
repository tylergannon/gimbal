package observation

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// copyRun copies a saved run directory into a temporary one. The saved logs
// under testdata are never written to: a rebuild writes its tables beside the
// logs it read, so every test reads its own copy.
func copyRun(t *testing.T, from string) string {
	t.Helper()
	to := t.TempDir()
	if err := filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(to, rel), raw, 0o644)
	}); err != nil {
		t.Fatal(err)
	}
	return to
}

// wantRootTotals is the saved run's roll-up across every scope, summed by
// hand from the six turn_ended records in its log.
var wantRootTotals = map[string]Usage{
	"claude-haiku-4-5-20251001": {
		Tokens:     Tokens{Input: 20, CacheRead: 66341, CacheWrite: 8269, Output: 130, Reasoning: 220},
		StatedCost: 0.0249421,
	},
	"gpt-5.6-luna":         {Tokens: Tokens{Input: 15210, CacheRead: 30208, Output: 112}},
	"gemini-3.8-flash-low": {Tokens: Tokens{Input: 30006, Output: 101}},
}

// checkSavedRun is what the saved run holds, however it was read.
func checkSavedRun(t *testing.T, snapshot RunSnapshot) {
	t.Helper()
	if len(snapshot.Scopes) != 4 {
		t.Errorf("scopes = %d, want the root and three", len(snapshot.Scopes))
	}
	if len(snapshot.Sessions) != 3 {
		t.Errorf("sessions = %d, want 3", len(snapshot.Sessions))
	}
	if len(snapshot.Turns) != 6 {
		t.Errorf("turns = %d, want 6", len(snapshot.Turns))
	}
	calls := 0
	for _, perTurn := range snapshot.ModelCalls {
		calls += len(perTurn)
	}
	if calls != 6 {
		t.Errorf("model calls = %d, want 6", calls)
	}
	for id := range snapshot.Turns {
		if _, ok := snapshot.Transcripts[id]; !ok {
			t.Errorf("turn %s has no transcript", id)
		}
	}
	root := snapshot.Totals.Scopes[""].ByModel
	if len(root) != len(wantRootTotals) {
		t.Fatalf("the root's per-model totals = %+v, want %+v", root, wantRootTotals)
	}
	for model, want := range wantRootTotals {
		if got := root[model]; got != want {
			t.Errorf("the root's total for %s = %+v, want %+v", model, got, want)
		}
	}
}

// TestRunWithoutTablesIsRebuiltFromItsLogs is the rebuild path: a directory
// that has only logs is read record by record and its six tables are written
// beside them.
func TestRunWithoutTablesIsRebuiltFromItsLogs(t *testing.T) {
	dir := copyRun(t, filepath.Join("testdata", "issue-149"))
	store, err := open(nil, "issue-149", dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	checkSavedRun(t, store.Snapshot())
	if missing := missingTable(dir); missing != "" {
		t.Fatalf("the rebuild did not write %s.json", missing)
	}
	var rows []TurnUsageRow
	raw, err := os.ReadFile(filepath.Join(dir, "turn_usage.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 6 {
		t.Fatalf("turn_usage.json holds %d rows, want one per turn", len(rows))
	}
}

// TestFinishedRunIsReadFromItsDurableSnapshot verifies restart prefers the
// position-exact reduced state over independently altered legacy tables.
func TestFinishedRunIsReadFromItsDurableSnapshot(t *testing.T) {
	dir := copyRun(t, filepath.Join("testdata", "issue-149"))
	if _, err := open(nil, "issue-149", dir); err != nil {
		t.Fatalf("first open: %v", err)
	}

	path := filepath.Join(dir, "turn_usage.json")
	var rows []TurnUsageRow
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	rows[0].Input = 999999
	altered, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, altered, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// The other fact the session logs also carry: emptied in the file, it
	// stays empty, because reading the logs again adds none of it back.
	calls := filepath.Join(dir, "model_calls.json")
	if err := os.WriteFile(calls, []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := open(nil, "issue-149", dir)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	snapshot := store.Snapshot()
	if got := snapshot.TurnUsage[rows[0].Turn][rows[0].Model].Input; got == 999999 {
		t.Fatalf("restart trusted an altered table instead of its durable snapshot")
	}
	if got := snapshot.Totals.Scopes[""].ByModel[rows[0].Model].Input; got >= 999999 {
		t.Fatalf("restart totals trusted the altered table: %v", got)
	}
	// The load path writes nothing.
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) || after.Size() != before.Size() {
		t.Fatalf("turn_usage.json was rewritten: %v %d, was %v %d",
			after.ModTime(), after.Size(), before.ModTime(), before.Size())
	}
	// The snapshot already contains every transcript and accounting row; the
	// altered legacy files are not replay inputs.
	for id := range snapshot.Turns {
		if _, ok := snapshot.Transcripts[id]; !ok {
			t.Errorf("turn %s has no transcript", id)
		}
	}
	held := 0
	for _, perTurn := range snapshot.ModelCalls {
		held += len(perTurn)
	}
	if held == 0 {
		t.Fatal("durable snapshot lost its model calls")
	}
	raw, err = os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "[]" {
		t.Fatalf("model_calls.json = %s, want the empty array it was left as", raw)
	}

	// The turns table is likewise no longer a recovery cursor.
	turns := filepath.Join(dir, "turns.json")
	if err := os.WriteFile(turns, []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err = open(nil, "issue-149", dir)
	if err != nil {
		t.Fatalf("third open: %v", err)
	}
	snapshot = store.Snapshot()
	if len(snapshot.Turns) == 0 {
		t.Fatal("durable snapshot lost its turns")
	}
	if len(snapshot.Transcripts) == 0 {
		t.Fatal("the session logs were not read for their transcripts")
	}
	raw, err = os.ReadFile(turns)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "[]" {
		t.Fatalf("turns.json = %s, want the empty array it was left as", raw)
	}
}

// TestOneMissingTableRebuildsThemAll is what a half-written directory does: a
// table is not repaired in isolation, the run is read from its log again.
func TestOneMissingTableRebuildsThemAll(t *testing.T) {
	dir := copyRun(t, filepath.Join("testdata", "issue-149"))
	if _, err := open(nil, "issue-149", dir); err != nil {
		t.Fatalf("first open: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "turn_usage.json")); err != nil {
		t.Fatal(err)
	}
	store, err := open(nil, "issue-149", dir)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	// The durable snapshot makes a missing legacy table irrelevant to restart.
	checkSavedRun(t, store.Snapshot())
}

// TestRegistryReadsEachRunOnce is how the page is served: a run this process
// ran is answered from its own store, and a directory the process never saw
// is read once and kept.
func TestRegistryReadsEachRunOnce(t *testing.T) {
	project := t.TempDir()
	registry := NewRegistry(project)

	dir := filepath.Join(project, "runs", "in-process")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := Open(registry, "in-process", "fixture", dir)
	if err != nil {
		t.Fatal(err)
	}
	fold(t, store, sessionCreated("", "s1", "m"), turnStarted("", "s1", "t1"))
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Snapshot("in-process"); err != nil {
		t.Fatalf("a run that finished in this process: %v", err)
	}
	if kept := registry.runs["in-process"]; kept != store {
		t.Fatalf("the registry replaced the run's own store with %p", kept)
	}

	// A directory it never saw is read on first sight and kept, so two
	// requests read it once.
	saved := filepath.Join(project, "runs", "issue-149")
	if err := os.MkdirAll(filepath.Dir(saved), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(copyRun(t, filepath.Join("testdata", "issue-149")), saved); err != nil {
		t.Fatal(err)
	}
	first, err := registry.Snapshot("issue-149")
	if err != nil {
		t.Fatalf("a run this process never saw: %v", err)
	}
	opened := registry.runs["issue-149"]
	if _, err := registry.Snapshot("issue-149"); err != nil {
		t.Fatal(err)
	}
	if registry.runs["issue-149"] != opened {
		t.Fatal("the second request read the run again instead of using the one it kept")
	}
	checkSavedRun(t, first)
}
