package routes

import (
	"github.com/tylergannon/gimbal/auth"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/webauth"
	"github.com/tylergannon/polytype"
	"github.com/tylergannon/skgo"
)

type WorkspaceLayout struct {
	AuthEnabled       bool                               `json:"authEnabled"`
	User              polytype.Nullable[AuthUserSummary] `json:"user"`
	Desktop           bool                               `json:"desktop"`
	WorkspaceProjects []host.ProjectChoice               `json:"workspaceProjects"`
}

func layout(event LayoutRequestEvent) (WorkspaceLayout, error) {
	event.RouteID()
	ctx := event.Context()
	desktop := false
	if r := event.Request(); r != nil {
		ctx = r.Context()
		desktop = r.Header.Get("X-Gimbal-Desktop") == "macos"
	}
	out := WorkspaceLayout{Desktop: desktop, WorkspaceProjects: []host.ProjectChoice{}}
	out.AuthEnabled = webauth.From(ctx) != nil
	if user := auth.UserFrom(ctx); out.AuthEnabled && user != nil {
		out.User = polytype.Nullable[AuthUserSummary]{Present: true, Value: AuthUserSummary{ID: user.ID(), Name: user.Name()}}
	}
	if out.AuthEnabled && !out.User.Present {
		return out, nil
	}
	if owner := host.OwnerFrom(ctx); owner != nil {
		for _, p := range owner.Projects() {
			out.WorkspaceProjects = append(out.WorkspaceProjects, host.ProjectChoice{ID: p.ID(), Path: p.Path()})
		}
	}
	return out, nil
}

var _ = skgo.Load(layout)
