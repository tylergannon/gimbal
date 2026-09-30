package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/tylergannon/gimbal/internal/observation"
)

type runtimeDiscovery struct {
	PID    int    `json:"pid"`
	Socket string `json:"socket"`
}

type runtimeClient struct {
	pid     int
	socket  string
	project string
}

type listedRun struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	PID    int    `json:"pid"`
}

type runsOutput struct {
	Runs []listedRun `json:"runs"`
}

func newRunsCommand() *cobra.Command {
	var workDir string
	command := &cobra.Command{
		Use:   "runs",
		Short: "List runs in progress in this project's running Gimbal instance",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return listRuns(cmd.Context(), cmd.OutOrStdout(), workDir)
		},
	}
	command.Flags().StringVar(&workDir, "work-dir", ".", "repository directory owning this project's .gimbal state")
	return command
}

func newWatchCommand() *cobra.Command {
	var workDir string
	command := &cobra.Command{
		Use:   "watch RUN",
		Short: "Show a run snapshot and follow its observation stream",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return watchRun(ctx, cmd.OutOrStdout(), workDir, args[0])
		},
	}
	command.Flags().StringVar(&workDir, "work-dir", ".", "repository directory owning this project's .gimbal state")
	return command
}

func listRuns(ctx context.Context, output io.Writer, workDir string) error {
	clients, err := discoverRuntimes(workDir)
	if err != nil {
		return err
	}
	out := runsOutput{Runs: []listedRun{}}
	for _, runtime := range clients {
		rows, ok := runtime.runs(ctx)
		if !ok {
			continue
		}
		for _, row := range rows {
			out.Runs = append(out.Runs, listedRun{ID: row.ID, Name: row.Name, Status: row.Status, PID: runtime.pid})
		}
	}
	slices.SortFunc(out.Runs, func(a, b listedRun) int {
		return strings.Compare(a.ID, b.ID)
	})
	return json.NewEncoder(output).Encode(out)
}

func watchRun(ctx context.Context, output io.Writer, workDir, runID string) error {
	runtime, ok, err := findRun(ctx, workDir, runID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no running Gimbal instance owns run %q", runID)
	}
	response, cleanup, err := runtime.request(ctx, http.MethodGet, "/api/runs/"+url.PathEscape(runID)+"/events", nil)
	if err != nil {
		return err
	}
	defer cleanup()
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("watch %s: runtime returned %s", runID, response.Status)
	}
	return streamObservation(output, response.Body)
}

func findRun(ctx context.Context, workDir, runID string) (runtimeClient, bool, error) {
	clients, err := discoverRuntimes(workDir)
	if err != nil {
		return runtimeClient{}, false, err
	}
	for _, runtime := range clients {
		rows, ok := runtime.runs(ctx)
		if !ok {
			continue
		}
		for _, row := range rows {
			if row.ID == runID {
				return runtime, true, nil
			}
		}
	}
	return runtimeClient{}, false, nil
}

func (r runtimeClient) request(ctx context.Context, method, path string, body io.Reader) (*http.Response, func(), error) {
	request, err := http.NewRequestWithContext(ctx, method, "http://gimbal"+path, body)
	if err != nil {
		return nil, func() {}, err
	}
	request.Header.Set("X-Gimbal-Project", r.project)
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", r.socket)
	}}
	response, err := (&http.Client{Transport: transport}).Do(request)
	if err != nil {
		transport.CloseIdleConnections()
		return nil, func() {}, err
	}
	return response, transport.CloseIdleConnections, nil
}

func streamObservation(output io.Writer, input io.Reader) error {
	encoder := json.NewEncoder(output)
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	event := ""
	var data []string
	flush := func() error {
		if event == "" || len(data) == 0 {
			return nil
		}
		var value json.RawMessage
		if err := json.Unmarshal([]byte(strings.Join(data, "\n")), &value); err != nil {
			return err
		}
		if event == "snapshot" {
			var snapshot map[string]json.RawMessage
			if err := json.Unmarshal(value, &snapshot); err != nil {
				return err
			}
			snapshot["type"] = json.RawMessage(`"snapshot"`)
			return encoder.Encode(snapshot)
		}
		return encoder.Encode(struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}{Type: event, Data: value})
	}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if err := flush(); err != nil {
				return err
			}
			event, data = "", nil
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}

func discoverRuntimes(workDir string) ([]runtimeClient, error) {
	project, err := filepath.Abs(workDir)
	if err != nil {
		return nil, err
	}
	project, err = filepath.EvalSymlinks(project)
	if err != nil {
		return nil, err
	}
	controlDir := filepath.Join(project, ".gimbal", "control")
	entries, err := os.ReadDir(controlDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	clients := make([]runtimeClient, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(controlDir, entry.Name()))
		if err != nil {
			continue
		}
		var discovery runtimeDiscovery
		if json.Unmarshal(contents, &discovery) != nil || discovery.PID == 0 || discovery.Socket == "" {
			continue
		}
		clients = append(clients, runtimeClient{pid: discovery.PID, socket: discovery.Socket, project: project})
	}
	slices.SortFunc(clients, func(a, b runtimeClient) int { return a.pid - b.pid })
	return clients, nil
}

func (r runtimeClient) runs(ctx context.Context) ([]observation.RunRow, bool) {
	requestContext, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	response, cleanup, err := r.request(requestContext, http.MethodGet, "/control/runs", nil)
	if err != nil {
		return nil, false
	}
	defer cleanup()
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, false
	}
	var rows []observation.RunRow
	if err := json.NewDecoder(response.Body).Decode(&rows); err != nil {
		return nil, false
	}
	return rows, true
}
