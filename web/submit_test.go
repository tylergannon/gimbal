package web

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/observation"
)

func followServer(t *testing.T, instanceDir, socketName string, handler http.HandlerFunc) func() {
	t.Helper()
	// Unix sockets need a short path on macOS.
	dir, err := os.MkdirTemp("", "follow-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, socketName)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: handler}
	go func() { _ = server.Serve(listener) }()
	discoveryDir := filepath.Join(instanceDir, "control")
	if err := os.MkdirAll(discoveryDir, 0755); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(controlDiscovery{Socket: socket})
	discovery := filepath.Join(discoveryDir, socketName+".json")
	if err := os.WriteFile(discovery, raw, 0600); err != nil {
		t.Fatal(err)
	}
	stop := func() { _ = server.Close(); _ = os.Remove(discovery) }
	t.Cleanup(stop)
	return stop
}

func TestFollowRediscoversCentralWithoutResubmitting(t *testing.T) {
	instanceDir, project := t.TempDir(), t.TempDir()
	project, _ = host.CanonicalProject(project)
	const id = "accepted-run"
	seen := make(chan struct{}, 1)
	var unexpected atomic.Int32
	handler := func(status string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.Header.Get("X-Gimbal-Project") != project {
				unexpected.Add(1)
				http.Error(w, "unexpected request", 400)
				return
			}
			if r.URL.Path == "/control/live" {
				return
			}
			if r.URL.Path != "/api/runs/"+id {
				unexpected.Add(1)
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(observation.RunSnapshot{Run: observation.RunRow{ID: id, Status: status}})
			select {
			case seen <- struct{}{}:
			default:
			}
		}
	}
	stop := followServer(t, instanceDir, "first", handler(observation.StatusRunning))
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		row, err := Follow(ctx, instanceDir, project, id)
		if err == nil && (row.ID != id || row.Status != observation.StatusCompleted) {
			err = errors.New("wrong terminal row")
		}
		result <- err
	}()
	select {
	case <-seen:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	stop()
	// Ensure follow observes downtime before publishing a different socket.
	time.Sleep(450 * time.Millisecond)
	select {
	case err := <-result:
		t.Fatalf("follow exited during restart: %v", err)
	default:
	}
	var restored atomic.Bool
	followServer(t, instanceDir, "second", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/control/live" && !restored.Load() {
			http.NotFound(w, r)
			return
		}
		handler(observation.StatusCompleted)(w, r)
	})
	time.Sleep(450 * time.Millisecond)
	select {
	case err := <-result:
		t.Fatalf("follow exited before restoration: %v", err)
	default:
	}
	restored.Store(true)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if unexpected.Load() != 0 {
		t.Fatalf("unexpected requests: %d", unexpected.Load())
	}
}

func TestFollowReconnectFailureAndCancellation(t *testing.T) {
	instanceDir, project := t.TempDir(), t.TempDir()
	_, err := follow(t.Context(), instanceDir, project, "accepted", 250*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "reconnect") || !strings.Contains(err.Error(), "accepted") {
		t.Fatalf("error=%v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { _, err := Follow(ctx, instanceDir, project, "accepted"); done <- err }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation blocked")
	}
}

func TestFollowPreservesHTTPAndRunFailures(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusForbidden, http.StatusOK} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			instanceDir, project := t.TempDir(), t.TempDir()
			followServer(t, instanceDir, "terminal", func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/control/live" {
					return
				}
				w.WriteHeader(status)
				if status == http.StatusOK {
					_ = json.NewEncoder(w).Encode(observation.RunSnapshot{Run: observation.RunRow{ID: "accepted", Status: observation.StatusFailed, Error: "workflow failed"}})
				}
			})
			row, err := Follow(t.Context(), instanceDir, project, "accepted")
			if status == http.StatusOK {
				if err != nil || row.Status != observation.StatusFailed || row.Error != "workflow failed" {
					t.Fatalf("row=%+v err=%v", row, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), http.StatusText(status)) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
