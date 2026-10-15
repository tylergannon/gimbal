package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/observation"
)

func TestRunReadsUseTheirExplicitProject(t *testing.T) {
	owner := host.New(t.Context(), t.TempDir())
	defer owner.Close()
	a, err := owner.AdmitProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b, err := owner.AdmitProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(a.Dir(), "runs", "owned-run")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	store, err := observation.Open(a.Registry(), "owned-run", "review", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Lifecycle(json.RawMessage(`{"seq":1,"time":"2026-10-14T12:00:00Z","scope":"","event":{"kind":"run_started","name":"review"}}`)); err != nil {
		t.Fatal(err)
	}
	ctx := host.WithOwner(b.RequestContext(t.Context()), owner)
	// Even when the route context belongs to B, the retained panel requests A.
	got, err := viewRun(ctx, RunRequest{Project: a.ID(), ID: "owned-run"})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot observation.RunSnapshot
	if err := json.Unmarshal([]byte(got.Snapshot.JSON), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Run.ID != "owned-run" {
		t.Fatalf("read wrong run: %+v", snapshot.Run)
	}
	if _, err := viewRun(ctx, RunRequest{Project: b.ID(), ID: "owned-run"}); err == nil {
		t.Fatal("read A's run through B")
	}
	if _, err := viewRun(ctx, RunRequest{Project: "unadmitted", ID: "owned-run"}); err == nil {
		t.Fatal("read run through an unadmitted project")
	}
}
