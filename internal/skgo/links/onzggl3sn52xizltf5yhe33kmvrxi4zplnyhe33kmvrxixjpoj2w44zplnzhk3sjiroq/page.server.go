// Package runid serves the run observation page.
package runid

import (
	"github.com/tylergannon/gimbal/web/runview"
	"github.com/tylergannon/skgo"
)

func load(event PageRequestEvent) (runview.Data, error) {
	ctx := event.Context()
	if request := event.Request(); request != nil {
		ctx = request.Context()
	}
	return runview.Read(ctx, event.Params.RunID())
}

var _ = skgo.Load(load)
