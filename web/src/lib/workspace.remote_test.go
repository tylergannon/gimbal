package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tylergannon/gimbal/internal/app"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/skgo/params"
)

func TestCreateProjectRejectsMissingDirectory(t *testing.T) {
	ctx := t.Context()
	owner := host.New(ctx, t.TempDir())
	defer owner.Close()
	ctx = host.WithOwner(ctx, owner)
	var event app.RequestEvent[params.Params]

	typo := filepath.Join(t.TempDir(), "gim")
	if _, err := createProject(ctx, event, NewProject{Path: typo}); err == nil {
		t.Fatal("a mistyped path was admitted")
	}
	if _, err := os.Stat(typo); !os.IsNotExist(err) {
		t.Fatalf("a mistyped path was created: %v", err)
	}
	if n := len(owner.Projects()); n != 0 {
		t.Fatalf("projects=%d after a failed admission", n)
	}

	dir := t.TempDir()
	got, err := createProject(ctx, event, NewProject{Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(owner.Projects()) != 1 || got.ID == "" {
		t.Fatalf("existing directory was not admitted: %+v", got)
	}
}
