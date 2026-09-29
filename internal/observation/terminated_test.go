package observation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrchestratorSettlesRunWithoutInventingCleanup(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "runs", "lost")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	log := `{"seq":1,"time":"2026-09-28T00:00:00Z","scope":"","event":{"kind":"run_started","name":"lost"}}
{"seq":2,"time":"2026-09-28T00:00:01Z","scope":"","event":{"kind":"scope_began","name":""}}
`
	if err := os.WriteFile(filepath.Join(dir, "run.jsonl"), []byte(log), 0644); err != nil {
		t.Fatal(err)
	}
	if err := RecordTerminated(dir, "Orchestrator: worker lost; cleanup is unconfirmed"); err != nil {
		t.Fatal(err)
	}
	if err := RecordTerminated(dir, "must not replace first outcome"); err != nil {
		t.Fatal(err)
	}
	snapshot, err := NewRegistry(filepath.Dir(filepath.Dir(dir))).Snapshot("lost")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Run.Status != StatusFailed || snapshot.Scopes[""].Ended != 0 || !strings.Contains(snapshot.Run.Error, "unconfirmed") {
		t.Fatalf("%+v", snapshot)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "run.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 3 {
		t.Fatal(string(raw))
	}
	var last record
	if err = json.Unmarshal([]byte(lines[2]), &last); err != nil {
		t.Fatal(err)
	}
	if last.Seq != 3 || last.Event.Kind != "run_ended" {
		t.Fatal(last)
	}
}
