package compiledscope

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotIsolationTransportAndPublication(t *testing.T) {
	store := Store{Root: t.TempDir()}
	entry := func(k, v string) Entry { b, _ := json.Marshal(v); return Entry{Key: k, Value: b} }
	root, err := store.Extend(t.Context(), "", entry("layer", "parent"), entry("large", strings.Repeat("payload ", 200000)))
	if err != nil {
		t.Fatal(err)
	}
	left, err := store.Extend(t.Context(), root, entry("layer", "left"), entry("child-only", "left private"))
	if err != nil {
		t.Fatal(err)
	}
	right, err := store.Extend(t.Context(), root, entry("layer", "right"))
	if err != nil {
		t.Fatal(err)
	}
	for ref, want := range map[Snapshot]string{root: "parent", left: "left", right: "right"} {
		entries, err := store.Load(t.Context(), ref)
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
	entries, err := store.Load(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	large := entries[1]
	if large.File == "" || large.Value != nil {
		t.Fatal("large value not stored by reference")
	}
	manifest, err := os.ReadFile(filepath.Join(store.Root, "objects", string(root)))
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
	got, err := moved.Load(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	path, err := moved.Materialize(t.Context(), got[1])
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
	if _, err = moved.Materialize(t.Context(), got[1]); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(path)
	if len(b) != 1600000 {
		t.Fatal("copy not restored")
	}
	blob := filepath.Join(moved.Root, "objects", large.File)
	if err := os.Chmod(blob, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blob, []byte("corrupt"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = moved.Load(t.Context(), root); err == nil {
		t.Fatal("corrupt content accepted")
	}
	if err := os.Remove(blob); err != nil {
		t.Fatal(err)
	}
	if _, err = moved.Load(t.Context(), root); err == nil {
		t.Fatal("missing content accepted")
	}
	blocked := Store{Root: t.TempDir()}
	if err := os.WriteFile(filepath.Join(blocked.Root, "objects"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if ref, err := blocked.Extend(t.Context(), "", entry("large", strings.Repeat("x", 2000000))); err == nil || ref != "" {
		t.Fatal("failed publication exposed a snapshot")
	}
}

// A second physical store proves neither resolution nor corruption checks rely
// on a shared filesystem layout. Materialized files are disposable local copies.
type memoryObjects struct{ data map[string][]byte }

func (m *memoryObjects) Put(_ context.Context, id string, data []byte) error {
	if _, exists := m.data[id]; !exists {
		m.data[id] = append([]byte(nil), data...)
	}
	return nil
}
func (m *memoryObjects) Get(_ context.Context, id string) ([]byte, error) {
	data, ok := m.data[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), data...), nil
}
func TestObjectsStoreAndSeparateMaterialization(t *testing.T) {
	objects := &memoryObjects{data: map[string][]byte{}}
	producer := Store{Objects: objects, LocalDir: t.TempDir()}
	text := strings.Repeat("captured ", 2000)
	entry, err := Encode("value", text)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := producer.Extend(t.Context(), "", entry)
	if err != nil {
		t.Fatal(err)
	}
	consumer := Store{Objects: objects, LocalDir: t.TempDir()}
	entries, err := consumer.Load(t.Context(), ref)
	if err != nil {
		t.Fatal(err)
	}
	file, err := consumer.Materialize(t.Context(), entries[0])
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(file) != consumer.LocalDir {
		t.Fatal("wrong local root")
	}
	if err = os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if _, err = consumer.Materialize(t.Context(), entries[0]); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(file)
	if err != nil || string(got) != text {
		t.Fatalf("materialization: %v", err)
	}
	objects.data[entries[0].File] = []byte("damaged")
	if _, err = consumer.Load(t.Context(), ref); err == nil {
		t.Fatal("corrupt object accepted")
	}
}
