package runs

import (
	"github.com/tylergannon/gimbal/web/runview"
	"github.com/tylergannon/skgo"
)

func load(event PageRequestEvent) (runview.RunsData, error) {
	ctx := event.Context()
	if request := event.Request(); request != nil {
		ctx = request.Context()
	}
	return runview.List(ctx)
}

var _ = skgo.Load(load)
