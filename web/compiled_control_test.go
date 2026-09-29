package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/compiledscope"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/observation"
	routes "github.com/tylergannon/gimbal/internal/skgo/links/onzggl3sn52xizlt"
	"github.com/tylergannon/gimbal/workflow"
)

func init() { gimbal.RegisterGraph(workflow.Graph{Name: "hosted-cancellation-test"}) }

func compiledProject(t *testing.T) (*host.Project, *compiledscope.Store) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	store := &compiledscope.Store{Root: t.TempDir()}
	project := t.TempDir()
	ctx, err := host.WithContextStore(ctx, project, store, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	owner := host.New(ctx, t.TempDir())
	p, err := owner.AdmitProject(project)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); owner.Close() })
	return p, store
}

func TestCompiledConsoleCancellationDelivery(t *testing.T) {
	for _, mode := range []string{"accepted-race", "unavailable-retry", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			p, _ := compiledProject(t)
			var root context.Context
			calls := 0
			release := make(chan struct{})
			callback := func(ctx context.Context, cause gimbal.Killed) error {
				if cause.By != "person" {
					t.Errorf("backend cause = %#v", cause)
				}
				calls++
				switch mode {
				case "accepted-race":
					if err := compiledscope.CancelRun(root); err != nil {
						t.Error(err)
					}
					// Backend notification must not race finish ahead of the delivery record.
					if root.Err() != nil {
						t.Error("local stop preceded backend acknowledgement")
					}
				case "unavailable-retry":
					if calls == 1 {
						return errors.New("controller unavailable")
					}
				case "timeout":
					<-release // deliberately ignores deadline; the host must still stop locally
				}
				return nil
			}
			var finish func(error) error
			var err error
			root, finish, err = p.OpenCompiledRun(t.Context(), "hosted-cancellation-test", nil, "", t.TempDir(), host.CompiledControls{CancelRun: callback, DeliveryTimeout: 20 * time.Millisecond})
			if err != nil {
				t.Fatal(err)
			}
			id := startedRunID(t, p.Path())
			active := make(chan struct{})
			drained := make(chan struct{})
			go func() { close(active); <-root.Done(); close(drained) }()
			<-active
			started := time.Now()
			accepted, err := routes.Skgo_cancelRun(p.Context(), routes.CancelRun{Run: id})
			if mode == "accepted-race" && (err != nil || !accepted.Accepted) {
				t.Fatalf("command: %+v %v", accepted, err)
			}
			if mode != "accepted-race" && err == nil {
				t.Fatal("delivery failure was acknowledged as success")
			}
			if mode == "timeout" {
				close(release)
				if time.Since(started) > time.Second {
					t.Fatal("deadline did not bound local stop")
				}
			}
			select {
			case <-drained:
			case <-time.After(time.Second):
				t.Fatal("local active work did not drain")
			}
			var cause gimbal.Killed
			if !errors.As(context.Cause(root), &cause) || cause.By != "person" {
				t.Fatalf("cause = %v", context.Cause(root))
			}
			pending, err := p.Registry().Snapshot(id)
			if err != nil {
				t.Fatal(err)
			}
			if !pending.Run.Cancellation.Present || pending.Run.Cancellation.Value.Cleanup != "pending" {
				t.Fatalf("cleanup before finish = %+v", pending.Run)
			}
			if mode == "unavailable-retry" {
				result, err := routes.Skgo_cancelRun(p.Context(), routes.CancelRun{Run: id})
				if err != nil || !result.Accepted || calls != 2 {
					t.Fatalf("delivery retry: %+v %v calls=%d", result, err, calls)
				}
			}
			if err := compiledscope.CancelRun(root); err != nil {
				t.Fatal(err)
			}
			_ = finish(context.Canceled)
			snapshot, err := observation.NewRegistry(p.Dir()).Snapshot(id)
			if err != nil {
				t.Fatal(err)
			}
			cancellation := snapshot.Run.Cancellation.Value
			if snapshot.Run.Status != "cancelled" || !snapshot.Run.Cancellation.Present || cancellation.By != "person" || cancellation.Cleanup != "completed" {
				t.Fatalf("retained cancellation = %+v %+v", snapshot.Run, cancellation)
			}
			want := "accepted"
			if mode == "timeout" {
				want = "unconfirmed"
			}
			if got := cancellation.Deliveries[len(cancellation.Deliveries)-1].Status; got != want {
				t.Fatalf("delivery = %s want %s", got, want)
			}
			if mode == "unavailable-retry" && (len(cancellation.Deliveries) != 2 || cancellation.Deliveries[0].Status != "unconfirmed") {
				t.Fatalf("history = %+v", cancellation.Deliveries)
			}
			raw, err := os.ReadFile(filepath.Join(p.Dir(), "runs", id, "run.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			scanner := bufio.NewScanner(strings.NewReader(string(raw)))
			kills := 0
			for scanner.Scan() {
				var record gimbal.LifecycleRecord
				if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
					t.Fatal(err)
				}
				if _, ok := record.Event.(gimbal.Killed); ok {
					kills++
				}
			}
			if kills != 1 {
				t.Fatalf("root kill count = %d", kills)
			}
		})
	}
}

