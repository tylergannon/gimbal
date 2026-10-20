package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/runstore"
)

func TestNativeDaemonProcess(t *testing.T) {
	root := os.Getenv("GIMBAL_NATIVE_DAEMON_CHILD")
	if root == "" {
		t.Skip("separate daemon fixture")
	}
	stop, err := serveEnvironmentOptions(context.Background(), os.Getenv("GIMBAL_NATIVE_DAEMON_ADDRESS"), root, nil, root, filepath.Join(root, "executors"))
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	select {}
}

func TestNativeRemoteInteractiveDaemonRestart(t *testing.T) {
	if os.Getenv("GIMBAL_TEST_NATIVE_INTERACTIVE") != "1" {
		t.Skip("set GIMBAL_TEST_NATIVE_INTERACTIVE=1 to run Codex luna")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	root := t.TempDir()
	project := filepath.Join(root, "project")
	if err := os.Mkdir(project, 0700); err != nil {
		t.Fatal(err)
	}
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := reservation.Addr().String()
	_ = reservation.Close()
	endpoint := "http://" + address
	var child *exec.Cmd
	var logFile *os.File
	stop := func() {
		if child != nil {
			_ = child.Process.Kill()
			_ = child.Wait()
			_ = logFile.Close()
			child = nil
		}
	}
	t.Cleanup(stop)
	start := func() {
		t.Helper()
		logFile, err = os.CreateTemp(root, "daemon-*.log")
		if err != nil {
			t.Fatal(err)
		}
		child = exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNativeDaemonProcess$", "-test.v")
		child.Env = append(os.Environ(), "GIMBAL_NATIVE_DAEMON_CHILD="+root, "GIMBAL_NATIVE_DAEMON_ADDRESS="+address)
		child.Stdout, child.Stderr = logFile, logFile
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		for {
			request, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/health", nil)
			response, e := http.DefaultClient.Do(request)
			if e == nil {
				_ = response.Body.Close()
				if response.StatusCode == 200 {
					return
				}
			}
			select {
			case <-ctx.Done():
				stop()
				raw, _ := os.ReadFile(logFile.Name())
				t.Fatalf("daemon startup: %v\n%s", ctx.Err(), raw)
			case <-time.After(50 * time.Millisecond):
			}
		}
	}
	start()
	transport := backend.HTTP{Endpoint: endpoint}
	id := string(runstore.NewRunID())
	config, _ := json.Marshal(map[string]string{"endpoint": endpoint})
	request := backend.Workflow{ID: id, Name: "conversation", Project: project, Workdir: project, Input: json.RawMessage(`{"purpose":"Remember the human token and answer briefly with chat; do not end unless asked."}`), Roles: map[string]string{"agentic-dialogues": "luna"}, TaskRoute: backend.Route{Name: "http", Configuration: config}}
	status, err := transport.StartWorkflow(ctx, request)
	if err != nil || status.State != "running" {
		t.Fatalf("admit=%+v err=%v", status, err)
	}
	snapshot := func() observation.RunSnapshot {
		t.Helper()
		value, err := transport.ReadObservation(ctx, backend.Target{Run: id}, "")
		if err != nil {
			t.Fatal(err)
		}
		var snap observation.RunSnapshot
		if err := json.Unmarshal(value.Snapshot, &snap); err != nil {
			t.Fatal(err)
		}
		return snap
	}
	wait := func(messages int) observation.InteractiveRow {
		t.Helper()
		for {
			snap := snapshot()
			if snap.Run.Status == observation.StatusFailed {
				t.Fatalf("remote run failed: %s", snap.Run.Error)
			}
			for _, row := range snap.Interactions {
				if len(row.Messages) == messages && row.Status == "pending" {
					return row
				}
			}
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-time.After(50 * time.Millisecond):
			}
		}
	}
	send := func(inputID, message string, finish bool) {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"id": inputID, "message": message, "finish": finish})
		if _, err := transport.ControlWorkflow(ctx, backend.Control{Run: id, Action: "interactive", Target: "conversation", Message: string(raw)}); err != nil {
			t.Fatal(err)
		}
	}
	wait(0)
	send("remote-remember", "Remember token amber-willow-642.", false)
	first := wait(2)
	sessions, err := filepath.Glob(filepath.Join(root, "executors", "taskdaemon", "sessions", "*.json"))
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions=%v err=%v", sessions, err)
	}
	var saved struct{ Native string }
	raw, err := os.ReadFile(sessions[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &saved); err != nil || saved.Native == "" {
		t.Fatalf("saved session=%s err=%v", raw, err)
	}
	checkpoint := filepath.Join(root, "executors", "sessions", saved.Native, "native.json")
	before, err := os.ReadFile(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	stop()
	start() // Independent daemon process, same durable state and HTTP route.
	status, err = transport.WorkflowStatus(ctx, backend.Target{Run: id})
	if err != nil || status.ID != id || status.State != observation.StatusRunning {
		t.Fatalf("restored=%+v err=%v", status, err)
	}
	send("remote-recall", "What token did I give you? Echo it exactly.", false)
	second := wait(4)
	if second.Messages[1].Message != first.Messages[1].Message || !strings.Contains(second.Messages[3].Message, "amber-willow-642") {
		t.Fatalf("resumed transcript=%+v", second.Messages)
	}
	after, err := os.ReadFile(checkpoint)
	if err != nil || string(after) != string(before) {
		t.Fatalf("native checkpoint identity changed: before=%s after=%s err=%v", before, after, err)
	}
	send("remote-active", "Run a shell sleep 60 before replying. Keep this native turn active while you wait.", false)
	active := false
	for !active {
		snap := snapshot()
		for _, row := range snap.Interactions {
			active = active || (row.Status == "active" && len(row.Messages) == 5)
		}
		if !active {
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-time.After(50 * time.Millisecond):
			}
		}
	}
	send("remote-steer", "Remember the extra token silver-leaf-208 while staying in this turn.", false)
	send("remote-steer", "Remember the extra token silver-leaf-208 while staying in this turn.", false)
	send("remote-finish", "", true)
	for {
		status, err = transport.WorkflowStatus(ctx, backend.Target{Run: id})
		if err != nil {
			t.Fatal(err)
		}
		if status.State == observation.StatusCompleted {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
	final := snapshot()
	for _, row := range final.Interactions {
		if row.EndedBy != "human" || len(row.Messages) != 6 || !strings.Contains(row.Messages[5].Message, "silver-leaf-208") {
			t.Fatalf("active steer/finish=%+v", row)
		}
	}
	t.Logf("alias=luna run=%s executor session=%s native checkpoint=%s: separate HTTP daemon restart preserved binding/transcript and resumed accepted input; active steering deduplicated and Finish safely interrupted", id, saved.Native, strings.TrimSpace(string(after)))
}
