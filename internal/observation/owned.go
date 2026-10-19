package observation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/tylergannon/gimbal/internal/runstate"
	"github.com/tylergannon/gimbal/runstore"
)

// RecordTerminatedOwned commits through the caller's exclusive run writer.
// Observation never locates or acquires an authoritative store on its own.
func RecordTerminatedOwned(ctx context.Context, r runstore.Run, dir, reason string) error {
	summary, e := runstate.Inspect(ctx, r)
	if e != nil {
		return e
	}
	if summary.Ended {
		return nil
	}
	raw, e := json.Marshal(struct {
		Time time.Time `json:"time"
 "os"
 "path/filepath"`
		Scope string            `json:"scope"`
		Event map[string]string `json:"event"`
	}{time.Now().UTC(), "", map[string]string{"kind": "run_ended", "name": summary.Header.Workflow, "error": reason}})
	if e != nil {
		return e
	}
	if e = runstate.Append(ctx, r, summary.Header.Root, summary.Header.Root, "lifecycle", json.RawMessage(raw)); e != nil {
		return e
	}
	return recordStopped(dir, reason, false, raw)
}
func RecordCleanupOwned(ctx context.Context, r runstore.Run, dir string, result error) error {
	h, e := r.Header(ctx)
	if e != nil {
		return e
	}
	state := runstate.CleanupState{Status: "released"}
	if result != nil {
		state.Status = "pending"
		state.Error = result.Error()
	}
	changed, e := runstate.UpdateCleanup(ctx, r, h.Root, state)
	if e != nil || !changed {
		return e
	}
	s, e := open(nil, string(h.RunID), dir, true)
	if e != nil {
		return e
	}
	// This is the existing projection, updated only after the authoritative fact.
	s.noWrite = false
	return s.CleanupStatus(state.Status, state.Error)
}

// ReleasedProjection is the cheap positive gate for the periodic cleanup
// owner. Missing or malformed projection data never establishes release.
func ReleasedProjection(dir string) bool {
	raw, e := os.ReadFile(filepath.Join(dir, "run.json"))
	if e != nil {
		return false
	}
	var rows []RunRow
	return json.Unmarshal(raw, &rows) == nil && len(rows) == 1 && rows[0].Status != StatusRunning && rows[0].Status != "" && rows[0].CleanupStatus == "released"
}
