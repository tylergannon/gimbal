// Package runid serves the run observation page.
package runid

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/observation"
	hooks "github.com/tylergannon/gimbal/web/src"
)

// Data is what the page renders from. The snapshot crosses as the app's
// transported type, so the component receives the observation itself.
type Data struct {
	Snapshot hooks.RunSnapshot `json:"snapshot"`
	// Graph is the workflow graph saved by this run, independent of the daemon build.
	Graph string `json:"graph"`
}

// load answers the page's data from the run registry the server was started
// with. It reads the same registry, through the same context, that the JSON
// endpoint and the event stream read: there is one observation of a run in
// this process, and this is a rendering of it rather than a second source.
func load(event PageRequestEvent) (Data, error) {
	ctx := event.Context()
	if request := event.Request(); request != nil {
		// The instance attaches the admitted project's registry to the
		// request context, which an SSR render also carries.
		ctx = request.Context()
	}
	registry := observation.FromContext(ctx)
	if registry == nil {
		return Data{}, skgo.Errorf(http.StatusInternalServerError,
			"This server has no observation registry in its context, so no run can be read.")
	}

	runID := event.Params.RunID()
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
	var raw []byte
	if registry.Remote != nil {
		connection, remoteErr := registry.Remote(runID)
		if remoteErr != nil {
			return Data{}, remoteErr
		}
		if connection.Execution != nil {
			file, readErr := connection.Execution.ReadBytes(ctx, backend.Artifact{Owner: backend.Owner{Run: runID}, Name: "graph.json"})
			if readErr != nil {
				return Data{}, readErr
			}
			raw, err = io.ReadAll(file)
			_ = file.Close()
		} else {
			raw, err = os.ReadFile(filepath.Join(host.ProjectDir(ctx), "runs", runID, "graph.json"))
		}
	} else {
		raw, err = os.ReadFile(filepath.Join(host.ProjectDir(ctx), "runs", runID, "graph.json"))
	}
	if err == nil {
		graphJSON = string(raw)
	} else if !os.IsNotExist(err) {
		return Data{}, err
	}
	return Data{Snapshot: hooks.RunSnapshot{JSON: string(encoded)}, Graph: graphJSON}, nil
}

var _ = skgo.Load(load)