func TestCompiledHostedEntryRequirements(t *testing.T) {
	p, _ := compiledProject(t)
	callback := host.CompiledControls{CancelRun: func(context.Context, gimbal.Killed) error { return nil }}
	if _, _, err := p.OpenCompiledRun(t.Context(), "missing-compiled-graph", nil, "", t.TempDir(), callback); err == nil || !strings.Contains(err.Error(), "registered graph") {
		t.Fatalf("missing graph: %v", err)
	}
	if _, _, err := p.OpenCompiledRun(t.Context(), "hosted-cancellation-test", nil, "", t.TempDir(), host.CompiledControls{}); err == nil || !strings.Contains(err.Error(), "callback") {
		t.Fatalf("missing callback: %v", err)
	}
	callback.DeliveryTimeout = -time.Second
	if _, _, err := p.OpenCompiledRun(t.Context(), "hosted-cancellation-test", nil, "", t.TempDir(), callback); err == nil {
		t.Fatal("negative timeout accepted")
	}
}

func TestCompiledBackendCancellationHasNoOperatorDelivery(t *testing.T) {
	p, _ := compiledProject(t)
	root, finish, err := p.OpenCompiledRun(t.Context(), "hosted-cancellation-test", nil, "", t.TempDir(), host.CompiledControls{CancelRun: func(context.Context, gimbal.Killed) error {
		t.Error("backend notification invoked console delivery")
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	id := startedRunID(t, p.Path())
	if err := compiledscope.CancelRun(root); err != nil {
		t.Fatal(err)
	}
	_ = finish(context.Canceled)
	snapshot, err := p.Registry().Snapshot(id)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Run.Status != "cancelled" || snapshot.Run.Cancellation.Value.By != "" || len(snapshot.Run.Cancellation.Value.Deliveries) != 0 {
		t.Fatalf("backend cancellation = %+v", snapshot.Run)
	}
}

func TestCompiledContextArtifactSurvivesHostRestart(t *testing.T) {
	project := t.TempDir()
	store := &compiledscope.Store{Root: t.TempDir()}
	text := strings.Repeat("retained complete value\n", 4096)
	raw, _ := json.Marshal(text)
	initial, err := store.Extend(t.Context(), "", compiledscope.Entry{Key: "large", Value: raw})
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := host.WithContextStore(t.Context(), project, store, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	owner := host.New(ctx, t.TempDir())
	p, err := owner.AdmitProject(project)
	if err != nil {
		t.Fatal(err)
	}
	_, finish, err := p.OpenCompiledRun(t.Context(), "hosted-cancellation-test", nil, initial, t.TempDir(), host.CompiledControls{CancelRun: func(context.Context, gimbal.Killed) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	id := startedRunID(t, project)
	if err := finish(nil); err != nil {
		t.Fatal(err)
	}
	snapshot, err := p.Registry().Snapshot(id)
	if err != nil {
		t.Fatal(err)
	}
	artifact := snapshot.Scopes[""].Values["large"].Artifact
	if artifact == nil {
		t.Fatal("large initial value has no retained artifact")
	}
	owner.Close()
	// Fresh owner and fresh host cache: neither live worker materializations nor
	// a run-directory symlink can be required to resolve the recorded reference.
	cache := t.TempDir()
	ctx, err = host.WithContextStore(t.Context(), project, store, cache)
	if err != nil {
		t.Fatal(err)
	}
	restarted := host.New(ctx, t.TempDir())
	defer restarted.Close()
	p, err = restarted.AdmitProject(project)
	if err != nil {
		t.Fatal(err)
	}
	object := strings.TrimPrefix(artifact.File, "context/objects/")
	request := httptest.NewRequest(http.MethodGet, "/api/runs/"+id+"/artifacts/"+object, nil).WithContext(p.RequestContext(t.Context()))
	response := httptest.NewRecorder()
	observation.Routes(http.NotFoundHandler()).ServeHTTP(response, request)
	if response.Code != 200 || response.Body.String() != text {
		t.Fatalf("retained artifact status=%d bytes=%d", response.Code, response.Body.Len())
	}
	if _, err := os.Stat(filepath.Join(cache, object)); err != nil {
		t.Fatalf("host cache: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(p.Dir(), "runs", id, "context")); !os.IsNotExist(err) {
		t.Fatalf("unexpected run-local context link: %v", err)
	}
	if err := os.Remove(filepath.Join(store.Root, "objects", object)); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	observation.Routes(http.NotFoundHandler()).ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing retained object = %d; cache must not hide loss", response.Code)
	}
}

func TestCompiledFinishWaitsForDeliveryObservation(t *testing.T) {
	p, _ := compiledProject(t)
	deliveryEntered := make(chan struct{})
	acknowledge := make(chan struct{})
	root, finish, err := p.OpenCompiledRun(t.Context(), "hosted-cancellation-test", nil, "", t.TempDir(), host.CompiledControls{CancelRun: func(context.Context, gimbal.Killed) error { close(deliveryEntered); <-acknowledge; return nil }})
	if err != nil {
		t.Fatal(err)
	}
	id := startedRunID(t, p.Path())
	commandDone := make(chan error, 1)
	go func() { _, err := routes.Skgo_cancelRun(p.Context(), routes.CancelRun{Run: id}); commandDone <- err }()
	<-deliveryEntered
	if err := compiledscope.CancelRun(root); err != nil {
		t.Fatal(err)
	}
	finishDone := make(chan error, 1)
	go func() { finishDone <- finish(context.Canceled) }()
	// Completion is deliberately requested while the backend response is pending.
	// Unlock delivery; finish must retain both that outcome and operator cause.
	close(acknowledge)
	if err := <-commandDone; err != nil {
		t.Fatal(err)
	}
	<-finishDone
	snapshot, err := observation.NewRegistry(p.Dir()).Snapshot(id)
	if err != nil {
		t.Fatal(err)
	}
	cancellation := snapshot.Run.Cancellation.Value
	if !snapshot.Run.Cancellation.Present || cancellation.By != "person" || len(cancellation.Deliveries) != 1 || cancellation.Deliveries[0].Status != "accepted" {
		t.Fatalf("racing finish lost observations: %+v", cancellation)
	}
}

func TestCompiledStoreConfigurationIsRequiredAndUnique(t *testing.T) {
	project := t.TempDir()
	owner := host.New(t.Context(), t.TempDir())
	defer owner.Close()
	p, err := owner.AdmitProject(project)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.OpenCompiledRun(t.Context(), "hosted-cancellation-test", nil, "", t.TempDir(), host.CompiledControls{CancelRun: func(context.Context, gimbal.Killed) error { return nil }}); err == nil || !strings.Contains(err.Error(), "context-store") {
		t.Fatalf("missing store: %v", err)
	}
	store := &compiledscope.Store{Root: t.TempDir()}
	ctx, err := host.WithContextStore(t.Context(), project, store, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.WithContextStore(ctx, project, store, t.TempDir()); err == nil {
		t.Fatal("duplicate context store accepted")
	}
}
