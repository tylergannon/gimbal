package host

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWindowReopensOnlyWhereTheInstanceStillOwnsTheProject(t *testing.T) {
	instance, base := t.TempDir(), t.TempDir()
	a, b := filepath.Join(base, "a"), filepath.Join(base, "b")
	for _, dir := range []string{a, b} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	owner := New(ctx, instance)
	if got := owner.Window(); got.Project != "" || got.Tabs == nil {
		t.Fatalf("window with no projects = %+v", got)
	}
	first, err := owner.AdmitProject(a)
	if err != nil {
		t.Fatal(err)
	}
	second, err := owner.AdmitProject(b)
	if err != nil {
		t.Fatal(err)
	}
	if got := owner.Window(); got.Project != first.ID() || got.Path != "/projects/"+first.ID()+"/recordings" {
		t.Fatalf("unremembered window = %+v, want first project by path", got)
	}
	if err := owner.RememberWindow(WindowState{Project: "missing", Path: "/projects/missing/recordings"}); err == nil {
		t.Fatal("remembered a project the instance does not own")
	}
	root := "/projects/" + second.ID()
	chat := WindowTab{ID: root + "/conversations/c1", Title: "Plan", Kind: "chat"}
	if err := owner.RememberWindow(WindowState{Project: second.ID(), Path: chat.ID, Tabs: []WindowTab{
		chat, chat, {ID: "/projects/" + first.ID() + "/files/f", Title: "other", Kind: "file"},
	}}); err != nil {
		t.Fatal(err)
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
	want := WindowState{Project: second.ID(), Path: chat.ID, Tabs: []WindowTab{chat}}
	if got := restored.Window(); !reflect.DeepEqual(got, want) {
		t.Fatalf("restored window = %+v, want %+v", got, want)
	}
}
