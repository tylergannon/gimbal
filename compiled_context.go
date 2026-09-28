package gimbal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimbal/internal/compiledscope"
)

type compiledContextKey struct{}
type compiledContext struct {
	values []visibleValue
	store  compiledscope.Store
	ref    compiledscope.Snapshot
}

func init() {
	compiledscope.WriteContext = writeCompiledContext
	compiledscope.BindContext = bindCompiledContext
}

func writeCompiledContext(ctx context.Context, store compiledscope.Store, base compiledscope.Snapshot, writes ...compiledscope.Entry) (compiledscope.Snapshot, error) {
	s, err := current(ctx)
	if err != nil {
		return "", err
	}
	if err := configureCompiledStore(s.run, store); err != nil {
		return "", err
	}
	ref, err := store.Extend(base, writes...)
	if err != nil {
		return "", err
	}
	entries, err := store.Load(ref)
	if err != nil {
		return "", err
	}
	for _, write := range writes {
		for _, e := range entries {
			if e.Key != write.Key {
				continue
			}
			value, err := compiledValue(s, store, e)
			if err != nil {
				return "", err
			}
			s.mu.Lock()
			if _, exists := s.values[e.Key]; exists || s.ended {
				s.mu.Unlock()
				return "", fmt.Errorf("compiled context key %q already set or scope ended", e.Key)
			}
			if s.values == nil {
				s.values = map[string]*scopeValue{}
			}
			s.keys = append(s.keys, e.Key)
			s.values[e.Key] = value
			s.mu.Unlock()
			s.run.event(s.key, "", "", valueEvent(e.Key, value))
		}
	}
	return ref, nil
}

func configureCompiledStore(r *run, store compiledscope.Store) error {
	// Installed before the first operation. Subsequent activities use the same
	// run store; scope/session ownership remains independent of context storage.
	r.mu.Lock()
	if r.contextStore == nil {
		// The specimen mounts the same run-directory layout on host and worker.
		// UI artifact references resolve through this relative observation link.
		rel, linkErr := filepath.Rel(r.dir, store.Root)
		if linkErr == nil {
			linkErr = os.Symlink(rel, filepath.Join(r.dir, "context"))
		}
		if linkErr != nil {
			r.mu.Unlock()
			return linkErr
		}
		copy := store
		r.contextStore = &copy
	}
	mismatch := r.contextStore.Root != store.Root
	r.mu.Unlock()
	if mismatch {
		return fmt.Errorf("compiled context store changed within run")
	}

	return nil
}

func compiledValue(owner *scope, store compiledscope.Store, e compiledscope.Entry) (*scopeValue, error) {
	raw, err := store.Value(e)
	if err != nil {
		return nil, err
	}
	value := &scopeValue{owner: owner, raw: raw}
	if e.File != "" {
		path, err := store.Materialize(e)
		if err != nil {
			return nil, err
		}
		text := render(raw)
		desc := artifactDescriptor{file: "context/materialized/" + e.File, local: path, format: e.Format, preview: preview(text), size: int64(len(raw))}
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

func bindCompiledContext(ctx context.Context, store compiledscope.Store, ref compiledscope.Snapshot) (context.Context, error) {
	s, err := current(ctx)
	if err != nil {
		return nil, err
	}
	if err := configureCompiledStore(s.run, store); err != nil {
		return nil, err
	}
	entries, err := store.Load(ref)
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
		value, err := compiledValue(owner, store, e)
		if err != nil {
			return nil, err
		}
		values = append(values, visibleValue{owner: owner, key: e.Key, value: value})
	}
	return context.WithValue(ctx, compiledContextKey{}, compiledContext{values: values, store: store, ref: ref}), nil
}

func (r *run) writeCompiledArtifact(data []byte, format, text string) (artifactDescriptor, error) {
	id, path, err := r.contextStore.Text(string(data))
	if err != nil {
		return artifactDescriptor{}, err
	}
	return artifactDescriptor{file: filepath.ToSlash(filepath.Join("context", "materialized", id)), local: path, size: int64(len(data)), format: format, preview: preview(text)}, nil
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
