package workspace

import (
	"context"
	"os"
	"path/filepath"

	"github.com/tylergannon/gimbal/internal/app"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/skgo/params"
	"github.com/tylergannon/skgo"
)

// openProject admits a folder the desktop's own folder panel chose. Unlike a
// typed path, the folder already exists, so nothing is created.
func openProject(ctx context.Context, event app.RequestEvent[params.Params], r NewProject) (host.ProjectChoice, error) {
	o := host.OwnerFrom(ctx)
	if o == nil {
		return host.ProjectChoice{}, skgo.Errorf(503, "Server unavailable")
	}
	if info, e := os.Stat(r.Path); !filepath.IsAbs(r.Path) || e != nil || !info.IsDir() {
		return host.ProjectChoice{}, skgo.Errorf(400, "Choose an existing folder")
	}
	p, e := o.AdmitProject(r.Path)
	if e != nil {
		return host.ProjectChoice{}, e
	}
	return host.ProjectChoice{ID: p.ID(), Path: p.Path()}, nil
}

// rememberWindow records where the desktop window is, so it reopens there.
func rememberWindow(ctx context.Context, event app.RequestEvent[params.Params], w host.WindowState) (bool, error) {
	o := host.OwnerFrom(ctx)
	if o == nil || event.Request().Header.Get("X-Gimbal-Desktop") != "macos" {
		return false, nil
	}
	if e := o.RememberWindow(w); e != nil {
		return false, skgo.Errorf(404, "Project not found")
	}
	return true, nil
}

var _ = skgo.Command(openProject)
var _ = skgo.Command(rememberWindow)
