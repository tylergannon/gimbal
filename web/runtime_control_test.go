package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/observation"
)

func TestRuntimeControlSocket(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	project := t.TempDir()
	runtime, err := NewRuntime(ctx, project, WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}

	discoveries, err := filepath.Glob(filepath.Join(project, "control", "*.json"))
	if err != nil || len(discoveries) != 1 {
		t.Fatalf("control discovery files = %v, %v; want one", discoveries, err)
	}
	var discovery controlDiscovery
	contents, err := os.ReadFile(discoveries[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(contents, &discovery); err != nil {
		t.Fatal(err)
	}
	if discovery.Socket == "" || discovery.Project != project {
		t.Fatalf("discovery = %+v", discovery)
	}

	client := controlClient(discovery.Socket)
	defer client.CloseIdleConnections()
	runs := getRuns(t, client)
	if len(runs) != 0 {
		t.Fatalf("initial runs = %+v; want none", runs)
	}

	b := &blocking{}
	var runWG sync.WaitGroup
	runWG.Go(func() {
		_ = runtime.Run(ctx, "control", map[gimble.WorkflowRole]gimble.ModelBinding{
			"coder": {Adapter: b, Model: "m"},
		}, func(ctx context.Context) error {
			return gimble.Scope(ctx, "lap", func(ctx context.Context) error {
				coder := gimble.NewSession(ctx, "coder", "/w")
				gimble.NewSession(ctx, "coder", "/w") // An idle sibling session.
				_, err := coder.Generate[gimble.Text](ctx, "wait")
				return err
			})
		})
	})
	startedTurns(t, b, 1)
	id := runID(t, project)

	runs = getRuns(t, client)
	if len(runs) != 1 || runs[0].ID != id || runs[0].Status != observation.StatusRunning {
		t.Fatalf("runs = %+v; want active %s", runs, id)
	}
	response, err := client.Get("http://control/api/runs/" + id)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot observation.RunSnapshot
	if err := json.NewDecoder(response.Body).Decode(&snapshot); err != nil {
		_ = response.Body.Close()
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || len(snapshot.Sessions) != 2 {
		t.Fatalf("snapshot: status %d, sessions %+v", response.StatusCode, snapshot.Sessions)
	}

	var steerResponse struct {
		Landed bool `json:"landed"`
	}
	postJSON(t, client, "/control/steer", map[string]string{
		"run": id, "session": "lap.1/coder.1", "message": "look at this",
	}, &steerResponse)
	if !steerResponse.Landed {
		t.Fatal("control steer was dropped")
	}
	postJSON(t, client, "/control/steer", map[string]string{
		"run": id, "session": "lap.1/coder.2", "message": "nothing should receive this",
	}, &steerResponse)
	if steerResponse.Landed {
		t.Fatal("control steer to an idle session reported landed")
	}
	if err := runtime.KillTurn(id, "lap.1/coder.1/turn.1", "test", "done"); err != nil {
		t.Fatal(err)
	}
	runWG.Wait()

	cancel()
	<-runtime.done
	if _, err := os.Stat(discovery.Socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("control socket after shutdown: %v", err)
	}
	if _, err := os.Stat(discoveries[0]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("control discovery after shutdown: %v", err)
	}
}

func controlClient(socket string) *http.Client {
	return &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}}
}

func getRuns(t *testing.T, client *http.Client) []observation.RunRow {
	t.Helper()
	response, err := client.Get("http://control/control/runs")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	var runs []observation.RunRow
	if err := json.NewDecoder(response.Body).Decode(&runs); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("runs status: %d", response.StatusCode)
	}
	return runs
}

func postJSON(t *testing.T, client *http.Client, path string, value any, result any) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, "http://control"+path, strings.NewReader(string(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("POST %s: status %d, body %s", path, response.StatusCode, body)
	}
	if err := json.NewDecoder(response.Body).Decode(result); err != nil {
		t.Fatal(err)
	}
}
