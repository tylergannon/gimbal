package routes

import (
	"context"

	hooks "github.com/tylergannon/gimble/web/src"
	"github.com/tylergannon/skgo"
)

type ProjectsData struct {
	Projects []hooks.ProjectChoice `json:"projects"`
}

func load(ctx context.Context) (ProjectsData, error) {
	if request := skgo.EventFrom(ctx).Request(); request != nil {
		ctx = request.Context()
	}
	return ProjectsData{Projects: hooks.Projects(ctx)}, nil
}

var _ = skgo.Load(load)
