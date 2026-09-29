// Package contextdata captures scoped values as immutable content-addressed
// snapshots. Objects supplies physical storage; Store verifies identities,
// resolves lexical shadowing and materializes complete values for local agents.
// Retain objects for as long as retained runs refer to them. Content addressing
// supplies integrity, not access control or confidentiality.
package contextdata

import (
	"bytes"
	"context"
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

// Entry is one encoded scoped value. File references a complete immutable object;
// Value may carry a supplied summary when File is present.
type Entry struct {
	Key    string          `json:"key"`
	Value  json.RawMessage `json:"value,omitempty"`
	File   string          `json:"file,omitempty"`
	Format string          `json:"format,omitempty"`
}

// Objects publishes complete immutable objects atomically or returns an error.
// Existing identities must retain their original bytes. Store computes and
// verifies identities and publishes referenced content before its manifest.
type Objects interface {
	Put(context.Context, string, []byte) error
	Get(context.Context, string) ([]byte, error)
}

// Store separates immutable object access from agent-visible materialization.
// Root selects the built-in filesystem store; Objects selects another backend.
type Store struct {
	Root     string
	Objects  Objects
	LocalDir string
}

// FileObjects stores immutable objects as files directly beneath Root.
type FileObjects struct{ Root string }

func (s Store) objects() Objects {
	if s.Objects != nil {
		return s.Objects
	}
	return FileObjects{Root: filepath.Join(s.Root, "objects")}
}
func (s Store) localDir() string {
	if s.LocalDir != "" {
		return s.LocalDir
	}
	return filepath.Join(s.Root, "materialized")
}

// Encode captures a value at its authored write boundary using Go JSON methods.
func Encode(key string, value any) (Entry, error) {
	raw, err := json.Marshal(value)
	return Entry{Key: key, Value: raw}, err
}

func objectID(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }
func validID(id string) bool {
	b, err := hex.DecodeString(id)
	return err == nil && len(b) == sha256.Size
}

// Put links a complete file into its immutable name without replacing old bytes.
func (f FileObjects) Put(ctx context.Context, id string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validID(id) {
		return fmt.Errorf("invalid object identity %q", id)
	}
	if err := os.MkdirAll(f.Root, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(f.Root, ".publish-")
	if err != nil {
		return err
	}
	defer func() { _ = tmp.Close(); _ = os.Remove(tmp.Name()) }()
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Chmod(0444); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Link(tmp.Name(), filepath.Join(f.Root, id)); err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

// Get reads an immutable object by its content identity.
func (f FileObjects) Get(ctx context.Context, id string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !validID(id) {
		return nil, fmt.Errorf("invalid object identity %q", id)
	}
	return os.ReadFile(filepath.Join(f.Root, id))
}
func (s Store) publish(ctx context.Context, kind string, data []byte) (string, error) {
	id := objectID(data)
	if err := s.objects().Put(ctx, id, data); err != nil {
		return "", err
	}
	if _, err := s.read(ctx, kind, id); err != nil {
		return "", err
	}
	return id, nil
}

// ReadObject reads and verifies a content-addressed object.
func (s Store) ReadObject(ctx context.Context, id string) ([]byte, error) {
	return s.read(ctx, "object", id)
}
func (s Store) read(ctx context.Context, kind, id string) ([]byte, error) {
	if !validID(id) {
		return nil, fmt.Errorf("invalid context %s reference %q", kind, id)
	}
	b, err := s.objects().Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("read context %s %s: %w", kind, id, err)
	}
	if objectID(b) != id {
		return nil, fmt.Errorf("damaged context %s %s", kind, id)
	}
	return b, nil
}

// Load verifies a snapshot and every referenced complete value. An empty
// reference means empty input; missing or damaged objects return an error.
func (s Store) Load(ctx context.Context, ref Snapshot) ([]Entry, error) {
	if ref == "" {
		return nil, nil
	} // only the initial, empty context
	b, err := s.read(ctx, "snapshots", string(ref))
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
		if _, err = s.Value(ctx, e); err != nil {
			return nil, err
		}
	}
	return entries, nil
}

// Value resolves the complete JSON value of an entry, including file contents.
func (s Store) Value(ctx context.Context, e Entry) (json.RawMessage, error) {
	if e.File == "" {
		if !json.Valid(e.Value) {
			return nil, fmt.Errorf("invalid context value %q", e.Key)
		}
		return e.Value, nil
	}
	b, err := s.read(ctx, "blobs", e.File)
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
func (s Store) Extend(ctx context.Context, base Snapshot, writes ...Entry) (Snapshot, error) {
	entries, err := s.Load(ctx, base)
	if err != nil {
		return "", err
	}
	for _, e := range writes {
		raw, err := s.Value(ctx, e)
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
			e.File, err = s.publish(ctx, "blobs", data)
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
	id, err := s.publish(ctx, "snapshots", b)
	return Snapshot(id), err
}

// Materialize makes a consumer-local copy, separate from immutable backing
// objects. Each dispatch repairs changed copies before advertising their paths.
func (s Store) Materialize(ctx context.Context, e Entry) (string, error) {
	b, err := s.read(ctx, "blobs", e.File)
	if err != nil {
		return "", err
	}
	return s.MaterializeText(e.File, b)
}

// MaterializeText verifies and repairs a local copy of an already identified object.
func (s Store) MaterializeText(id string, b []byte) (string, error) {
	if !validID(id) || objectID(b) != id {
		return "", fmt.Errorf("invalid context materialization")
	}
	dir := s.localDir()
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

// Text publishes and materializes text, returning its object ID and local path.
func (s Store) Text(ctx context.Context, text string) (string, string, error) {
	id, err := s.publish(ctx, "blobs", []byte(text))
	if err != nil {
		return "", "", err
	}
	path, err := s.MaterializeText(id, []byte(text))
	return id, path, err
}
