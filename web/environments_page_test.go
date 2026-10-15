package web

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/devalue/v5"
	generated "github.com/tylergannon/gimbal/internal/skgo"
)

func TestMachinesSettingsRemotesWorkWithoutProjectReferer(t *testing.T) {
	instance, err := NewInstance(t.Context(), filepath.Join(t.TempDir(), "catalog"), nil, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { instance.cancel(nil); instance.Wait() })
	project, err := instance.Owner.AdmitProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	origin := "http://" + instance.Address()
	remoteIDs := map[string]string{}
	for _, remote := range generated.Remotes() {
		if remote.Module() == "src/lib/environments.remote.ts" {
			remoteIDs[remote.Name()] = remote.ID()
		}
		if remote.Module() == "src/routes/conversation.remote.ts" && remote.Name() == "cleanupConversation" {
			remoteIDs[remote.Name()] = remote.ID()
		}
	}
	for _, name := range []string{"watchEnvironments", "environmentConfig", "cleanupConversation"} {
		if remoteIDs[name] == "" {
			t.Fatalf("%s is not registered", name)
		}
	}
	encoded, err := devalue.Stringify("")
	if err != nil {
		t.Fatal(err)
	}
	for _, referer := range []string{"", origin + "/settings/machines", origin + "/settings/machines/projects/" + project.ID()} {
		t.Run(strings.TrimPrefix(referer, origin), func(t *testing.T) {
			client := &http.Client{Timeout: 5 * time.Second}
			request, err := http.NewRequest(http.MethodGet, origin+"/_app/remote/"+remoteIDs["watchEnvironments"]+"?payload="+base64.RawURLEncoding.EncodeToString([]byte(encoded)), nil)
			if err != nil {
				t.Fatal(err)
			}
			if referer != "" {
				request.Header.Set("Referer", referer)
			}
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = response.Body.Close() }()
			if response.StatusCode != 200 || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
				raw, _ := io.ReadAll(response.Body)
				t.Fatalf("Machines stream: status=%d body=%s", response.StatusCode, raw)
			}
			reader := bufio.NewReader(response.Body)
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			var frame struct {
				Type   string `json:"type"`
				Result string `json:"result"`
				Error  any    `json:"error"`
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &frame); err != nil {
				t.Fatalf("frame=%s: %v", line, err)
			}
			if frame.Type != "result" {
				t.Fatalf("Machines stream returned %s: %+v", frame.Type, frame.Error)
			}
			value, err := devalue.Parse(frame.Result, nil)
			if err != nil {
				t.Fatal(err)
			}
			snapshot, ok := value.(*devalue.Object)
			if !ok {
				t.Fatalf("snapshot=%T", value)
			}
			configPath, _ := snapshot.Get("config_path")
			if configPath != filepath.Join(instance.dir, "environments.json") {
				t.Fatalf("config path=%v", configPath)
			}
			if !strings.Contains(frame.Result, project.Path()) {
				t.Fatalf("snapshot omitted admitted project %s", project.ID())
			}
			_ = response.Body.Close()

			request, err = http.NewRequest(http.MethodGet, origin+"/_app/remote/"+remoteIDs["environmentConfig"], nil)
			if err != nil {
				t.Fatal(err)
			}
			if referer != "" {
				request.Header.Set("Referer", referer)
			}
			response, err = client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = response.Body.Close() }()
			raw, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(raw, &envelope); err != nil {
				t.Fatalf("config response=%s: %v", raw, err)
			}
			if response.StatusCode != 200 || envelope.Type != "result" {
				t.Fatalf("config: status=%d response=%s", response.StatusCode, raw)
			}

			// Machines sends an explicit project identity when ending selected
			// sessions. It must reach that manager outside a project page too.
			payload, err := devalue.Stringify(map[string]any{"project": project.ID(), "conversation": "missing-session"})
			if err != nil {
				t.Fatal(err)
			}
			body, err := json.Marshal(map[string]any{"payload": base64.RawURLEncoding.EncodeToString([]byte(payload))})
			if err != nil {
				t.Fatal(err)
			}
			request, err = http.NewRequest(http.MethodPost, origin+"/_app/remote/"+remoteIDs["cleanupConversation"], strings.NewReader(string(body)))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Origin", origin)
			if referer != "" {
				request.Header.Set("Referer", referer)
			}
			response, err = client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = response.Body.Close() }()
			raw, err = io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != 200 || !strings.Contains(string(raw), "no remote environment to release") {
				t.Fatalf("cleanup did not reach its named project manager: status=%d body=%s", response.StatusCode, raw)
			}
		})
	}
}
