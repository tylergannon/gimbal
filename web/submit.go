package web

import (
	"bytes"
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

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/internal/observation"
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

// Start submits pure Go invocation data through the daemon's control socket.
func Start(ctx context.Context, instanceDir string, request invocation.Request) (invocation.Accepted, error) {
	client, err := selectedInstance(ctx, instanceDir, request.Project)
	if err != nil {
		return invocation.Accepted{}, err
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return invocation.Accepted{}, err
	}
	response, err := client.request(ctx, http.MethodPost, "/control/start", bytes.NewReader(raw))
	if err != nil {
		return invocation.Accepted{}, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(response.Body)
		return invocation.Accepted{}, fmt.Errorf("start %s: %s", request.Workflow, message)
	}
	var result invocation.Accepted
	err = json.NewDecoder(response.Body).Decode(&result)
	return result, err
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
		return selectedClient{}, fmt.Errorf("no running Gimbal instance at %s; start gimbal --instance-dir %s --project %s", instanceDir, instanceDir, project)
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
		return selectedClient{}, fmt.Errorf("selected Gimbal instance at %s has %d live endpoints; start or select one instance with --instance-dir", instanceDir, len(clients))
	}
	return clients[0], nil
}

func (c selectedClient) request(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, "http://gimbal"+path, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Gimbal-Project", c.project)
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", c.socket)
	}}
	response, err := (&http.Client{Transport: transport}).Do(request)
	transport.CloseIdleConnections()
	return response, err
}

// WorkflowCatalog fetches the selected central's runtime catalog. It loads no plugins.
func WorkflowCatalog(ctx context.Context, instanceDir, project string) ([]gimbal.WorkflowDescription, error) {
	client, err := selectedInstance(ctx, instanceDir, project)
	if err != nil {
		return nil, err
	}
	response, err := client.request(ctx, http.MethodGet, "/control/workflows", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("workflow catalog: %s", response.Status)
	}
	var catalog []gimbal.WorkflowDescription
	err = json.NewDecoder(response.Body).Decode(&catalog)
	return catalog, err
}
