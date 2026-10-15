package workspace

import (
	"context"
	"time"

	"github.com/tylergannon/gimbal/web/runview"
	"github.com/tylergannon/skgo"
)

type RunRequest struct {
	Project string `json:"project"`
	ID      string `json:"id"`
}

func viewRun(ctx context.Context, r RunRequest) (runview.Data, error) {
	p, err := project(ctx, r.Project)
	if err != nil {
		return runview.Data{}, err
	}
	return runview.Read(p.RequestContext(ctx), r.ID)
}

func watchRuns(ctx context.Context, id string, yield func(runview.RunsData) error) error {
	p, err := project(ctx, id)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		data, err := runview.List(p.RequestContext(ctx))
		if err != nil {
			return err
		}
		if err := yield(data); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

var _ = skgo.Query(viewRun)
var _ = skgo.LiveQuery(watchRuns)
