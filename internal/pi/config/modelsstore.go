package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// ModelsStoreEntry is one provider's persisted catalog (pi ModelsStoreEntry).
type ModelsStoreEntry struct {
	Models       []*model.Model `json:"models"`
	LastModified int64          `json:"lastModified,omitempty"`
	CheckedAt    int64          `json:"checkedAt,omitempty"`
	Etag         string         `json:"etag,omitempty"`
}

// Clone returns a deep copy of the entry. Model pointers are cloned too, so a
// caller cannot mutate stored catalogs.
func (e *ModelsStoreEntry) Clone() *ModelsStoreEntry {
	if e == nil {
		return nil
	}
	out := &ModelsStoreEntry{LastModified: e.LastModified, CheckedAt: e.CheckedAt, Etag: e.Etag}
	if e.Models != nil {
		out.Models = make([]*model.Model, len(e.Models))
		for i, entry := range e.Models {
			if entry == nil {
				continue
			}
			clone := entry.Clone()
			out.Models[i] = &clone
		}
	}
	return out
}

// ModelsStore is persistent model-catalog storage keyed by provider id. Read
// returns (nil, nil) when nothing is stored.
type ModelsStore interface {
	Read(ctx context.Context, providerID string) (*ModelsStoreEntry, error)
	Write(ctx context.Context, providerID string, entry ModelsStoreEntry) error
	Delete(ctx context.Context, providerID string) error
}

// InMemoryCodingAgentModelsStore is the in-memory ModelsStore.
type InMemoryCodingAgentModelsStore struct {
	mu      sync.Mutex
	entries map[string]*ModelsStoreEntry
}

// NewInMemoryCodingAgentModelsStore returns an empty in-memory store.
func NewInMemoryCodingAgentModelsStore() *InMemoryCodingAgentModelsStore {
	return &InMemoryCodingAgentModelsStore{entries: map[string]*ModelsStoreEntry{}}
}

// Read returns a copy of the stored entry for a provider, or (nil, nil).
func (s *InMemoryCodingAgentModelsStore) Read(ctx context.Context, providerID string) (*ModelsStoreEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.entries[providerID].Clone(), nil
}

// Write stores a copy of the entry for a provider.
func (s *InMemoryCodingAgentModelsStore) Write(ctx context.Context, providerID string, entry ModelsStoreEntry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[providerID] = entry.Clone()
	return nil
}

// Delete removes a provider's stored entry.
func (s *InMemoryCodingAgentModelsStore) Delete(ctx context.Context, providerID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, providerID)
	return nil
}

// unmarshalModelsEntry decodes one stored entry, tolerating an unknown extra
// field in entries written by a newer pi.
func unmarshalModelsEntry(raw json.RawMessage) (ModelsStoreEntry, error) {
	var entry ModelsStoreEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return ModelsStoreEntry{}, err
	}
	return entry, nil
}

type modelsFileState struct {
	mu       sync.Mutex
	loaded   bool
	revision string
	entries  map[string]ModelsStoreEntry
}

// FileModelsStore is JSON-backed, lock-protected storage for dynamically
// refreshed provider catalogs.
type FileModelsStore struct {
	path  string
	state *modelsFileState
}

var (
	modelsFileStatesMu sync.Mutex
	modelsFileStates   = map[string]*modelsFileState{}
)

func sharedModelsFileState(path string) *modelsFileState {
	modelsFileStatesMu.Lock()
	defer modelsFileStatesMu.Unlock()
	state, ok := modelsFileStates[path]
	if !ok {
		state = &modelsFileState{entries: map[string]ModelsStoreEntry{}}
		modelsFileStates[path] = state
	}
	return state
}

// NewFileModelsStore opens the models store at path. An empty path uses
// <agentDir>/models-store.json.
func NewFileModelsStore(path string) *FileModelsStore {
	if path == "" {
		path = filepath.Join(GetAgentDir(), "models-store.json")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return &FileModelsStore{path: abs, state: sharedModelsFileState(abs)}
}

// Path returns the absolute path of the backing file.
func (s *FileModelsStore) Path() string { return s.path }

func (s *FileModelsStore) readFile() (map[string]ModelsStoreEntry, string, error) {
	revision := getFileRevision(s.path)
	content, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]ModelsStoreEntry{}, revision, nil
		}
		return nil, revision, err
	}
	entries := map[string]ModelsStoreEntry{}
	trimmed := strings.TrimSpace(StripBOM(string(content)))
	if trimmed != "" {
		raw := map[string]json.RawMessage{}
		if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
			return nil, revision, err
		}
		for providerID, entryRaw := range raw {
			entry, err := unmarshalModelsEntry(entryRaw)
			if err != nil {
				return nil, revision, err
			}
			entries[providerID] = entry
		}
	}
	return entries, revision, nil
}

func (s *FileModelsStore) load(ctx context.Context) (map[string]ModelsStoreEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	revision := getFileRevision(s.path)
	if s.state.loaded && revision != "" && revision == s.state.revision {
		return s.cloneEntries(), nil
	}
	entries, newRevision, err := s.readFile()
	if err != nil {
		return nil, err
	}
	s.state.entries = entries
	s.state.revision = newRevision
	s.state.loaded = true
	return s.cloneEntries(), nil
}

func (s *FileModelsStore) cloneEntries() map[string]ModelsStoreEntry {
	out := make(map[string]ModelsStoreEntry, len(s.state.entries))
	for providerID, entry := range s.state.entries {
		out[providerID] = *entry.Clone()
	}
	return out
}

// Read returns a copy of the stored entry for a provider.
func (s *FileModelsStore) Read(ctx context.Context, providerID string) (*ModelsStoreEntry, error) {
	entries, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entry, ok := entries[providerID]
	if !ok {
		return nil, nil
	}
	return entry.Clone(), nil
}

// Write stores a provider's entry without replacing unrelated providers.
func (s *FileModelsStore) Write(ctx context.Context, providerID string, entry ModelsStoreEntry) error {
	return s.mutate(ctx, func(entries map[string]ModelsStoreEntry) {
		entries[providerID] = *entry.Clone()
	})
}

// Delete removes a provider's entry without touching unrelated providers.
func (s *FileModelsStore) Delete(ctx context.Context, providerID string) error {
	return s.mutate(ctx, func(entries map[string]ModelsStoreEntry) {
		delete(entries, providerID)
	})
}

func (s *FileModelsStore) mutate(ctx context.Context, apply func(map[string]ModelsStoreEntry)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return withFileLock(ctx, s.path, func() error {
		s.state.mu.Lock()
		defer s.state.mu.Unlock()

		entries, revision, err := s.readFile()
		if err != nil {
			return err
		}
		apply(entries)

		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(s.path, data, 0o644); err != nil {
			return err
		}
		s.state.entries = entries
		s.state.revision = getFileRevision(s.path)
		if s.state.revision == "" {
			s.state.revision = revision
		}
		s.state.loaded = true
		return nil
	})
}

// getFileRevision returns an opaque revision string for a path, or "" when the
// path cannot be stat'd.
func getFileRevision(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d:%d:%d:%t", info.Size(), info.Mode(), info.ModTime().UnixNano(), info.IsDir())
}

// withFileLock serializes fn across goroutines and processes using an
// exclusive lock file beside path. The context bounds the wait for the lock.
func withFileLock(ctx context.Context, path string, fn func() error) error {
	lockPath := path + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_ = file.Close()
			defer func() { _ = os.Remove(lockPath) }()
			return fn()
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out acquiring models-store lock %s", lockPath)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}
