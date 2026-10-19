package environments

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
	"github.com/tylergannon/gimbal/internal/taskdaemon"
)

func TestSuccessfulHTTPMutationReconcilesLostStdoutWithoutReplay(t *testing.T) {
	for _, kind := range []string{"prepare", "clone"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			daemon := taskdaemon.New(t.TempDir())
			handler := daemon.Handler()
			var admissions, stdoutReads atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/tasks/identified-process" {
					admissions.Add(1)
				}
				if r.URL.Path == "/tasks/bytes" && stdoutReads.Add(1) == 1 {
					http.Error(w, "temporary artifact outage", http.StatusServiceUnavailable)
					return
				}
				handler.ServeHTTP(w, r)
			}))
			defer server.Close()
			s := testService(t)
			dir := repository(t)
			destination := filepath.Join(t.TempDir(), "clone")
			target := Target{Machine: Machine{ID: "remote", Name: "Remote", Kind: KindAddress, Address: server.URL, Executable: os.Args[0]}, Directory: dir}
			run := func() (string, error) {
				if kind == "prepare" {
					p, e := s.Prepare(ctx, target, Checkout{Branch: "retained-artifact", Base: "main"}, "same-operation")
					return p.Directory, e
				}
				d, e := s.Clone(ctx, target, dir, destination, "same-operation")
				return d.Path, e
			}
			_, e := run()
			var uncertain *backend.DeliveryUncertain
			if !errors.As(e, &uncertain) {
				t.Fatalf("missing artifact was cached as helper failure: %v", e)
			}
			journal := filepath.Join(s.options.InstanceDir, "environment-operations", kind+"-same-operation.json")
			raw, e := os.ReadFile(journal)
			if e != nil {
				t.Fatal(e)
			}
			var pending operation
			if e = json.Unmarshal(raw, &pending); e != nil {
				t.Fatal(e)
			}
			if pending.Error != "" || !pending.Attempted || !pending.CleanupPending {
				t.Fatalf("artifact uncertainty lost original ownership: %+v", pending)
			}
			// Reconstruct the desktop service to establish that recovery uses the durable
			// request, not an in-memory result or another execution of git.
			if e = s.Close(); e != nil {
				t.Fatal(e)
			}
			s, e = New(ctx, Options{InstanceDir: s.options.InstanceDir})
			if e != nil {
				t.Fatal(e)
			}
			defer func() { _ = s.Close() }()
			path, e := run()
			if e != nil {
				t.Fatalf("same operation could not re-read retained stdout: %v", e)
			}
			d, e := inspectLocal(ctx, path, false)
			if e != nil || !d.Git {
				t.Fatalf("successful mutation was lost: %+v %v", d, e)
			}
			if admissions.Load() != 1 || stdoutReads.Load() != 2 {
				t.Fatalf("recovery replayed native work: admissions=%d artifact reads=%d", admissions.Load(), stdoutReads.Load())
			}
		})
	}
}
