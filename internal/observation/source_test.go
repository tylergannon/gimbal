package observation

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tylergannon/gimbal/backend"
)

type testSource struct {
	responses []backend.Observation
	calls     int
}

func (s *testSource) ReadObservation(context.Context, backend.Target, string) (backend.Observation, error) {
	r := s.responses[s.calls]
	s.calls++
	return r, nil
}
func TestRemoteSnapshotGapReplacesStaleRowsWithoutLocalDeath(t *testing.T) {
	first := newStore(nil, "remote", "workflow", "").snapshotLocked()
	first.Scopes["stale"] = ScopeRow{}
	next := newStore(nil, "remote", "workflow", "").snapshotLocked()
	next.Scopes["fresh"] = ScopeRow{}
	rawFirst, _ := json.Marshal(first)
	rawNext, _ := json.Marshal(next)
	source := &testSource{responses: []backend.Observation{{Snapshot: rawFirst, Cursor: "first"}, {Unavailable: true}, {Gap: true, Snapshot: rawNext, Cursor: "next"}}}
	store := OpenSource(nil, "remote", "workflow", source)
	if err := store.refresh(); err != nil {
		t.Fatal(err)
	}
	if err := store.refresh(); err == nil {
		t.Fatal("missing unavailable response")
	}
	if store.run.Status != StatusRunning {
		t.Fatal("disconnect established death")
	}
	if err := store.refresh(); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.scopes["stale"]; ok {
		t.Fatal("stale row survived replacement")
	}
	if _, ok := store.scopes["fresh"]; !ok {
		t.Fatal("new remote row missing")
	}
}

func TestRemoteSourceAppliesRetainedDelta(t *testing.T) {
	snapshot := newStore(nil, "remote", "workflow", "").snapshotLocked()
	raw, _ := json.Marshal(snapshot)
	row := snapshot.Run
	row.Status = StatusCompleted
	body, _ := json.Marshal(rowFrame{Table: "run", Key: "remote", Row: row})
	delta := Delta{Stream: snapshot.Stream, Position: snapshot.Position + 1, Frames: []Frame{{Name: FrameRow, Data: body}}}
	rawDelta, _ := json.Marshal(delta)
	source := &testSource{responses: []backend.Observation{{Snapshot: raw, Cursor: "first"}, {Deltas: []json.RawMessage{rawDelta}, Cursor: "next"}}}
	store := OpenSource(nil, "remote", "workflow", source)
	if err := store.refresh(); err != nil {
		t.Fatal(err)
	}
	if err := store.refresh(); err != nil {
		t.Fatal(err)
	}
	if store.run.Status != StatusCompleted {
		t.Fatal("committed delta not folded")
	}
}

func TestRemoteSourceNullSnapshotKeepsCurrentProjection(t *testing.T) {
	snapshot := newStore(nil, "remote", "workflow", "").snapshotLocked()
	snapshot.Scopes["kept"] = ScopeRow{}
	raw, _ := json.Marshal(snapshot)
	source := &testSource{responses: []backend.Observation{{Snapshot: raw, Cursor: "first"}, {Snapshot: json.RawMessage(`null`), Cursor: "first"}}}
	store := OpenSource(nil, "remote", "workflow", source)
	if err := store.refresh(); err != nil {
		t.Fatal(err)
	}
	if err := store.refresh(); err != nil {
		t.Fatal(err)
	}
	if store.run.ID != "remote" || store.run.Status != StatusRunning {
		t.Fatalf("no-change response erased run: %+v", store.run)
	}
	if _, ok := store.scopes["kept"]; !ok {
		t.Fatal("no-change response erased rows")
	}
}
