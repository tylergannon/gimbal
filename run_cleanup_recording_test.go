package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/live"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/internal/runstate"
	"github.com/tylergannon/gimbal/runstore"
	"github.com/tylergannon/gimbal/runstore/directory"
)

func TestRecordingFailureDoesNotRetainStoppedResources(t *testing.T) {
	for _, remote := range []bool{false, true} {
		for _, retry := range []bool{false, true} {
			for _, boundary := range []string{"intent", "outcome"} {
				name := "session"
				if remote {
					name = "remote-scope"
				}
				if retry {
					name += "-retry"
				}
				t.Run(name+"-"+boundary, func(t *testing.T) {
					project := t.TempDir()
					id := runstore.NewRunID()
					factory := directory.Store{Root: filepath.Join(project, "runs")}
					injected := errors.New("recording unavailable")
					var fail atomic.Bool
					store := faultStore{Store: factory, beforeAppend: func(facts []runstore.Fact) error {
						if fail.Load() {
							for _, fact := range facts {
								if fact.Kind == boundary {
									return injected
								}
							}
						}
						return nil
					}}
					var stops atomic.Int32
					stop := func() error {
						if stops.Add(1) == 1 && retry {
							return errors.New("native stop failed")
						}
						return nil
					}
					adapter := &fake{answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
						return "ok", nil
					}, closeErr: func(string) error { return stop() }}
					var owner *run
					ctx := live.WithHook(WithRunStore(Project(t.Context(), project), store), func(_ string, c live.Controller) func() {
						owner = c.(*run)
						return func() {}
					})
					if remote {
						server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
							if request.URL.Path != "/tasks/cleanup" {
								t.Errorf("unexpected dispatch: %s", request.URL.Path)
							}
							_ = json.NewEncoder(w).Encode(backend.Reply{Error: errString(stop())})
						}))
						defer server.Close()
						ctx = effects.WithEndpoint(ctx, server.URL)
					}
					err := Run(ctx, string(id), name, bind(adapter, "test", "agent"), func(ctx context.Context) error {
						if !remote {
							if _, err := NewSession(ctx, "agent", project).Generate[Text](ctx, "work"); err != nil {
								return err
							}
						}
						fail.Store(true)
						return nil
					})
					if owner == nil {
						t.Fatal("run controller missing")
					}
					if !errors.Is(err, injected) || owner.CleanupPending() != retry {
						t.Fatalf("err=%v pending=%v retry=%v", err, owner.CleanupPending(), retry)
					}
					if retry {
						if _, err := factory.Open(t.Context(), id); !errors.Is(err, runstore.ErrBusy) {
							t.Fatalf("failed native cleanup lost ownership: %v", err)
						}
						if err := owner.retryCleanup(); !errors.Is(err, injected) {
							t.Fatalf("retry lost recording failure: %v", err)
						}
					}
					wantStops := int32(1)
					if retry {
						wantStops++
					}
					if owner.CleanupPending() || stops.Load() != wantStops {
						t.Fatalf("pending=%v stops=%d", owner.CleanupPending(), stops.Load())
					}
					reopened, err := factory.Open(t.Context(), id)
					if err != nil {
						t.Fatalf("stopped run still owns writer: %v", err)
					}
					_ = reopened.Close()
				})
			}
		}
	}
}

func TestProjectionMarkerFailureReleasesStoreWriter(t *testing.T) {
	project := t.TempDir()
	id := runstore.NewRunID()
	factory := directory.Store{Root: filepath.Join(project, "runs")}
	store := faultStore{Store: factory, afterCreate: func() error {
		return os.Mkdir(filepath.Join(project, "runs", string(id), "recording-format"), 0700)
	}}
	registry := observation.NewRegistry(project)
	ctx := WithRunStore(observation.WithRegistry(Project(t.Context(), project), registry), store)
	if err := Run(ctx, string(id), "marker-failure", nil, func(context.Context) error {
		t.Error("body executed after failed initialization")
		return nil
	}); err == nil {
		t.Fatal("marker write failure hidden")
	}
	if _, ok := registry.Live(string(id)); ok {
		t.Fatal("failed initialization remained live")
	}
	reopened, err := factory.Open(t.Context(), id)
	if err != nil {
		t.Fatalf("failed initialization leaked writer: %v", err)
	}
	_ = reopened.Close()
}

func TestLateServiceResultRetainsWriterUntilExplicitCleanup(t *testing.T) {
	project := t.TempDir()
	id := runstore.NewRunID()
	resultReady := make(chan struct{})
	var resultOnce sync.Once
	release := func() { resultOnce.Do(func() { close(resultReady) }) }
	defer release()
	var submissions atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/tasks/identified-process":
			submissions.Add(1)
			_ = json.NewEncoder(w).Encode(backend.Reply{})
		case "/tasks/cleanup":
			_ = json.NewEncoder(w).Encode(backend.Reply{})
		case "/tasks/result":
			<-resultReady
			_ = json.NewEncoder(w).Encode(backend.Result{Done: true, Result: json.RawMessage(`{"ExitCode":0,"Interrupted":true}`)})
		case "/tasks/bytes":
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected dispatch: %s", request.URL.Path)
			http.NotFound(w, request)
		}
	}))
	defer func() { release(); server.Close() }()
	liveRuns := live.NewRuns()
	registry := observation.NewRegistry(project)
	ctx := live.WithHook(observation.WithRegistry(Project(t.Context(), project), registry), liveRuns.Hook)
	ctx = effects.WithEndpoint(WithCleanupTimeout(ctx, 50*time.Millisecond), server.URL)
	var service *ownedService
	err := Run(ctx, string(id), "late-service", nil, func(ctx context.Context) error {
		if err := Service(ctx, "server", project, "serve"); err != nil {
			return err
		}
		s, _ := current(ctx)
		service = s.services[0]
		return nil
	})
	if err == nil || service == nil {
		t.Fatalf("expected retained service cleanup: %v", err)
	}
	factory := directory.Store{Root: filepath.Join(project, "runs")}
	if _, err := factory.Open(t.Context(), id); !errors.Is(err, runstore.ErrBusy) {
		t.Fatalf("pending result lost writer: %v", err)
	}
	release()
	<-service.done
	controller, err := liveRuns.InProgress(string(id))
	if err != nil || !controller.CleanupPending() {
		t.Fatalf("status query abandoned cleanup owner: %v", err)
	}
	if err := controller.CancelScope("", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := liveRuns.InProgress(string(id)); err == nil {
		t.Fatal("released controller remained live")
	}
	reopened, err := factory.Open(t.Context(), id)
	if err != nil {
		t.Fatalf("late service completion leaked writer: %v", err)
	}
	defer func() { _ = reopened.Close() }()
	summary, err := runstate.Inspect(t.Context(), reopened)
	if err != nil || summary.Cleanup[summary.Header.Root].Status != "released" || submissions.Load() != 1 {
		t.Fatalf("cleanup=%v submissions=%d err=%v", summary.Cleanup, submissions.Load(), err)
	}
	snapshot, err := registry.Snapshot(string(id))
	if err != nil || !snapshot.Run.CleanupStatus.Present || snapshot.Run.CleanupStatus.Value != "released" {
		t.Fatalf("cleanup projection was not released: %+v %v", snapshot.Run, err)
	}
}
