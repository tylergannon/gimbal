package routes

import (
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/skgo"
)

type ProjectsData struct {
	Projects []host.ProjectChoice `json:"projects"`
}

func load(event PageRequestEvent) (ProjectsData, error) {
	ctx := event.Context()
	if request := event.Request(); request != nil {
		ctx = request.Context()
	}
	return ProjectsData{Projects: host.Projects(ctx)}, nil
}

var _ = skgo.Load(load)
