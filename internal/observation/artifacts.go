package observation

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"time"
)

type contextArtifactsKey struct{}

// WithContextArtifacts installs the project's retained immutable object access.
// The handler checks that a run actually references the requested object.
func WithContextArtifacts(ctx context.Context, read func(string) ([]byte, error)) context.Context {
	return context.WithValue(ctx, contextArtifactsKey{}, read)
}
func serveContextArtifact(w http.ResponseWriter, r *http.Request) {
	registry := FromContext(r.Context())
	read, _ := r.Context().Value(contextArtifactsKey{}).(func(string) ([]byte, error))
	if registry == nil || read == nil {
		http.Error(w, "Retained context storage is not configured for this project.", http.StatusServiceUnavailable)
		return
	}
	snapshot, err := registry.Snapshot(r.PathValue("runID"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	id := r.PathValue("object")
	ref := "context/objects/" + id
	found := false
	for _, scope := range snapshot.Scopes {
		for _, value := range scope.Values {
			if value.Artifact != nil && value.Artifact.File == ref {
				found = true
			}
		}
	}
	if !found || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	data, err := read(id)
	if err != nil {
		http.Error(w, "Retained context object is unavailable: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, id, time.Time{}, bytes.NewReader(data))
}
