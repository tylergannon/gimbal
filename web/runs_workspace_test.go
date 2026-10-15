package web

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/devalue/v5"
	generated "github.com/tylergannon/gimbal/internal/skgo"
)

func TestRetainedRunQueryUsesExplicitProjectWithoutReferer(t *testing.T) {
	base := t.TempDir()
	instance, err := NewInstance(t.Context(), filepath.Join(base, "instance"), nil, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { instance.cancel(nil); instance.Wait() })
	a, err := instance.Owner.AdmitProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b, err := instance.Owner.AdmitProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store := openTestRun(t, a.Registry(), a.Dir(), "owned-run", "review")
	defer func() { _ = store.Close() }()
	if err := store.Lifecycle(json.RawMessage(`{"seq":1,"time":"2026-10-14T12:00:00Z","scope":"","event":{"kind":"run_started","name":"review"}}`)); err != nil {
		t.Fatal(err)
	}
	id := ""
	for _, remote := range generated.Remotes() {
		if remote.Module() == "src/lib/runs.remote.ts" && remote.Name() == "viewRun" {
			id = remote.ID()
		}
	}
	if id == "" {
		t.Fatal("viewRun remote not registered")
	}
	origin := "http://" + instance.Address()
	for _, tc := range []struct {
		name, project, referer string
		status                 int
	}{
		{"native", a.ID(), "", 200},
		{"different visible project", a.ID(), origin + "/projects/" + b.ID() + "/recordings", 200},
		{"wrong project", b.ID(), "", 404},
		{"unadmitted", "unknown", "", 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := devalue.Stringify(map[string]any{"project": tc.project, "id": "owned-run"})
			if err != nil {
				t.Fatal(err)
			}
			request, err := http.NewRequest(http.MethodGet, origin+"/_app/remote/"+id+"?payload="+base64.RawURLEncoding.EncodeToString([]byte(encoded)), nil)
			if err != nil {
				t.Fatal(err)
			}
			if tc.referer != "" {
				request.Header.Set("Referer", tc.referer)
			}
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = response.Body.Close() }()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				Type  string `json:"type"`
				Error struct {
					Status int `json:"status"`
				} `json:"error"`
			}
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != 200 || (tc.status == 200 && result.Type != "result") || (tc.status != 200 && (result.Type != "error" || result.Error.Status != tc.status)) {
				t.Fatalf("status=%d remote=%+v body=%s", response.StatusCode, result, body)
			}
			if tc.status == 200 && !strings.Contains(string(body), "owned-run") {
				t.Fatalf("missing own observation: %s", body)
			}
		})
	}
}
