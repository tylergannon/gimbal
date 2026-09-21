package routes

import (
	"context"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimble/internal/observation"
)

// watchRuns sends the complete, small runs-list snapshot whenever the runtime
// records a change to one of this project's runs.
func watchRuns(ctx context.Context, yield func(RunsData) error) error {
	event := skgo.EventFrom(ctx)
	if request := event.Request(); request != nil {
		ctx = request.Context()
	}
	registry := observation.FromContext(ctx)
	if registry == nil {
		_, err := runsData(ctx)
		return err
	}

	for {
		changed := registry.Changes()
		data, err := runsData(ctx)
		if err != nil {
			return err
		}
		if err := yield(data); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-changed:
		}
	}
}

var _ = skgo.LiveQuery(watchRuns)
