package taskdaemon

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"

	"github.com/tylergannon/gimbal/runstore"
)

// A failed read of a completed command's stdout must not be what retires it:
// after the command returns the read error and the scope has cleaned up, the
// real daemon still serves the exact bytes and has not expired the task.
func TestFailedOutputReadLeavesRecoverableOutputAfterScopeCleanup(t *testing.T) {
	d := New(t.TempDir())
	native := d.Handler()
	var failed atomic.Bool
	var artifact backend.Artifact
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks/bytes" {
			native.ServeHTTP(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		var v backend.Artifact
		if err := json.Unmarshal(body, &v); err == nil && v.Name == "stdout" && failed.CompareAndSwap(false, true) {
			artifact = v
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = conn.Close()
			return
		}
		native.ServeHTTP(w, r)
	}))
	defer server.Close()
	ctx := effects.WithEndpoint(gimbal.Project(context.Background(), t.TempDir()), server.URL)
	err := gimbal.Run(ctx, string(runstore.NewRunID()), "failed-output-read", nil, func(ctx context.Context) error {
		_, _, _, err := gimbal.RunCommand(ctx, "command", t.TempDir(), "sh", "-c", "printf payload")
		if err == nil {
			t.Error("injected stdout read failure was not reported")
		}
		return nil
	})
	if err != nil {
		t.Logf("run: %v", err)
	}
	if !failed.Load() {
		t.Fatal("stdout read was never attempted")
	}
	reader, err := (backend.HTTP{Endpoint: server.URL}).ReadBytes(context.Background(), artifact)
	if err != nil {
		t.Fatalf("stdout unreadable after failed read and scope cleanup: %v", err)
	}
	got, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil || string(got) != "payload" {
		t.Fatalf("stdout after cleanup = %q, %v", got, err)
	}
	progress := d.poll(effects.Poll{Run: artifact.Run, ID: artifact.Task})
	if progress.Error == "task payload expired" {
		t.Fatal("task payload expired after one failed read")
	}
}
