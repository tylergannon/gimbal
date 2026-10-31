package web

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/devalue/v5"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/observation"
	generated "github.com/tylergannon/gimbal/internal/skgo"
	"github.com/tylergannon/gimbal/web/runview"
)

// The workspace reads its inventory from watchRuns, not the route's SSR
// markup. Exercise that same generated HTTP stream, including later updates.
func watchTestRuns(t *testing.T, instance *Instance, project string) func(func(runview.RunsData) bool) runview.RunsData {
	t.Helper()
	id := ""
	for _, remote := range generated.Remotes() {
		if remote.Module() == "src/lib/runs.remote.ts" && remote.Name() == "watchRuns" {
			id = remote.ID()
		}
	}
	if id == "" {
		t.Fatal("watchRuns remote not registered")
	}
	encoded, err := devalue.Stringify(project)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+instance.Address()+"/_app/remote/"+id+"?payload="+base64.RawURLEncoding.EncodeToString([]byte(encoded)), nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("watchRuns: status=%d content-type=%s", response.StatusCode, response.Header.Get("Content-Type"))
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	return func(accept func(runview.RunsData) bool) runview.RunsData {
		t.Helper()
		timer := time.AfterFunc(20*time.Second, cancel)
		defer timer.Stop()
		var data runview.RunsData
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var frame struct {
				Type   string `json:"type"`
				Result string `json:"result"`
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &frame); err != nil {
				t.Fatal(err)
			}
			if frame.Type != "result" {
				t.Fatalf("watchRuns frame: %s", line)
			}
			data, err = generated.ParseRunsData(frame.Result)
			if err != nil {
				t.Fatal(err)
			}
			if accept(data) {
				return data
			}
		}
		t.Fatalf("watchRuns ended before expected inventory: last=%+v error=%v", data, scanner.Err())
		return data
	}
}

func inventoryTestProject(t *testing.T) (*Instance, *host.Project) {
	t.Helper()
	instance, err := NewInstance(t.Context(), filepath.Join(t.TempDir(), "instance"), nil, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { instance.cancel(nil); instance.Wait() })
	project, err := instance.Owner.AdmitProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return instance, project
}

func TestRunInventoryStreamsEveryRunAndPendingInteractive(t *testing.T) {
	instance, owner := inventoryTestProject(t)
	project, registry := owner.Dir(), owner.Registry()
	running := openTestRun(t, registry, project, "run-running", "plan-trip")
	for _, record := range []json.RawMessage{
		json.RawMessage(`{"seq":1,"time":"2026-09-18T12:00:00Z","scope":"","event":{"kind":"run_started","name":"plan-trip"}}`),
		json.RawMessage(`{"seq":2,"time":"2026-09-18T12:00:01Z","scope":"","event":{"kind":"scope_began","name":"plan-trip","loop":false}}`),
		json.RawMessage(`{"seq":3,"time":"2026-09-18T12:00:05Z","scope":"research.1","session":"researcher.1","event":{"kind":"session_created","name":"researcher","adapter":"fixture","model":"gpt-5.6-luna","effort":"low","workdir":"/work"}}`),
		json.RawMessage(`{"seq":4,"time":"2026-09-18T12:00:06Z","scope":"research.1","session":"researcher.1","turn":"researcher.1/turn.1","event":{"kind":"turn_started","prompt":"Compare cabins using the traveler's latest preferences.","output_type":"gimbal.Text"}}`),
		json.RawMessage(`{"seq":5,"time":"2026-09-18T12:00:10Z","scope":"research.1/lodging.1","session":"researcher.1","event":{"kind":"interactive_changed","id":"question-1","name":"preferences","address":"research.1/lodging.1/preferences","ordinal":1,"consumed":["initial"],"status":"waiting","waiting":true,"transcript":{"messages":[{"role":"user","message":"Help me choose a cabin."},{"role":"assistant","message":"Would you trade reliable Wi-Fi for a secluded cabin?"}],"ended_by":""},"error":""}}`),
		json.RawMessage(`{"seq":6,"time":"2026-09-18T12:00:20Z","scope":"research.1","session":"researcher.1","turn":"researcher.1/turn.1","event":{"kind":"turn_ended","result":"\"done\"","error":"","usage":[{"model":"gpt-5.6-luna","cost":0,"tokens":{"input":1000000,"output":100000,"reasoning":0,"cache":{"read":0,"write":0}}}],"duration":14000000000,"interrupted":false}}`),
	} {
		if err := running.Lifecycle(record); err != nil {
			t.Fatalf("fold running record %s: %v", record, err)
		}
	}

	ended := openTestRun(t, registry, project, "run-ended", "release")
	for _, record := range []json.RawMessage{
		json.RawMessage(`{"seq":1,"time":"2026-09-15T11:00:00Z","scope":"","event":{"kind":"run_started","name":"release"}}`),
		json.RawMessage(`{"seq":2,"time":"2026-09-15T11:07:30Z","scope":"","event":{"kind":"run_ended","name":"release","error":""}}`),
	} {
		if err := ended.Lifecycle(record); err != nil {
			t.Fatalf("fold ended record %s: %v", record, err)
		}
	}

	defer func() { _ = running.Close(); _ = ended.Close() }()
	next := watchTestRuns(t, instance, owner.ID())
	data := next(func(data runview.RunsData) bool { return len(data.Items) == 2 })
	if len(data.Attention) != 1 || data.Attention[0].Run != "run-running" || data.Attention[0].QuestionID != "question-1" || data.Attention[0].Status != observation.InteractiveStatusPending {
		t.Fatalf("pending interaction missing: %+v", data.Attention)
	}
	live, completed := data.Items[0], data.Items[1]
	if live.Run.ID != "run-running" || live.Run.Name != "plan-trip" || live.Run.Status != observation.StatusRunning || !strings.Contains(live.Summary, "1 question waiting") || live.Cost != "$0.32" || live.SessionCount != 1 {
		t.Fatalf("running inventory: %+v", live)
	}
	if completed.Run.ID != "run-ended" || completed.Run.Name != "release" || completed.Run.Status != observation.StatusCompleted || completed.Summary != "Completed" || completed.Elapsed != "7m 30s" {
		t.Fatalf("completed inventory: %+v", completed)
	}
}

