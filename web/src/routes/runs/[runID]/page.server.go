// Package runid serves the run observation page.
package runid

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/observation"
	hooks "github.com/tylergannon/gimble/web/src"
)

// Data is what the page renders from. The snapshot crosses as the app's
// transported type, so the component receives the observation itself.
type Data struct {
	Snapshot hooks.RunSnapshot `json:"snapshot"`
	// Graph is the registered workflow graph's own JSON. It is empty when this
	// binary has no graph under the run's workflow name; the page then renders
	// the durable history without inventing a source map.
	Graph string `json:"graph"`
}

// load answers the page's data from the run registry the server was started
// with. It reads the same registry, through the same context, that the JSON
// endpoint and the event stream read: there is one observation of a run in
// this process, and this is a rendering of it rather than a second source.
func load(ctx context.Context) (Data, error) {
	event := skgo.EventFrom(ctx)
	if request := event.Request(); request != nil {
		// The registry rides on the request context the runtime supplies
		// through BaseContext, which is also what an SSR render carries.
		ctx = request.Context()
	}
	registry := observation.FromContext(ctx)
	if registry == nil {
		return Data{}, skgo.Errorf(http.StatusInternalServerError,
			"This server has no observation registry in its context, so no run can be read.")
	}

	runID := event.Param("runID")
	snapshot, err := registry.Snapshot(runID)
	if errors.Is(err, observation.ErrNoRun) {
		return Data{}, skgo.Errorf(http.StatusNotFound, "There is no run %s in this project.", runID)
	}
	if err != nil {
		return Data{}, err
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return Data{}, err
	}
	graphJSON := ""
	if graph, ok := gimble.RegisteredGraph(snapshot.Run.Name); ok {
		encodedGraph, err := json.Marshal(graph)
		if err != nil {
			return Data{}, err
		}
		graphJSON = string(encodedGraph)
	}
	return Data{Snapshot: hooks.RunSnapshot{JSON: string(encoded)}, Graph: graphJSON}, nil
}

var _ = skgo.Load(load)
