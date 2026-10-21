package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/live"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/runstore"
	"github.com/tylergannon/gimbal/runstore/directory"
)

func TestRecordingFailureDoesNotRetainStoppedResources(t *testing.T) {
	for _, remote := range []bool{false, true} {
		for _, stopFailure := range []bool{false, true} {
			for _, boundary := range []string{"lifecycle"} {
				name := "session"
				if remote {
					name = "remote-scope"
				}
				if stopFailure {
					name += "-stop-error"
				}
				t.Run(name+"-"+boundary, func(t *testing.T) {
					project := t.TempDir()
					id := runstore.NewRunID()
					factory := directory.Store{Root: filepath.Join(project, "runs")}
					injected := errors.New("recording unavailable")
					var fail atomic.Bool
					var stops atomic.Int32
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
					stop := func() error {
						if stops.Add(1) == 1 && stopFailure {
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
						_ = owner.eventResult("", "", "", ResourceReady{Name: "recording-fault"})
						return nil
					})
					if owner == nil {
						t.Fatal("run controller missing")
					}
					// The recording failure is diagnostic; only the native stop's own
					// failure, when there is one, reaches the verdict.
					if errors.Is(err, injected) || (err != nil) != stopFailure || stops.Load() != 1 {
						t.Fatalf("recording failure blocked stop or changed the verdict: err=%v stops=%d", err, stops.Load())
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

// A remote endpoint may acknowledge stop before publishing its final output.
// That cannot keep this workflow's writer, logs, or result waiter alive forever.
func TestMissingRemoteServiceResultReleasesLocalHandles(t *testing.T) {
	project := t.TempDir()
	id := runstore.NewRunID()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/tasks/identified-process", "/tasks/cleanup":
			_ = json.NewEncoder(w).Encode(backend.Reply{})
		case "/tasks/result":
			_ = json.NewEncoder(w).Encode(backend.Result{Done: false})
		case "/tasks/bytes":
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()
	runs := live.NewRuns()
	ctx := live.WithHook(Project(t.Context(), project), runs.Hook)
	ctx = effects.WithEndpoint(ctx, server.URL)
	var service *ownedService
	err := Run(ctx, string(id), "missing-service-result", nil, func(ctx context.Context) error {
		if err := Service(ctx, "server", project, "serve"); err != nil {
			return err
		}
		scope, _ := current(ctx)
		service = scope.services[0]
		return nil
	})
	if err == nil || service == nil {
		t.Fatalf("missing result not reported: %v", err)
	}
	select {
	case <-service.done:
	case <-time.After(2 * time.Second):
		t.Fatal("local remote-result waiter was not released")
	}
	if _, err := runs.InProgress(string(id)); err == nil {
		t.Fatal("terminal controller retained")
	}
	writer, err := (directory.Store{Root: filepath.Join(project, "runs")}).Open(t.Context(), id)
	if err != nil {
		t.Fatalf("writer retained after unavailable result: %v", err)
	}
	defer func() {
		if err := writer.Close(); err != nil {
			t.Error(err)
		}
	}()
	records, err := writer.Read(t.Context(), 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.Kind == "cleanup-state" {
			t.Fatal("shutdown created a cleanup ledger")
		}
	}
}

func TestCanceledAgentDiagnosticDoesNotRetryOrFailRecording(t *testing.T) {
	project := t.TempDir()
	var attempts atomic.Int32
	store := faultStore{Store: directory.Store{Root: filepath.Join(project, "runs")}, beforeAppend: func(facts []runstore.Fact) error {
		for _, fact := range facts {
			if fact.Kind == "agent" {
				attempts.Add(1)
				return context.Canceled
			}
		}
		return nil
	}}
	ctx := WithRunStore(Project(t.Context(), project), store)
	err := Run(ctx, string(runstore.NewRunID()), "canceled-diagnostic", nil, func(ctx context.Context) error {
		s, _ := current(ctx)
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if err := s.run.sessionEvent(canceled, s.key, "", "", AgentEvent{}); err != nil {
			return err
		}
		return s.run.recordingError()
	})
	if err != nil || attempts.Load() != 1 {
		t.Fatalf("canceled diagnostic: append attempts=%d run error=%v", attempts.Load(), err)
	}
}
