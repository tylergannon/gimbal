package compiledscope

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotIsolationTransportAndPublication(t *testing.T) {
	store := Store{Root: t.TempDir()}
	entry := func(k, v string) Entry { b, _ := json.Marshal(v); return Entry{Key: k, Value: b} }
	root, err := store.Extend("", entry("layer", "parent"), entry("large", strings.Repeat("payload ", 200000)))
	if err != nil {
		t.Fatal(err)
	}
	left, err := store.Extend(root, entry("layer", "left"), entry("child-only", "left private"))
	if err != nil {
		t.Fatal(err)
	}
	right, err := store.Extend(root, entry("layer", "right"))
	if err != nil {
		t.Fatal(err)
	}
	for ref, want := range map[Snapshot]string{root: "parent", left: "left", right: "right"} {
		entries, err := store.Load(ref)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.Key == "layer" && string(e.Value) != `"`+want+`"` {
				t.Fatalf("leaked: %+v", entries)
			}
		}
		if ref != left && len(entries) != 2 {
			t.Fatal("child leaked")
		}
	}
	entries, err := store.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	large := entries[1]
	if large.File == "" || large.Value != nil {
		t.Fatal("large value not stored by reference")
	}
	manifest, err := os.ReadFile(filepath.Join(store.Root, "snapshots", string(root)))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest) > 1000 || strings.Contains(string(manifest), store.Root) {
		t.Fatal("manifest contains data or local paths")
	}
	// Copy only backing objects. A fresh resolver needs no ancestors or producer.
	moved := Store{Root: t.TempDir()}
	if err := os.CopyFS(moved.Root, os.DirFS(store.Root)); err != nil {
		t.Fatal(err)
	}
	got, err := moved.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	path, err := moved.Materialize(got[1])
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil || len(b) != 1600000 {
		t.Fatalf("materialized %d: %v", len(b), err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("agent changed copy"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = moved.Materialize(got[1]); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(path)
	if len(b) != 1600000 {
		t.Fatal("copy not restored")
	}
	blob := filepath.Join(moved.Root, "blobs", large.File)
	if err := os.Chmod(blob, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blob, []byte("corrupt"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = moved.Load(root); err == nil {
		t.Fatal("corrupt content accepted")
	}
	if err := os.Remove(blob); err != nil {
		t.Fatal(err)
	}
	if _, err = moved.Load(root); err == nil {
		t.Fatal("missing content accepted")
	}
	blocked := Store{Root: t.TempDir()}
	if err := os.WriteFile(filepath.Join(blocked.Root, "snapshots"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if ref, err := blocked.Extend("", entry("large", strings.Repeat("x", 2000000))); err == nil || ref != "" {
		t.Fatal("failed publication exposed a snapshot")
	}
}
