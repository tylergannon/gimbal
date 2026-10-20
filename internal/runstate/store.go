// Package runstate resolves a project's recording store on every application
// process, including detached runners and post-exit cleanup owners.
package runstate

import (
	"context"
	"github.com/tylergannon/gimbal/runstore"
	"github.com/tylergannon/gimbal/runstore/directory"
	"os"
	"path/filepath"
)

type storeKey struct{}

func ForProject(project string) runstore.Store {
	return directory.Store{Root: filepath.Join(project, "runs")}
}
func WithStore(ctx context.Context, store runstore.Store) context.Context {
	return context.WithValue(ctx, storeKey{}, store)
}
func Override(ctx context.Context) runstore.Store {
	s, _ := ctx.Value(storeKey{}).(runstore.Store)
	return s
}
func Resolve(ctx context.Context, project string) runstore.Store {
	if s := Override(ctx); s != nil {
		return s
	}
	return ForProject(project)
}

// Recognized detects projection format, including an initializing directory.
// It never opens the authoritative store or acquires a writer lock.
func Recognized(runDir string) bool {
	if _, e := os.Stat(filepath.Join(runDir, "recording-format")); e == nil {
		return true
	}
	_, e := os.Stat(filepath.Join(runDir, "state"))
	return e == nil
}

// InteractionInput is the journal's durable wire representation. Sequence in
// runstore, rather than a timestamp, orders admission and delivery updates.
type InteractionInput struct {
	Configure   bool   `json:"configure"`
	Model       string `json:"model"`
	Effort      string `json:"effort"`
	Stop        bool   `json:"stop"`
	ID          string `json:"id"`
	Address     string `json:"address"`
	Message     string `json:"message"`
	Finish      bool   `json:"finish"`
	Disposition string `json:"disposition"`
}
