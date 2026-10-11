package routes

import (
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/skgo"
)

type WorkspaceLayout struct {
	Desktop           bool                 `json:"desktop"`
	WorkspaceProjects []host.ProjectChoice `json:"workspaceProjects"`
}

func layout(event LayoutRequestEvent) (WorkspaceLayout, error) {
	ctx := event.Context()
	desktop := false
	if r := event.Request(); r != nil {
		ctx = r.Context()
		desktop = r.Header.Get("X-Gimbal-Desktop") == "macos"
	}
	out := WorkspaceLayout{Desktop: desktop, WorkspaceProjects: []host.ProjectChoice{}}
	if owner := host.OwnerFrom(ctx); owner != nil {
		for _, p := range owner.Projects() {
			out.WorkspaceProjects = append(out.WorkspaceProjects, host.ProjectChoice{ID: p.ID(), Path: p.Path()})
		}
	}
	return out, nil
}

var _ = skgo.Load(layout)
