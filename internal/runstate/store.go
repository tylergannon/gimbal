// Package runstate resolves a project's recording store on every application
// process, including detached runners and post-exit cleanup owners.
package runstate

import (
	"context"
	"github.com/tylergannon/gimbal/runstore"
	"github.com/tylergannon/gimbal/runstore/directory"
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
