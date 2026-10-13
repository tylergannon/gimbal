package host

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestProjectsKeepTheirOrderAcrossRestarts(t *testing.T) {
	instance, base := t.TempDir(), t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	owner := New(ctx, instance)
	var ids []string
	for _, name := range []string{"c", "a", "b"} {
		dir := filepath.Join(base, name)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		p, err := owner.AdmitProject(dir)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, p.ID())
	}
	order := func(o *Owner) []string {
		var got []string
		for _, p := range o.Projects() {
			got = append(got, p.ID())
		}
		return got
	}
	if got := order(owner); !slices.Equal(got, ids) {
		t.Fatalf("projects = %v, want admission order %v", got, ids)
	}
	if err := owner.ReorderProjects(ids[:2]); err == nil {
		t.Fatal("accepted an order that leaves a project out")
	}
	if err := owner.ReorderProjects([]string{ids[0], ids[0], ids[1]}); err == nil {
		t.Fatal("accepted an order that repeats a project")
	}
	want := []string{ids[2], ids[0], ids[1]}
	if err := owner.ReorderProjects(want); err != nil {
		t.Fatal(err)
	}
	if got := owner.Window().Project; got != want[0] {
		t.Fatalf("unremembered window opens %s, want the first project in order %s", got, want[0])
	}
	cancel()
	owner.Close()

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	restored := New(ctx, instance)
	defer restored.Close()
	if err := restored.RestoreProjects(); err != nil {
		t.Fatal(err)
	}
	if got := order(restored); !slices.Equal(got, want) {
		t.Fatalf("restored projects = %v, want %v", got, want)
	}
}
