package window

import (
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/skgo"
)

type Opening struct{}

// The desktop window opens here, into the project and view it last showed or
// the first project. With no project, or outside the desktop, it is the
// projects home, which stays reachable at `/`.
func load(event PageRequestEvent) (Opening, error) {
	location := "/"
	if request := event.Request(); request != nil && request.Header.Get("X-Gimbal-Desktop") == "macos" {
		if owner := host.OwnerFrom(request.Context()); owner != nil {
			if window := owner.Window(); window.Project != "" {
				location = window.Path
			}
		}
	}
	return Opening{}, &skgo.Redirect{Location: location}
}

var _ = skgo.Load(load)
