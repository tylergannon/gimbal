package host

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tylergannon/polytype"
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
	}, Sidebar: polytype.Nullable[bool]{Present: true, Value: false}, Seen: []SeenReply{
		{Path: chat.ID, Request: "reply-1"}, {Path: root + "/conversations/closed", Request: "reply-2"}, {Path: "/projects/missing/conversations/foreign", Request: "reply-3"},
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
	want := WindowState{Project: second.ID(), Path: chat.ID, Tabs: []WindowTab{chat}, Sidebar: polytype.Nullable[bool]{Present: true, Value: false}, Seen: []SeenReply{{Path: chat.ID, Request: "reply-1"}, {Path: root + "/conversations/closed", Request: "reply-2"}}}
	if got := restored.Window(); !reflect.DeepEqual(got, want) {
		t.Fatalf("restored window = %+v, want %+v", got, want)
	}
}

func TestSeenRepliesSurviveSwitchingProjects(t *testing.T) {
	owner := New(t.Context(), t.TempDir())
	defer owner.Close()
	a, err := owner.AdmitProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b, err := owner.AdmitProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	aPath, bPath := "/projects/"+a.ID()+"/conversations/a", "/projects/"+b.ID()+"/conversations/b"
	for _, state := range []WindowState{
		{Project: a.ID(), Seen: []SeenReply{{Path: aPath, Request: "a-reply"}}},
		{Project: b.ID(), Seen: []SeenReply{{Path: bPath, Request: "b-reply"}}},
		{Project: a.ID(), Seen: []SeenReply{{Path: aPath, Request: "a-new-reply"}}},
	} {
		if err := owner.RememberWindow(state); err != nil {
			t.Fatal(err)
		}
	}
	got := owner.Window()
	seen := map[string]string{}
	for _, item := range got.Seen {
		seen[item.Path] = item.Request
	}
	if got.Project != a.ID() || seen[aPath] != "a-new-reply" || seen[bPath] != "b-reply" {
		t.Fatalf("window lost read state after switching: %+v", got)
	}
}
