package environments

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/backend"
)

func testService(t *testing.T) *Service {
	t.Helper()
	s, e := New(context.Background(), Options{InstanceDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
func TestConfigurationPreservesOpaqueEntriesAndRejectsStaleWrites(t *testing.T) {
	s := testService(t)
	ctx := context.Background()
	raw := `{"version":1,"future":{"x":2},"machines":[{"id":"queue","name":"Queue","kind":"route","route":{"Name":"not-installed","Configuration":{"opaque":[1,2]}},"future_flag":true},{"id":"bad","kind":"address"}],"environments":[]}`
	d, e := s.SaveConfig(ctx, ConfigEdit{Text: raw})
	if e != nil {
		t.Fatal(e)
	}
	snap, e := s.Snapshot(ctx, "")
	if e != nil {
		t.Fatal(e)
	}
	if len(snap.Machines) != 2 || len(snap.Issues) == 0 {
		t.Fatalf("valid sibling not isolated: %+v", snap)
	}
	route := backend.Route{Name: "not-installed", Configuration: json.RawMessage(`{"opaque":[1,2]}`)}
	_, e = s.SaveMachine(ctx, MachineEdit{Revision: d.Revision, Machine: Machine{ID: "queue", Name: "Renamed", Kind: KindRoute, Route: &route}, ConnectAnyway: true})
	if e != nil {
		t.Fatal(e)
	}
	after, _ := s.Config(ctx)
	var saved map[string]json.RawMessage
	if e = json.Unmarshal([]byte(after.Text), &saved); e != nil {
		t.Fatal(e)
	}
	if string(saved["future"]) == "" || !strings.Contains(after.Text, "future_flag") || !strings.Contains(after.Text, `"bad"`) {
		t.Fatalf("unrelated fields/invalid siblings were lost: %s", after.Text)
	}
	if _, e = s.SaveConfig(ctx, ConfigEdit{Revision: d.Revision, Text: raw}); e == nil {
		t.Fatal("stale file revision accepted")
	}
	b, e := os.ReadFile(s.path)
	if e != nil || string(b) != after.Text {
		t.Fatal("conflict changed document", e)
	}
}
func TestMalformedHandEditKeepsLastGoodAndCannotBeOverwrittenByForm(t *testing.T) {
	s := testService(t)
	ctx := context.Background()
	d, e := s.SaveConfig(ctx, ConfigEdit{Text: `{"version":1,"machines":[{"id":"box","name":"Box","kind":"address","address":"http://box:8081"}],"environments":[]}`})
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(s.path, []byte("{\n broken"), 0600); e != nil {
		t.Fatal(e)
	}
	snap, e := s.Snapshot(ctx, "")
	if e != nil || len(snap.Machines) != 2 || len(snap.Issues) == 0 {
		t.Fatalf("last-good state lost: %+v %v", snap, e)
	}
	if _, e = s.SaveMachine(ctx, MachineEdit{Revision: d.Revision, Machine: Machine{ID: "box", Name: "B", Kind: KindAddress, Address: "http://box:8081"}, ConnectAnyway: true}); e == nil {
		t.Fatal("form overwrote malformed file")
	}
	doc, _ := s.Config(ctx)
	if doc.Valid || doc.Text != "{\n broken" || doc.Issues[0].Line != 2 {
		t.Fatalf("malformed state: %+v", doc)
	}
}
func TestSnapshotSessionCallbackCanReenterServiceAndLiveRemovalIsNamed(t *testing.T) {
	s := testService(t)
	s.options.Sessions = func() []SessionUsage {
		_ = s.RegisterProject("p", t.TempDir())
		return []SessionUsage{{Machine: "box", Conversation: "chat123", Title: "Active fix", Active: true}}
	}
	doc, e := s.SaveConfig(context.Background(), ConfigEdit{Text: `{"version":1,"machines":[{"id":"box","name":"Box","kind":"address","address":"http://box:8081"}],"environments":[]}`})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Snapshot(context.Background(), ""); e != nil {
		t.Fatal(e)
	}
	if e = s.RemoveMachine(context.Background(), MachineRemoval{Revision: doc.Revision, ID: "box"}); e == nil || !strings.Contains(e.Error(), "chat123") {
		t.Fatalf("live-use guard missing: %v", e)
	}
}
func TestInstanceConfigIsolation(t *testing.T) {
	s := testService(t)
	if filepath.Dir(s.path) != s.options.InstanceDir {
		t.Fatal("config escaped instance")
	}
	other := testService(t)
	if other.path == s.path {
		t.Fatal("instances share config")
	}
}
