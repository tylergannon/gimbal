package taskdaemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
)

// Supplied exercise dependency. It observes readiness from this producer's
// environment and records scoped use, but never provisions infrastructure.
type serviceFixture struct {
	mu          sync.Mutex
	url         string
	uses        []backend.ServiceUse
	releases    []backend.Scope
	failRelease bool
}

func (f *serviceFixture) Acquire(ctx context.Context, use backend.ServiceUse) (backend.ServiceBinding, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", f.url, nil)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return backend.ServiceBinding{}, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != 200 {
		return backend.ServiceBinding{}, fmt.Errorf("declared readiness/access failed: HTTP %d", response.StatusCode)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.uses = append(f.uses, use)
	return backend.ServiceBinding{Name: use.Name, URL: f.url}, nil
}
func (f *serviceFixture) Release(_ context.Context, scope backend.Scope) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releases = append(f.releases, scope)
	if f.failRelease {
		return errors.New("release unconfirmed")
	}
	return nil
}

type fetchingHarness struct {
	testHarness
	fetched bool
}

func (h *fetchingHarness) RunTurn(ctx context.Context, _ string, prompt string, _ json.RawMessage, _ func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	_, binding, ok := strings.Cut(prompt, "Runtime service http is ready for this agent at ")
	if !ok {
		return gimbal.TurnResult{}, errors.New("agent binding missing")
	}
	address := strings.TrimSuffix(strings.Fields(binding)[0], ".")
	req, _ := http.NewRequestWithContext(ctx, "GET", address, nil)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return gimbal.TurnResult{}, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != 200 {
		return gimbal.TurnResult{}, errors.New("agent fetch failed")
	}
	h.fetched = true
	return gimbal.TurnResult{Output: []byte(`"fetched"`)}, nil
}

func TestMessagingDeliversServiceToActualAgentAndCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, "GIMBAL_SERVICE_OK") }))
	defer server.Close()
	d, bus, ctx := messagingOwner(t)
	fixture := &serviceFixture{url: server.URL}
	d.Orchestration = fixture
	native := &fetchingHarness{}
	d.adapter = func(string) (gimbal.HarnessAdapter, error) { return native, nil }
	h := &Harness{Name: "fake", Connection: backend.Connection{Messages: bus}}
	ctx = effects.WithOwner(ctx, backend.Owner{Run: "run", Scope: "one", Task: "agent"})
	id, err := h.CreateSession(ctx, "model", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	turnCtx := effects.WithOwner(effects.WithServices(ctx, []string{"http"}), backend.Owner{Run: "run", Scope: "one", Task: "turn"})
	if _, err := h.RunTurn(turnCtx, id, "Fetch the service", nil, nil); err != nil {
		t.Fatal(err)
	}
	if !native.fetched {
		t.Fatal("actual agent did not fetch")
	}
	for _, scope := range []string{"one", "two"} {
		r := effects.Request("process", backend.Owner{Run: "run", Scope: scope, Task: scope + "/command"})
		r.Process = &backend.Process{Owner: r.Owner, Command: "python3", Args: []string{"-c", `import os, urllib.request; print(urllib.request.urlopen(os.environ['GIMBAL_SERVICE_HTTP']).read().decode())`}, Services: []string{"http"}}
		result, err := effects.MessageCall(ctx, r)
		if err != nil || result.Result.Process.ExitCode != 0 || !strings.Contains(string(result.Result.Stdout), "GIMBAL_SERVICE_OK") {
			t.Fatalf("command %s: %+v %v", scope, result.Result, err)
		}
	}
	if err := effects.Cleanup(ctx, "", "run", "one"); err != nil {
		t.Fatal(err)
	}
	if len(fixture.releases) != 1 || fixture.releases[0] != (backend.Scope{Run: "run", Scope: "one"}) {
		t.Fatalf("release attribution: %+v", fixture.releases)
	}
	if _, err := fixture.Acquire(ctx, backend.ServiceUse{Scope: backend.Scope{Run: "run", Scope: "two"}, Name: "http", Kind: "process", Consumer: "command"}); err != nil {
		t.Fatal("unrelated shared use stopped", err)
	}
	if len(fixture.uses) != 4 || fixture.uses[0].Kind != "agent" || fixture.uses[0].Consumer != "agent" || fixture.uses[1].Kind != "process" || fixture.uses[1].Consumer != "one/command" {
		t.Fatalf("actual consumers: %+v", fixture.uses)
	}
	fixture.failRelease = true
	if err := effects.Cleanup(ctx, "", "run", "two"); err == nil {
		t.Fatal("failed release reported cleanup success")
	}
	fixture.failRelease = false
	if err := effects.Cleanup(ctx, "", "run", "two"); err != nil {
		t.Fatal("release could not be retried", err)
	}
}

func TestFailedServiceReadinessPreventsNativeAdmission(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(503) }))
	defer server.Close()
	d, _, ctx := messagingOwner(t)
	d.Orchestration = &serviceFixture{url: server.URL}
	owner := backend.Owner{Run: "run", Scope: "scope", Task: "no-start"}
	r := effects.Request("process", owner)
	r.Process = &backend.Process{Owner: owner, Command: "sh", Args: []string{"-c", "exit 0"}, Services: []string{"http"}}
	if _, err := effects.MessageCall(ctx, r); err == nil || !strings.Contains(err.Error(), "readiness/access failed") {
		t.Fatalf("readiness failure: %v", err)
	}
	if d.tasks[key(owner.Run, owner.Task)] != nil {
		t.Fatal("native process launched despite failed acquisition")
	}
	server.Close()
	r.ID += "new"
	if _, err := effects.MessageCall(ctx, r); err == nil {
		t.Fatal("unreachable service appeared acquired")
	}
	if d.tasks[key(owner.Run, owner.Task)] != nil {
		t.Fatal("native process launched despite no consumer access")
	}
}

