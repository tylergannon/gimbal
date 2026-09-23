package web

import (
	"context"
	"path/filepath"

	"github.com/tylergannon/gimble/internal/host"
)

func newProject(ctx context.Context, dir string, opts ...Option) (*Instance, *host.Project, error) {
	path, err := host.CanonicalProject(dir)
	if err != nil {
		return nil, nil, err
	}
	instance, err := NewInstance(ctx, filepath.Join(path, ".gimble"), []string{path}, opts...)
	if err != nil {
		return nil, nil, err
	}
	project, err := instance.Owner.Project(path)
	return instance, project, err
}
