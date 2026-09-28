package compiledscope

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// Snapshot names a complete immutable manifest within this run's store.
// Neither it nor its entries contain producer-local paths or scope provenance.
type Snapshot string
type Entry struct {
	Key    string          `json:"key"`
	Value  json.RawMessage `json:"value,omitempty"`
	File   string          `json:"file,omitempty"`
	Format string          `json:"format,omitempty"`
}
type Store struct{ Root string }

func objectID(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }
func validID(id string) bool {
	b, err := hex.DecodeString(id)
	return err == nil && len(b) == sha256.Size
}

// publish links a fully written object into its immutable name. Interrupted
// publication may leave unreachable blobs, never a partly readable manifest.
func (s Store) publish(kind string, data []byte) (string, error) {
	id := objectID(data)
	dir := filepath.Join(s.Root, kind)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, ".publish-")
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close(); _ = os.Remove(f.Name()) }()
	if _, err = f.Write(data); err != nil {
		return "", err
	}
	if err = f.Chmod(0444); err != nil {
		return "", err
	}
	if err = f.Sync(); err != nil {
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	if err = os.Link(f.Name(), filepath.Join(dir, id)); err != nil && !os.IsExist(err) {
		return "", err
	}
	if _, err = s.read(kind, id); err != nil {
		return "", err
	}
	return id, nil
}
func (s Store) read(kind, id string) ([]byte, error) {
	if !validID(id) {
		return nil, fmt.Errorf("invalid context %s reference %q", kind, id)
	}
	b, err := os.ReadFile(filepath.Join(s.Root, kind, id))
	if err != nil {
		return nil, fmt.Errorf("read context %s %s: %w", kind, id, err)
	}
	if objectID(b) != id {
		return nil, fmt.Errorf("damaged context %s %s", kind, id)
	}
	return b, nil
}
func (s Store) Load(ref Snapshot) ([]Entry, error) {
	if ref == "" {
		return nil, nil
	} // only the initial, empty context
	b, err := s.read("snapshots", string(ref))
	if err != nil {
		return nil, err
	}
	var entries []Entry
	if err = json.Unmarshal(b, &entries); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.Key] {
			return nil, fmt.Errorf("duplicate context key %q", e.Key)
		}
		seen[e.Key] = true
		if _, err = s.Value(e); err != nil {
			return nil, err
		}
	}
	return entries, nil
}
func (s Store) Value(e Entry) (json.RawMessage, error) {
	if e.File == "" {
		if !json.Valid(e.Value) {
			return nil, fmt.Errorf("invalid context value %q", e.Key)
		}
		return e.Value, nil
	}
	b, err := s.read("blobs", e.File)
	if err != nil {
		return nil, err
	}
	switch e.Format {
	case "text":
		return json.Marshal(string(b))
	case "json":
		if !json.Valid(b) {
			return nil, fmt.Errorf("invalid JSON context file %q", e.Key)
		}
		return b, nil
	default:
		return nil, fmt.Errorf("invalid context format %q", e.Format)
	}
}

// Extend resolves shadowing now. Large values are published at the producing
// activity boundary; only this result reference crosses orchestration history.
// The inline threshold is a storage choice, never an accepted-size limit.
func (s Store) Extend(base Snapshot, writes ...Entry) (Snapshot, error) {
	entries, err := s.Load(base)
	if err != nil {
		return "", err
	}
	for _, e := range writes {
		raw, err := s.Value(e)
		if err != nil {
			return "", err
		}
		if e.File == "" && len(raw) > 4096 {
			data := []byte(raw)
			e.Format = "json"
			var scalar string
			if json.Unmarshal(raw, &scalar) == nil {
				data = []byte(scalar)
				e.Format = "text"
			}
			e.File, err = s.publish("blobs", data)
			if err != nil {
				return "", err
			}
			e.Value = nil
		}
		entries = slices.DeleteFunc(entries, func(old Entry) bool { return old.Key == e.Key })
		entries = append(entries, e)
	}
	b, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	id, err := s.publish("snapshots", b)
	return Snapshot(id), err
}

// Materialize makes a consumer-local copy, separate from immutable backing
// objects. Each dispatch repairs changed copies before advertising their paths.
func (s Store) Materialize(e Entry) (string, error) {
	b, err := s.read("blobs", e.File)
	if err != nil {
		return "", err
	}
	return s.MaterializeText(e.File, b)
}
func (s Store) MaterializeText(id string, b []byte) (string, error) {
	if !validID(id) || objectID(b) != id {
		return "", fmt.Errorf("invalid context materialization")
	}
	dir := filepath.Join(s.Root, "materialized")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	name := filepath.Join(dir, id)
	if existing, err := os.ReadFile(name); err == nil && bytes.Equal(existing, b) {
		return name, nil
	}
	f, err := os.CreateTemp(dir, ".materialize-")
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close(); _ = os.Remove(f.Name()) }()
	if _, err = f.Write(b); err != nil {
		return "", err
	}
	if err = f.Chmod(0444); err != nil {
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	if err = os.Rename(f.Name(), name); err != nil {
		return "", err
	}
	return name, nil
}
func (s Store) Text(text string) (string, string, error) {
	id, err := s.publish("blobs", []byte(text))
	if err != nil {
		return "", "", err
	}
	path, err := s.MaterializeText(id, []byte(text))
	return id, path, err
}
