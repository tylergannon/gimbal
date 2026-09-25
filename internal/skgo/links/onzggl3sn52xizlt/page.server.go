package routes

import (
	"context"

	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/skgo"
)

type ProjectsData struct {
	Projects []host.ProjectChoice `json:"projects"`
}

func load(ctx context.Context) (ProjectsData, error) {
	if request := skgo.EventFrom(ctx).Request(); request != nil {
		ctx = request.Context()
	}
	return ProjectsData{Projects: host.Projects(ctx)}, nil
}

var _ = skgo.Load(load)
