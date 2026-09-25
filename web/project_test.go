package web

import (
	"context"
	"path/filepath"

	"github.com/tylergannon/gimbal/internal/host"
)

func newProject(ctx context.Context, dir string, opts ...Option) (*Instance, *host.Project, error) {
	path, err := host.CanonicalProject(dir)
	if err != nil {
		return nil, nil, err
	}
	instance, err := NewInstance(ctx, filepath.Join(path, ".gimbal"), []string{path}, opts...)
	if err != nil {
		return nil, nil, err
	}
	project, err := instance.Owner.Project(path)
	return instance, project, err
}
