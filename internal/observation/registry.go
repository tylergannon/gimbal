package observation

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
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
	dir string

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

func (r *Registry) add(s *Store) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs[s.id] = s
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
// replaying its logs once. The replay happens under the registry lock, so two
// requests for the same unseen run replay it once between them.
func (r *Registry) Snapshot(id string) (RunSnapshot, error) {
	if r == nil {
		return RunSnapshot{}, ErrNoRun
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if store, ok := r.runs[id]; ok {
		return store.Snapshot(), nil
	}
	dir, err := r.runDir(id)
	if err != nil {
		return RunSnapshot{}, err
	}
	store, err := open(r, id, dir)
	if err != nil {
		return RunSnapshot{}, err
	}
	r.runs[id] = store
	return store.Snapshot(), nil
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
