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

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/live"
	"github.com/tylergannon/gimbal/internal/observation"
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
