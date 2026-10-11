package web

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"html"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/tylergannon/devalue/v5"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/host"
	generated "github.com/tylergannon/gimbal/internal/skgo"
	"github.com/tylergannon/gimbal/workflow"
)

func TestNativeLaunchFormDecodesIndexedFields(t *testing.T) {
	const name = "native-launch-fixture"
	var received string
	description := gimbal.WorkflowDescription{Name: name, Summary: "Native form", InputSchema: json.RawMessage(`{"type":"object","properties":{"goal":{"type":"string"},"enabled":{"type":"boolean"}},"required":["goal"]}`), Fields: []gimbal.WorkflowField{{Name: "goal", Flag: "goal", Kind: "string"}, {Name: "enabled", Flag: "enabled", Kind: "bool", Optional: true}}, SourcePage: json.RawMessage(`{"name":"native-launch-fixture","entry":"Fixture","module":"test","source":{"file":"fixture.go","line":1},"body":[]}`)}
	err := gimbal.DefaultRegistry.RegisterWorkflow(gimbal.WorkflowDefinition{WorkflowDescription: description, Graph: workflow.Graph{Name: name}, Prepare: func(raw json.RawMessage) (gimbal.Invocation, error) {
		received = string(raw)
		return nil, &gimbal.InputError{Field: "goal", Message: "Prepared indexed input"}
	}})
	if err != nil {
		t.Fatal(err)
	}
	dist, err := fs.Sub(Build, "build")
	if err != nil {
		t.Fatal(err)
	}
	handler, _, err := NewHandler(dist, "", "")
	if err != nil {
		t.Fatal(err)
	}
	id := ""
	for _, remote := range generated.Remotes() {
		if strings.HasSuffix(remote.ID(), "/launchWorkflow") {
			id = remote.ID()
		}
	}
	if id == "" {
		t.Fatal("launch form missing from generated remotes")
	}
	get := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/workflows/"+name+"?v=1", nil)
	getRequest = getRequest.WithContext(host.WithProjects(getRequest.Context(), []host.ProjectChoice{{ID: "fixture", Path: "/unused"}}))
	handler.ServeHTTP(get, getRequest)
	if !strings.Contains(get.Body.String(), `value="/unused" selected`) {
		t.Fatalf("initial project default not selected: %s", get.Body.String())
	}
	action := regexp.MustCompile(`<form[^>]*action="([^"]+)"`).FindStringSubmatch(get.Body.String())
	if len(action) != 2 {
		t.Fatalf("missing visible native form: status=%d body=%s", get.Code, get.Body.String())
	}
	if html.UnescapeString(action[1]) != "?v=1&/remote="+id {
		t.Fatalf("native action=%s", action[1])
	}
	for _, field := range []string{"workflow/" + id, "inputs[0].name/" + id, "inputs[0].value/" + id, "b:inputs[1].include/" + id} {
		if !strings.Contains(get.Body.String(), `name="`+field+`"`) {
			t.Fatalf("visible native form missing %s; rebuild frontend", field)
		}
	}
	values := url.Values{}
	for key, value := range map[string]string{"workflow": name, "project": "/unused", "workdir": "", "inputs[0].name": "goal", "inputs[0].value": "ship", "b:inputs[0].include": "on", "inputs[1].name": "enabled", "inputs[1].value": "false", "b:inputs[1].include": "on"} {
		values.Set(key+"/"+id, value)
	}
	request := httptest.NewRequest(http.MethodPost, "/workflows/"+name+html.UnescapeString(action[1]), strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "text/html")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if received != `{"enabled":false,"goal":"ship"}` {
		t.Fatalf("native form decoded %q, status=%d body=%s", received, response.Code, response.Body.String())
	}
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Prepared indexed input") {
		t.Fatalf("addressable validation status=%d body=%s", response.Code, response.Body.String())
	}
	// This is the unchanged native browser body with a blank required goal.
	received = ""
	values.Set("inputs[0].value/"+id, "")
	request = httptest.NewRequest(http.MethodPost, "/workflows/"+name+html.UnescapeString(action[1]), strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "text/html")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if received != "" || !strings.Contains(response.Body.String(), "nonblank string") {
		t.Fatalf("invalid native form status=%d prepare=%q body=%s", response.Code, received, response.Body.String())
	}
}

func TestSharedLaunchEnhancedEndpointDoesNotRequireProjectReferrer(t *testing.T) {
	instance, err := NewInstance(t.Context(), t.TempDir(), nil, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { instance.cancel(nil); instance.Wait() })
	id := ""
	for _, remote := range generated.Remotes() {
		if remote.Module() == "src/routes/launch.remote.ts" {
			id = remote.ID()
		}
	}
	header, err := devalue.Stringify([]any{map[string]any{"workflow": "missing-workflow", "inputs": []any{}, "roles": []any{}}, map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	body := make([]byte, 7+len(header))
	binary.LittleEndian.PutUint32(body[1:5], uint32(len(header)))
	copy(body[7:], header)
	origin := "http://" + instance.address
	request, err := http.NewRequest(http.MethodPost, origin+"/_app/remote/"+id, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-sveltekit-formdata")
	request.Header.Set("Origin", origin)
	request.Header.Set("Referer", origin+"/workflows/external-first?v=1")
	request.Header.Set("x-sveltekit-pathname", "/workflows/external-first")
	request.Header.Set("x-sveltekit-search", "?v=1")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(raw), "Unknown workflow") {
		t.Fatalf("shared enhanced endpoint: status=%d body=%s", response.StatusCode, raw)
	}
}
