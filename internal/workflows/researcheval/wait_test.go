package researcheval

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimbal/internal/observation"
)

func TestWaitResearchFollowsChildAndCancelsByIdentity(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(map[bool]string{false: "completion", true: "deadline"}[deadline], func(t *testing.T) {
			project := t.TempDir()
			state := filepath.Join(project, "snapshot.json")
			writeSnapshot := func(status string) {
				t.Helper()
				body, _ := json.Marshal(observation.RunSnapshot{Run: observation.RunRow{ID: "child", Status: status}})
				if err := os.WriteFile(state, append(body, '\n'), 0600); err != nil {
					t.Error(err)
				}
			}
			status := observation.StatusCompleted
			if deadline {
				status = observation.StatusRunning
			}
			writeSnapshot(status)
			installWatch(t, state)
			var mu sync.Mutex
			cancelled := ""
			socketDir, err := os.MkdirTemp("/tmp", "eval-wait-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
			socket := filepath.Join(socketDir, "s")
			listener, err := net.Listen("unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					Run string `json:"run"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					http.Error(w, err.Error(), 400)
					return
				}
				mu.Lock()
				cancelled = request.Run
				mu.Unlock()
				if r.URL.Path != "/control/cancel" || r.Header.Get("X-Gimbal-Project") != project {
					http.Error(w, "incorrect target", 400)
					return
				}
				writeSnapshot(observation.StatusFailed)
				w.WriteHeader(200)
			})}
			go func() { _ = server.Serve(listener) }()
			t.Cleanup(func() { _ = server.Close() })
			controls := filepath.Join(project, ".gimbal", "control")
			if err := os.MkdirAll(controls, 0700); err != nil {
				t.Fatal(err)
			}
			body, _ := json.Marshal(map[string]any{"pid": 1, "socket": socket})
			if err := os.WriteFile(filepath.Join(controls, "host.json"), body, 0600); err != nil {
				t.Fatal(err)
			}
			minutes := 1
			if deadline {
				minutes = 0
			}
			snap, err := waitResearch(workflowContext(t), project, "child", minutes)
			mu.Lock()
			gotCancelled := cancelled
			mu.Unlock()
			if deadline {
				if err == nil || !strings.Contains(err.Error(), "trial deadline") || gotCancelled != "child" || snap.Run.Status != observation.StatusFailed {
					t.Fatalf("deadline: %+v %v target=%q", snap.Run, err, gotCancelled)
				}
			} else if err != nil || snap.Run.Status != observation.StatusCompleted || gotCancelled != "" {
				t.Fatalf("completion: %+v %v target=%q", snap.Run, err, gotCancelled)
			}
		})
	}
}

func TestWaitResearchReportsUnavailableCancellation(t *testing.T) {
	project := t.TempDir()
	state := filepath.Join(project, "snapshot.json")
	body, _ := json.Marshal(observation.RunSnapshot{Run: observation.RunRow{ID: "child", Status: observation.StatusRunning}})
	if err := os.WriteFile(state, append(body, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	installWatch(t, state)
	snap, err := waitResearch(workflowContext(t), project, "child", 0)
	if err == nil || !strings.Contains(err.Error(), "cancel research child") || snap.Run.Status != observation.StatusRunning {
		t.Fatalf("unavailable cancellation: %+v %v", snap.Run, err)
	}
}

func installWatch(t *testing.T, state string) {
	t.Helper()
	bin := t.TempDir()
	command := "#!/bin/sh\nexec cat \"$EVAL_SNAPSHOT\"\n"
	if err := os.WriteFile(filepath.Join(bin, "gimbal"), []byte(command), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVAL_SNAPSHOT", state)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}
