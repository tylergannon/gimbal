package taskdaemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/tylergannon/gimbal/backend/messagefiles"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/runstore"
	"github.com/tylergannon/gimbal/runstore/directory"
)

func TestRecordedBindingsMatchAcceptedHTTPExecution(t *testing.T) {
	daemon := New(t.TempDir())
	daemon.adapter = func(string) (gimbal.HarnessAdapter, error) { return &testHarness{}, nil }
	handler := daemon.Handler()
	var mu sync.Mutex
	accepted := map[string]backend.ExecutionIdentity{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if backend.Identified(r.URL.Path[len("/tasks/"):]) && r.URL.Path != "/tasks/identified-close" {
			raw, e := io.ReadAll(r.Body)
			if e != nil {
				t.Error(e)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(raw))
			var v struct{ ExecutionIdentity backend.ExecutionIdentity }
			if e = json.Unmarshal(raw, &v); e != nil {
				t.Error(e)
			}
			mu.Lock()
			accepted[v.ExecutionIdentity.RequestID] = v.ExecutionIdentity
			mu.Unlock()
		}
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	project := t.TempDir()
	id := runstore.NewRunID()
	ctx := effects.WithEndpoint(gimbal.Project(t.Context(), project), server.URL)
	err := gimbal.Run(ctx, string(id), "recorded", map[gimbal.WorkflowRole]gimbal.ModelBinding{"agent": {Adapter: &Harness{Endpoint: server.URL, Name: "test"}, Model: "test"}}, func(ctx context.Context) error {
		if _, out, _, e := gimbal.RunCommand(ctx, "command", project, "printf", "out"); e != nil {
			return e
		} else if out != "out" {
			t.Errorf("output %q", out)
		}
		session := gimbal.NewSession(ctx, "agent", project)
		if _, err := session.Generate[gimbal.Text](ctx, "reply"); err != nil {
			return err
		}
		fork, err := session.Fork(ctx, "fork")
		if err != nil {
			return err
		}
		_, err = fork.Generate[gimbal.Text](ctx, "continue")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	r, e := (directory.Store{Root: filepath.Join(project, "runs")}).Open(t.Context(), id)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = r.Close() }()
	records, e := r.Read(t.Context(), 0, 100)
	if e != nil {
		t.Fatal(e)
	}
	bindings := map[string]backend.ExecutionIdentity{}
	for _, rec := range records {
		if rec.Kind == "binding" {
			var identity backend.ExecutionIdentity
			if e = json.Unmarshal(rec.Payload, &identity); e != nil {
				t.Fatal(e)
			}
			if identity.ExecutorOperation != "" {
				bindings[identity.RequestID] = identity
			}
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(accepted) != 5 || len(bindings) != 5 {
		t.Fatalf("accepted=%d bindings=%d", len(accepted), len(bindings))
	}
	for id, identity := range accepted {
		if saved, ok := bindings[id]; !ok || saved != identity {
			t.Fatalf("unrecorded or changed admission: %+v saved %+v", identity, saved)
		}
	}
}

func TestOldDaemonRejectsWorkflowTurnWithoutFallbackOrPolling(t *testing.T) {
	var starts, polls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tasks/session", "/tasks/turn":
			starts.Add(1)
		case "/tasks/result":
			polls.Add(1)
		case "/tasks/cleanup":
			_ = json.NewEncoder(w).Encode(backend.Reply{})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	project := t.TempDir()
	ctx := effects.WithEndpoint(gimbal.Project(t.Context(), project), server.URL)
	err := gimbal.Run(ctx, string(runstore.NewRunID()), "old-turn", map[gimbal.WorkflowRole]gimbal.ModelBinding{"agent": {Adapter: &Harness{Endpoint: server.URL, Name: "test"}, Model: "test"}}, func(ctx context.Context) error {
		_, err := gimbal.NewSession(ctx, "agent", project).Generate[gimbal.Text](ctx, "reply")
		return err
	})
	var rejected *backend.ContractRejected
	if !errors.As(err, &rejected) || starts.Load() != 0 || polls.Load() != 0 {
		t.Fatalf("unsafe old-daemon turn: %v starts=%d polls=%d", err, starts.Load(), polls.Load())
	}
}

type recordedMessages struct {
	*messagefiles.Store
	handle   backend.MessageHandler
	requests map[string]backend.ExecutionIdentity
	ctx      context.Context
}

func (m *recordedMessages) PublishRequest(_ context.Context, r backend.MessageRequest) error {
	if backend.Identified(r.Kind) && r.Kind != "identified-close" {
		m.requests[r.ID] = r.ExecutionIdentity
	}
	return m.handle(m.ctx, r)
}
func TestRecordedBindingsMatchAcceptedMessagingExecution(t *testing.T) {
	store, err := messagefiles.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bus := &recordedMessages{Store: store, requests: map[string]backend.ExecutionIdentity{}, ctx: t.Context()}
	daemon := New(t.TempDir())
	daemon.adapter = func(string) (gimbal.HarnessAdapter, error) { return &testHarness{}, nil }
	bus.handle = daemon.Messages(bus)
	connection := backend.Connection{Messages: bus}
	project := t.TempDir()
	id := runstore.NewRunID()
	route := backend.Route{Name: "fixture-messages", Configuration: json.RawMessage(`{"worker":"assigned"}`)}
	ctx := effects.WithRoute(effects.WithConnection(gimbal.Project(t.Context(), project), connection), route)
	err = gimbal.Run(ctx, string(id), "messages", map[gimbal.WorkflowRole]gimbal.ModelBinding{"agent": {Adapter: &Harness{Name: "test", Connection: connection}, Model: "test"}}, func(ctx context.Context) error {
		if _, _, _, err := gimbal.RunCommand(ctx, "check", project, "printf", "ok"); err != nil {
			return err
		}
		session := gimbal.NewSession(ctx, "agent", project)
		for range 2 {
			if _, err := session.Generate[gimbal.Text](ctx, "reply"); err != nil {
				return err
			}
		}
		if _, err := session.Fork(ctx, "unsupported-fork"); err == nil || !strings.Contains(err.Error(), "unsupported on messaging") {
			t.Fatalf("messaging fork admitted: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	r, err := (directory.Store{Root: filepath.Join(project, "runs")}).Open(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	records, err := r.Read(t.Context(), 0, 200)
	if err != nil {
		t.Fatal(err)
	}
	bindings := map[string]backend.ExecutionIdentity{}
	intents := map[string]bool{}
	for _, rec := range records {
		switch rec.Kind {
		case "binding":
			var identity backend.ExecutionIdentity
			if err = json.Unmarshal(rec.Payload, &identity); err != nil {
				t.Fatal(err)
			}
			bindings[identity.RequestID] = identity
		case "intent":
			var intent struct {
				Identity backend.ExecutionIdentity `json:"identity"`
				Route    backend.Route             `json:"route"`
			}
			if err = json.Unmarshal(rec.Payload, &intent); err != nil {
				t.Fatal(err)
			}
			if intent.Route.Name != route.Name || string(intent.Route.Configuration) != string(route.Configuration) {
				t.Fatalf("owner route changed: %+v", intent.Route)
			}
			intents[intent.Identity.RequestID] = true
		}
	}
	if len(bus.requests) != 4 {
		t.Fatalf("unexpected admissions: %d", len(bus.requests))
	}
	for id, identity := range bus.requests {
		if saved, ok := bindings[id]; !ok || saved != identity || !intents[id] {
			t.Fatalf("unrecorded messaging admission: %+v saved %+v", identity, saved)
		}
	}
}