type blockedAcquisition struct{ started, ended chan struct{} }

func (f *blockedAcquisition) Acquire(ctx context.Context, _ backend.ServiceUse) (backend.ServiceBinding, error) {
	close(f.started)
	<-ctx.Done()
	close(f.ended)
	return backend.ServiceBinding{}, ctx.Err()
}
func (f *blockedAcquisition) Release(context.Context, backend.Scope) error {
	select {
	case <-f.ended:
		return nil
	default:
		return errors.New("release preceded acquisition cancellation")
	}
}

func TestServicePreflightKeepsControlsAndUnrelatedAdmissionsAvailable(t *testing.T) {
	for _, control := range []string{"interrupt", "cleanup"} {
		t.Run(control, func(t *testing.T) {
			d, _, ctx := messagingOwner(t)
			fixture := &blockedAcquisition{started: make(chan struct{}), ended: make(chan struct{})}
			d.Orchestration = fixture
			owner := backend.Owner{Run: "run", Scope: "blocked", Task: "blocked/command"}
			request := effects.Request("process", owner)
			request.Process = &backend.Process{Owner: owner, Command: "sh", Args: []string{"-c", "exit 0"}, Services: []string{"http"}}
			finished := make(chan error, 1)
			go func() { _, err := effects.MessageCall(ctx, request); finished <- err }()
			<-fixture.started
			unrelated := effects.Request("process", backend.Owner{Run: "other", Task: "unrelated"})
			unrelated.Process = &backend.Process{Owner: unrelated.Owner, Command: "sh", Args: []string{"-c", "exit 0"}}
			bounded, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			if _, err := effects.MessageCall(bounded, unrelated); err != nil {
				t.Fatal("external acquire blocked unrelated native admission", err)
			}
			if control == "interrupt" {
				if err := effects.InterruptMessage(bounded, request); err != nil {
					t.Fatal("external acquire blocked interruption", err)
				}
			} else if err := effects.Cleanup(bounded, "", owner.Run, owner.Scope); err != nil {
				t.Fatal("preflight scope cleanup failed", err)
			}
			select {
			case err := <-finished:
				if err == nil {
					t.Fatal("cancelled acquisition appeared successful")
				}
			case <-bounded.Done():
				t.Fatal("preflight did not settle after control")
			}
			d.mu.Lock()
			native := d.tasks[key(owner.Run, owner.Task)]
			d.mu.Unlock()
			if native != nil {
				t.Fatal("native process started after preflight was cancelled")
			}
		})
	}
}

type resourceDiagnosticFailure struct{ *directMessages }

func (b *resourceDiagnosticFailure) PublishEvent(ctx context.Context, event backend.MessageEvent) error {
	if event.ReadyService != nil {
		return errors.New("resource diagnostic storage unavailable")
	}
	return b.directMessages.PublishEvent(ctx, event)
}

func TestResourceDiagnosticFailurePreservesNativeWorkAndNormalRunFacts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, "GIMBAL_SERVICE_OK") }))
	defer server.Close()
	d, bus, ctx := messagingOwner(t)
	failing := &resourceDiagnosticFailure{directMessages: bus}
	bus.handle = d.Messages(failing)
	d.Orchestration = &serviceFixture{url: server.URL}
	native := &fetchingHarness{}
	d.adapter = func(string) (gimbal.HarnessAdapter, error) { return native, nil }
	connection := backend.Connection{Messages: failing}
	ctx = effects.WithConnection(ctx, connection)
	project := t.TempDir()
	h := &Harness{Name: "fake", Connection: connection}
	err := gimbal.Run(gimbal.Project(ctx, project), "resource-facts", map[gimbal.WorkflowRole]gimbal.ModelBinding{"agent": {Adapter: h, Model: "fake"}}, func(ctx context.Context) error {
		return gimbal.Scope(gimbal.WithServices(ctx, "http"), "scope", func(ctx context.Context) error {
			agent := gimbal.NewSession(ctx, "agent", t.TempDir())
			if _, err := agent.Generate[gimbal.Text](ctx, "fetch"); err != nil {
				return err
			}
			code, stdout, _, err := gimbal.RunCommand(ctx, "consume", t.TempDir(), "python3", "-c", `import os,urllib.request;print(urllib.request.urlopen(os.environ['GIMBAL_SERVICE_HTTP']).read().decode())`)
			if err != nil {
				return err
			}
			if code != 0 || !strings.Contains(stdout, "GIMBAL_SERVICE_OK") {
				return fmt.Errorf("actual command fetch failed: %d %s", code, stdout)
			}
			return nil
		})
	})
	if err != nil || !native.fetched {
		t.Fatalf("diagnostic failure gated successful native execution: %v", err)
	}
	paths, err := filepath.Glob(filepath.Join(project, "runs", "*", "run.jsonl"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("normal run journal absent: %v %v", paths, err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		var record struct {
			Event struct {
				Kind         string `json:"kind"`
				Name         string `json:"name"`
				ConsumerKind string `json:"consumer_kind"`
				Consumer     string `json:"consumer"`
			} `json:"event"`
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		if record.Event.Kind == "resource_ready" {
			if record.Event.Name != "http" || record.Event.Consumer == "" {
				t.Fatalf("bad resource fact: %+v", record.Event)
			}
			kinds[record.Event.ConsumerKind] = true
		}
	}
	if !kinds["agent"] || !kinds["process"] {
		t.Fatalf("normal journal lost resource facts: %v", kinds)
	}
}
