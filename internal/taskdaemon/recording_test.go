package taskdaemon

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
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
		if backend.Identified(r.URL.Path[len("/tasks/"):]) {
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
		_, e := gimbal.NewSession(ctx, "agent", project).Generate[gimbal.Text](ctx, "reply")
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	r, e := (directory.Store{Root: filepath.Join(project, "runs")}).Open(t.Context(), id)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
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
	if len(accepted) != 4 || len(bindings) != 4 {
		t.Fatalf("accepted=%d bindings=%d", len(accepted), len(bindings))
	}
	for id, identity := range accepted {
		if saved, ok := bindings[id]; !ok || saved != identity {
			t.Fatalf("unrecorded or changed admission: %+v saved %+v", identity, saved)
		}
	}
}
