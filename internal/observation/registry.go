package observation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/runstore"
)

// Registry is a project's runs: the one the web runtime puts in the context
// it serves from. gimbal.Run finds it there, so there is no global map, and a
// run started without the web runtime simply does not find one.
//
// A run stays in the map after it finishes, so it is answered from memory
// rather than replayed again. A run this process never saw is replayed from
// its directory on first sight and kept. Nothing is evicted: a project has
// tens of runs.
type Registry struct {
	dir       string
	Remote    func(string) (backend.Connection, error)
	Placement func(string) (string, string)

	mu   sync.RWMutex
	runs map[string]*Store
}

// NewRegistry returns the registry for a project directory.
func NewRegistry(projectDir string) *Registry {
	return &Registry{dir: projectDir, runs: map[string]*Store{}}
}

type registryKey struct{}

// WithRegistry puts the registry in ctx.
func WithRegistry(ctx context.Context, registry *Registry) context.Context {
	return context.WithValue(ctx, registryKey{}, registry)
}

// FromContext returns the registry in ctx, or nil.
func FromContext(ctx context.Context) *Registry {
	if ctx == nil {
		return nil
	}
	registry, _ := ctx.Value(registryKey{}).(*Registry)
	return registry
}

func (r *Registry) add(s *Store) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.runs[s.id]; exists {
		return fmt.Errorf("observation: run %s already registered", s.id)
	}
	r.runs[s.id] = s
	return nil
}

func (r *Registry) remove(s *Store) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.runs[s.id] == s {
		delete(r.runs, s.id)
	}
}

// Live returns the store of a run that is still going. A finished run is
// still in the map and still readable, but there is no suffix to subscribe to.
func (r *Registry) Live(id string) (*Store, bool) {
	if r == nil {
		return nil, false
	}
	if _, err := r.Snapshot(id); err != nil {
		return nil, false
	}
	r.mu.RLock()
	store, ok := r.runs[id]
	r.mu.RUnlock()
	if !ok || !store.isOpen() {
		return nil, false
	}
	return store, true
}

// ErrNoRun is returned for a run id this project has never had.
var ErrNoRun = errors.New("observation: no such run")

// Snapshot returns a run's complete observation: from its store while that
// run is in the map, whether it is going or finished, and otherwise by
// reading durable state. External stores follow the writer's journal on each
// call. The registry lock serializes initialization and refresh.
func (r *Registry) Snapshot(id string) (RunSnapshot, error) {
	if r == nil {
		return RunSnapshot{}, ErrNoRun
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if store, ok := r.runs[id]; ok {
		store.mu.Lock()
		initializing := store.initializing
		store.mu.Unlock()
		if initializing {
			return RunSnapshot{}, runstore.ErrIncomplete
		}
		if store.external {
			if err := store.refresh(); err != nil {
				return RunSnapshot{}, err
			}
		}
		return r.decorate(id, store.Snapshot()), nil
	}
	if r.Remote != nil {
		connection, err := r.Remote(id)
		if err != nil {
			return RunSnapshot{}, err
		}
		if connection.Observations != nil {
			store := OpenSource(r, id, id, connection.Observations)
			if err := store.refresh(); err != nil {
				return RunSnapshot{}, err
			}
			r.runs[id] = store
			return r.decorate(id, store.Snapshot()), nil
		}
	}
	dir, err := r.runDir(id)
	if err != nil {
		return RunSnapshot{}, err
	}
	if !recognizedProjection(dir) {
		if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
			return RunSnapshot{}, ErrNoRun
		}
		return RunSnapshot{}, runstore.ErrIncomplete
	}
	store, err := open(r, id, dir, true)
	if err != nil {
		return RunSnapshot{}, err
	}
	store.external, store.noWrite = true, true
	store.closed = store.run.Status != StatusRunning
	r.runs[id] = store
	if store.external {
		if err := store.refresh(); err != nil {
			return RunSnapshot{}, err
		}
	}
	return r.decorate(id, store.Snapshot()), nil
}

// runDir is the run's directory below the project. A run id that could name
// anything outside it is refused rather than cleaned into something else.
func (r *Registry) runDir(id string) (string, error) {
	if r.dir == "" || id == "" || id == "." || id == ".." ||
		strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return "", ErrNoRun
	}
	return filepath.Join(r.dir, "runs", id), nil
}

// IDs includes local run directories and central-owned remote route identities.
func (r *Registry) IDs() ([]string, error) {
	seen := map[string]bool{}
	entries, err := os.ReadDir(filepath.Join(r.dir, "runs"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() && recognizedProjection(filepath.Join(r.dir, "runs", entry.Name())) {
			seen[entry.Name()] = true
		}
	}
	entries, err = os.ReadDir(filepath.Join(r.dir, "routes"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			seen[strings.TrimSuffix(entry.Name(), ".json")] = true
		}
	}
	r.mu.RLock()
	for id, s := range r.runs {
		s.mu.Lock()
		initializing := s.initializing
		s.mu.Unlock()
		if !initializing {
			seen[id] = true
		}
	}
	r.mu.RUnlock()
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids, nil
}

func (r *Registry) decorate(id string, snapshot RunSnapshot) RunSnapshot {
	if r.Placement != nil {
		snapshot.Run.WorkflowEnvironment, snapshot.Run.TaskEnvironment = r.Placement(id)
	}
	placements := map[string]bool{}
	for _, row := range snapshot.Sessions {
		if row.WorkerID != "" {
			placements[row.Environment+" · "+row.WorkerID] = true
		}
	}
	for _, row := range snapshot.Commands {
		if row.WorkerID != "" {
			placements[row.Environment+" · "+row.WorkerID] = true
		}
	}
	if len(placements) > 1 {
		snapshot.Run.TaskEnvironment = "mixed"
	} else {
		for placement := range placements {
			snapshot.Run.TaskEnvironment = placement
		}
	}
	return snapshot
}

// Bare and incompletely initialized store directories are not historical runs.
func recognizedProjection(dir string) bool {
	for _, name := range []string{"run.json", "run.jsonl", snapshotFile} {
		if info, err := os.Stat(filepath.Join(dir, name)); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}
