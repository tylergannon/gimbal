package gimbal

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimbal/internal/compiledscope"
)

type compiledContextKey struct{}
type compiledContext struct {
	values []visibleValue
	ref    compiledscope.Snapshot
}

func (compiledRuntime) WriteContext(ctx context.Context, base compiledscope.Snapshot, writes ...compiledscope.Entry) (compiledscope.Snapshot, error) {
	return writeCompiledContext(ctx, base, writes...)
}
func (compiledRuntime) BindContext(ctx context.Context, ref compiledscope.Snapshot) (context.Context, error) {
	return bindCompiledContext(ctx, ref)
}
func (compiledRuntime) InitializeContext(ctx context.Context, store compiledscope.Store, localDir string, initial compiledscope.Snapshot) error {
	s, err := current(ctx)
	if err != nil {
		return err
	}
	store.LocalDir = localDir
	if err := configureCompiledStore(s.run, store); err != nil {
		return err
	}
	entries, err := store.Load(ctx, initial)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	// Install initial values as run observations; the caller already retains the
	// immutable input reference, so initialization does not return another one.
	_, err = writeCompiledContext(ctx, "", entries...)
	return err
}
func writeCompiledContext(ctx context.Context, base compiledscope.Snapshot, writes ...compiledscope.Entry) (compiledscope.Snapshot, error) {
	s, err := current(ctx)
	if err != nil {
		return base, err
	}
	store, err := compiledStore(s.run)
	if err != nil {
		return base, err
	}
	// Reject all duplicate local writes before publishing any of them.
	s.mu.Lock()
	seen := map[string]bool{}
	for _, write := range writes {
		if _, exists := s.values[write.Key]; exists || seen[write.Key] || s.ended {
			s.mu.Unlock()
			return base, fmt.Errorf("compiled context key %q already set or scope ended", write.Key)
		}
		seen[write.Key] = true
	}
	s.mu.Unlock()
	// Immutable publication and materialization can block on consumer storage.
	// No scope state is held while those operations run.
	ref, err := store.Extend(ctx, base, writes...)
	if err != nil {
		return base, err
	}
	entries, err := store.Load(ctx, ref)
	if err != nil {
		return base, err
	}
	values := map[string]*scopeValue{}
	for _, entry := range entries {
		if !seen[entry.Key] {
			continue
		}
		value, err := compiledValue(ctx, s, store, entry)
		if err != nil {
			return base, err
		}
		values[entry.Key] = value
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// A competing write or finish may have completed while storage was busy.
	for _, write := range writes {
		if _, exists := s.values[write.Key]; exists || s.ended {
			return base, fmt.Errorf("compiled context key %q already set or scope ended", write.Key)
		}
	}
	if s.values == nil {
		s.values = map[string]*scopeValue{}
	}
	for _, write := range writes {
		value := values[write.Key]
		s.keys = append(s.keys, write.Key)
		s.values[write.Key] = value
		s.run.event(s.key, "", "", valueEvent(write.Key, value))
	}
	return ref, nil
}
func configureCompiledStore(r *run, store compiledscope.Store) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.contextStore != nil {
		return fmt.Errorf("compiled context store already configured")
	}
	if store.LocalDir == "" && store.Root == "" {
		return fmt.Errorf("compiled context requires a local materialization directory")
	}
	r.contextStore = &store
	return nil
}

func compiledStore(r *run) (compiledscope.Store, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.contextStore == nil {
		return compiledscope.Store{}, fmt.Errorf("compiled context store is not configured")
	}
	return *r.contextStore, nil
}

func compiledValue(ctx context.Context, owner *scope, store compiledscope.Store, e compiledscope.Entry) (*scopeValue, error) {
	raw, err := store.Value(ctx, e)
	if err != nil {
		return nil, err
	}
	value := &scopeValue{owner: owner, raw: raw}
	if e.File != "" {
		path, err := store.Materialize(ctx, e)
		if err != nil {
			return nil, err
		}
		text := render(raw)
		desc := artifactDescriptor{file: "context/objects/" + e.File, local: path, format: e.Format, preview: preview(text), size: int64(len(raw))}
		if e.Format == "text" {
			desc.size = int64(len(text))
		}
		if len(e.Value) > 0 {
			desc.preview = "Supplied summary (complete value in referenced file):\n" + render(e.Value)
		}
		value.raw = nil
		value.artifact = &desc
	}
	if value.artifact == nil && tokenCount("## "+e.Key+"\n\n"+render(raw)) > contextEntryTokenLimit {
		if err := owner.spillValueLocked(e.Key, value); err != nil {
			return nil, err
		}
	}
	return value, nil
}

func bindCompiledContext(ctx context.Context, ref compiledscope.Snapshot) (context.Context, error) {
	s, err := current(ctx)
	if err != nil {
		return nil, err
	}
	store, err := compiledStore(s.run)
	if err != nil {
		return nil, err
	}
	entries, err := store.Load(ctx, ref)
	if err != nil {
		return nil, err
	}
	// Provenance is optional observation metadata. It never supplies a value to
	// the operation; deleting it leaves exactly the same resolved snapshot.
	origins := map[string]string{}
	for parent := s; parent != nil; parent = parent.parent {
		parent.mu.Lock()
		for _, key := range parent.keys {
			if _, ok := origins[key]; !ok {
				origins[key] = parent.key
			}
		}
		parent.mu.Unlock()
	}
	values := make([]visibleValue, 0, len(entries))
	for _, e := range entries {
		owner := &scope{run: s.run, key: origins[e.Key]}
		value, err := compiledValue(ctx, owner, store, e)
		if err != nil {
			return nil, err
		}
		values = append(values, visibleValue{owner: owner, key: e.Key, value: value})
	}
	return context.WithValue(ctx, compiledContextKey{}, compiledContext{values: values, ref: ref}), nil
}

func (r *run) writeCompiledArtifact(data []byte, format, text string) (artifactDescriptor, error) {
	id, path, err := r.contextStore.Text(context.Background(), string(data))
	if err != nil {
		return artifactDescriptor{}, err
	}
	return artifactDescriptor{file: filepath.ToSlash(filepath.Join("context", "objects", id)), local: path, size: int64(len(data)), format: format, preview: preview(text)}, nil
}

// Templates may deliberately select just one value. Keep that presentation,
// while exposing omitted input through a single budgeted local index.
func compiledTemplateContext(ctx context.Context, text string) (string, error) {
	values := visibleValues(ctx)
	missing := false
	for _, e := range templateContextEntries(ctx, text) {
		if !e.Complete {
			missing = true
			break
		}
	}
	if !missing {
		return budgetRenderedText(ctx, "scope-template", text, contextTokenLimit)
	}
	var index strings.Builder
	for _, v := range values {
		if err := v.owner.ensureArtifact(v.key, v.value); err != nil {
			return "", err
		}
		fmt.Fprintf(&index, "## %s\n\nComplete value: %s\n\n", v.key, artifactAbsolute(v.owner.run, *v.value.artifact))
	}
	s, err := current(ctx)
	if err != nil {
		return "", err
	}
	desc, err := s.run.writeContentArtifact("scope-index", index.String())
	if err != nil {
		return "", err
	}
	suffix := "\n\nComplete scoped context index: " + artifactAbsolute(s.run, desc)
	text, err = budgetRenderedText(ctx, "scope-template", text, contextTokenLimit-tokenCount(suffix))
	return strings.TrimSpace(text + suffix), err
}