func TestRunInventoryStreamsAnEmptyProject(t *testing.T) {
	instance, project := inventoryTestProject(t)
	next := watchTestRuns(t, instance, project.ID())
	data := next(func(runview.RunsData) bool { return true })
	if data.Items == nil || len(data.Items) != 0 || data.Attention == nil || len(data.Attention) != 0 || data.Now <= 0 {
		t.Fatalf("empty inventory: %+v", data)
	}
}

func TestRunInventoryRefreshesAForeignRunFromDisk(t *testing.T) {
	instance, project := inventoryTestProject(t)
	next := watchTestRuns(t, instance, project.ID())
	next(func(data runview.RunsData) bool { return len(data.Items) == 0 })
	foreign := openTestRun(t, observation.NewRegistry(project.Dir()), project.Dir(), "foreign-review", "review")
	defer func() { _ = foreign.Close() }()
	if err := foreign.Lifecycle(json.RawMessage(`{"seq":1,"time":"2026-09-18T12:00:00Z","scope":"","event":{"kind":"run_started","name":"review"}}`)); err != nil {
		t.Fatal(err)
	}
	first := next(func(data runview.RunsData) bool { return len(data.Items) == 1 })
	if first.Items[0].Run.ID != "foreign-review" || first.Items[0].Run.Status != observation.StatusRunning {
		t.Fatalf("foreign run before completion: %+v", first)
	}
	if err := foreign.Lifecycle(json.RawMessage(`{"seq":2,"time":"2026-09-18T12:00:20Z","scope":"","event":{"kind":"run_ended","name":"review","error":""}}`)); err != nil {
		t.Fatal(err)
	}
	if err := foreign.Close(); err != nil {
		t.Fatal(err)
	}
	second := next(func(data runview.RunsData) bool {
		return len(data.Items) == 1 && data.Items[0].Run.Status != observation.StatusRunning
	})
	if second.Items[0].Run.ID != "foreign-review" || second.Items[0].Run.Status != observation.StatusCompleted || second.Items[0].Summary != "Completed" || second.Items[0].Elapsed != "20s" {
		t.Fatalf("foreign run after completion: %+v", second)
	}
}

func openTestRun(t *testing.T, registry *observation.Registry, project, id, name string) *observation.Store {
	t.Helper()
	dir := filepath.Join(project, "runs", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := observation.Open(registry, id, name, dir)
	if err != nil {
		t.Fatal(err)
	}
	return store
}
