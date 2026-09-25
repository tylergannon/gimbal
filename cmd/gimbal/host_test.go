package main

import (
	"context"
	"path/filepath"

	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/web"
)

func testProject(ctx context.Context, dir string, opts ...web.Option) (*web.Instance, *host.Project, error) {
	instance, err := web.NewInstance(ctx, filepath.Join(dir, ".gimbal"), []string{dir}, opts...)
	if err != nil {
		return nil, nil, err
	}
	project, err := instance.Owner.Project(dir)
	return instance, project, err
}
