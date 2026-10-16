// Package runid serves the run observation page.
package runview

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
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
	// Source is the designer's source page saved by this run; "" for runs
	// started before runs saved it.
	Source string `json:"source"`
}

// ReadRun reads one project-owned run for routing and retained workspace views.
func Read(ctx context.Context, runID string) (Data, error) {
	registry := observation.FromContext(ctx)
	if registry == nil {
		return Data{}, skgo.Errorf(http.StatusInternalServerError,
			"This server has no observation registry in its context, so no run can be read.")
	}

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
	graph, err := readRunFile(ctx, registry, runID, "graph.json")
	if err != nil {
		return Data{}, err
	}
	source, err := readRunFile(ctx, registry, runID, "source.json")
	if err != nil {
		return Data{}, err
	}
	return Data{Snapshot: hooks.RunSnapshot{JSON: string(encoded)}, Graph: graph, Source: source}, nil
}

// readRunFile reads one file the run saved at start, from the daemon that
// holds the run when there is one. A file the run never saved reads as "".
func readRunFile(ctx context.Context, registry *observation.Registry, runID, name string) (string, error) {
	var raw []byte
	var err error
	local := filepath.Join(host.ProjectDir(ctx), "runs", runID, name)
	if registry.Remote != nil {
		connection, remoteErr := registry.Remote(runID)
		if remoteErr != nil {
			return "", remoteErr
		}
		if connection.Execution != nil {
			file, readErr := connection.Execution.ReadBytes(ctx, backend.Artifact{Run: runID, Name: name})
			if readErr != nil {
				err = readErr
			} else {
				raw, err = io.ReadAll(file)
				_ = file.Close()
			}
		} else {
			raw, err = os.ReadFile(local)
		}
	} else {
		raw, err = os.ReadFile(local)
	}
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return string(raw), err
}
