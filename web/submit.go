package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/tylergannon/gimble/internal/host"
	"github.com/tylergannon/gimble/internal/observation"
	"github.com/tylergannon/skgo"
)

// Follow waits for the selected instance's project-scoped terminal run row.
func Follow(ctx context.Context, instanceDir, project, id string) (observation.RunRow, error) {
	client, err := selectedInstance(ctx, instanceDir, project)
	if err != nil {
		return observation.RunRow{}, err
	}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		response, err := client.request(ctx, http.MethodGet, "/api/runs/"+url.PathEscape(id), nil)
		if err != nil {
			return observation.RunRow{}, fmt.Errorf("follow run: %w", err)
		}
		if response.StatusCode != http.StatusOK {
			_ = response.Body.Close()
			return observation.RunRow{}, fmt.Errorf("follow run %s: %s", id, response.Status)
		}
		var snapshot observation.RunSnapshot
		err = json.NewDecoder(response.Body).Decode(&snapshot)
		_ = response.Body.Close()
		if err != nil {
			return observation.RunRow{}, err
		}
		if snapshot.Run.Status != observation.StatusRunning {
			return snapshot.Run, nil
		}
		select {
		case <-ctx.Done():
			return observation.RunRow{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

// SelectedFormClient connects generated SKGO clients to the selected instance's
// control socket. Start Forms carry their project path in their typed input.
func SelectedFormClient(ctx context.Context, instanceDir, project string) (skgo.FormClient, error) {
	selected, err := selectedInstance(ctx, instanceDir, project)
	if err != nil {
		return skgo.FormClient{}, err
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", selected.socket)
	}}
	return skgo.FormClient{BaseURL: "http://gimble", HTTPClient: &http.Client{Transport: transport}}, nil
}

type selectedClient struct {
	socket, project string
}

func selectedInstance(ctx context.Context, instanceDir, project string) (selectedClient, error) {
	instanceDir, err := filepath.Abs(instanceDir)
	if err != nil {
		return selectedClient{}, err
	}
	project, err = host.CanonicalProject(project)
	if err != nil {
		return selectedClient{}, err
	}
	entries, err := os.ReadDir(filepath.Join(instanceDir, "control"))
	if errors.Is(err, os.ErrNotExist) {
		return selectedClient{}, fmt.Errorf("no running Gimble instance at %s; start gimble --instance-dir %s --project %s", instanceDir, instanceDir, project)
	}
	if err != nil {
		return selectedClient{}, err
	}
	var clients []selectedClient
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(instanceDir, "control", entry.Name()))
		if err != nil {
			continue
		}
		var discovery controlDiscovery
		if json.Unmarshal(data, &discovery) == nil && discovery.Socket != "" {
			candidate := selectedClient{socket: discovery.Socket, project: project}
			probe, err := candidate.request(ctx, http.MethodGet, "/control/live", nil)
			if err == nil {
				_ = probe.Body.Close()
				if probe.StatusCode == http.StatusOK {
					clients = append(clients, candidate)
				}
			}
		}
	}
	if len(clients) != 1 {
		return selectedClient{}, fmt.Errorf("selected Gimble instance at %s has %d live endpoints; start or select one instance with --instance-dir", instanceDir, len(clients))
	}
	return clients[0], nil
}

func (c selectedClient) request(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, "http://gimble"+path, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Gimble-Project", c.project)
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", c.socket)
	}}
	response, err := (&http.Client{Transport: transport}).Do(request)
	transport.CloseIdleConnections()
	return response, err
}
