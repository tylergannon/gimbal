package gimbal

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/tylergannon/gimbal/runstore"
	"github.com/tylergannon/gimbal/runstore/directory"
)

type pausedAdmissionAdapter struct {
	*fake
	operation string
	pause     func()
	forks     atomic.Int32
}

func (a *pausedAdmissionAdapter) CreateSession(ctx context.Context, model, effort, workdir string) (string, error) {
	if a.operation == "session" {
		a.pause()
	}
	return a.fake.CreateSession(ctx, model, effort, workdir)
}

func (a *pausedAdmissionAdapter) Fork(ctx context.Context, native string) (string, error) {
	a.forks.Add(1)
	if a.operation == "fork" {
		a.pause()
	}
	return a.fake.Fork(ctx, native)
}

func TestCancellationDuringSessionAdmissionReleasesEveryNativeSession(t *testing.T) {
	for _, operation := range []string{"session", "fork"} {
		for _, boundary := range []string{"intent", "native"} {
			t.Run(operation+"-"+boundary, func(t *testing.T) {
				project := t.TempDir()
				id := runstore.NewRunID()
				factory := directory.Store{Root: filepath.Join(project, "runs")}
				entered, release := make(chan struct{}), make(chan struct{})
				pause := func() { close(entered); <-release }
				adapter := &pausedAdmissionAdapter{fake: &fake{answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
					return "ok", nil
				}}}
				store := faultStore{Store: factory}
				if boundary == "native" {
					adapter.operation, adapter.pause = operation, pause
				} else {
					store.beforeAppend = func(facts []runstore.Fact) error {
						for _, fact := range facts {
							var intent struct{ Operation string }
							if fact.Kind == "intent" && json.Unmarshal(fact.Payload, &intent) == nil && intent.Operation == operation {
								pause()
							}
						}
						return nil
					}
				}
				ctx, cancel := context.WithCancel(WithRunStore(Project(t.Context(), project), store))
				defer cancel()
				session := make(chan *Session, 1)
				done := make(chan error, 1)
				go func() {
					done <- Run(ctx, string(id), "cancel-admission", bind(adapter, "test", "agent"), func(ctx context.Context) error {
						s := NewSession(ctx, "agent", project)
						session <- s
						if _, err := s.Generate[Text](ctx, "work"); err != nil {
							return err
						}
						if operation == "fork" {
							_, err := s.Fork(ctx, "branch")
							return err
						}
						return nil
					})
				}()
				s := <-session
				<-entered
				cancel()
				_ = s.owner.end() // Teardown wins before admission is allowed to finish.
				close(release)
				if err := <-done; err == nil {
					t.Fatal("cancelled admission succeeded")
				}
				want := 0
				if operation == "fork" {
					want++ // The original session was created before the fork.
				}
				if boundary == "native" {
					want++ // The in-flight allocation must be closed after it returns.
				}
				adapter.mu.Lock()
				closed := append([]string(nil), adapter.closed...)
				made := adapter.made
				adapter.mu.Unlock()
				if len(closed) != want {
					t.Fatalf("closed=%v want=%d", closed, want)
				}
				if operation == "session" && made != want {
					t.Fatalf("created=%d closed=%v", made, closed)
				}
				if operation == "fork" {
					wantForks := int32(0)
					if boundary == "native" {
						wantForks = 1
					}
					if got := adapter.forks.Load(); got != wantForks {
						t.Fatalf("fork calls=%d want=%d", got, wantForks)
					}
				}
				if operation == "fork" && boundary == "native" && (closed[0] != "native-1" || closed[1] != "native-1-fork") {
					t.Fatalf("wrong resources closed: %v", closed)
				}
				reopened, err := factory.Open(t.Context(), id)
				if err != nil {
					t.Fatalf("cancelled run leaked writer: %v", err)
				}
				_ = reopened.Close()
			})
		}
	}
}
