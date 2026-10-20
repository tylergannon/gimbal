package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/conversation"
	"github.com/tylergannon/gimbal/web"
	"golang.org/x/sys/unix"
)

func TestZoomConnectRecordSavesConversationForLaterRuntime(t *testing.T) {
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "id: 1\nevent: transcript\ndata: {\"seq\":1,\"ts\":5000,\"kind\":\"transcript\",\"participant\":{\"id\":\"p\",\"name\":\"Ana\"},\"data\":{\"text\":\"hello team\",\"final\":true}}\n\n")
		_, _ = fmt.Fprint(w, "event: end\ndata: {}\n\n")
	}))
	defer server.Close()
	project := t.TempDir()
	var stdout, stderr bytes.Buffer
	getenv := func(name string) string {
		if name == "ZOOM_CONNECT_API_KEY" {
			return "secret-key"
		}
		return ""
	}
	args := []string{"zoom-connect", "record", "--work-dir", project, "--url", server.URL, "--call", "call1"}
	if err := run(args, &stdout, &stderr, getenv); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer secret-key" {
		t.Fatalf("authorization = %q", auth)
	}
	if strings.Contains(stdout.String(), "secret-key") {
		t.Fatalf("output leaked the key: %s", stdout.String())
	}
	state := filepath.Join(project, ".gimbal")
	manager, err := conversation.New(context.Background(), state, func(string) (gimbal.HarnessAdapter, error) { return nil, nil }, "", state, project, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	items := manager.List()
	if len(items) != 1 || items[0].Title != "Zoom call "+strings.TrimPrefix(server.URL, "http://")+"/call1" || len(items[0].Messages) != 1 || !strings.HasPrefix(items[0].Messages[0].Text, "Ana: hello team\n(") || !strings.Contains(items[0].Messages[0].Text, "#1 transcript ts=5000") || items[0].Messages[0].Created != 5000 {
		t.Fatalf("saved conversations = %+v", items)
	}
	// Once a workflow backs the conversation, recording into it is refused.
	file := filepath.Join(state, "conversations", items[0].ID+".json")
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, bytes.Replace(raw, []byte(`"workflow_run": ""`), []byte(`"workflow_run": "run-1"`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	manager.Close()
	err = run(append(args, "--conversation", items[0].ID), &stdout, &stderr, getenv)
	if !errors.Is(err, conversation.ErrWorkflowBacked) {
		t.Fatalf("err = %v, want ErrWorkflowBacked", err)
	}
}

func TestZoomConnectRecordRefusesProjectOwnedByAnotherInstance(t *testing.T) {
	project := t.TempDir()
	state := filepath.Join(project, ".gimbal")
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	owner, err := os.OpenFile(filepath.Join(state, "owner.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = owner.Close() }()
	if err := unix.Flock(int(owner.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err = run([]string{"zoom-connect", "record", "--work-dir", project, "--url", "http://127.0.0.1:1", "--call", "call1"}, &stdout, &stderr, func(string) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "owned by a gimbal instance whose control socket did not answer") {
		t.Fatalf("err = %v", err)
	}
}

// zoomStream serves two transcript events and the end frame, honoring ?from=.
func zoomStream(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		from, _ := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
		w.Header().Set("Content-Type", "text/event-stream")
		for seq, text := range []string{"first words", "second words"} {
			if int64(seq+1) > from {
				_, _ = fmt.Fprintf(w, "id: %d\nevent: transcript\ndata: {\"seq\":%d,\"ts\":%d,\"recv\":%d,\"kind\":\"transcript\",\"participant\":{\"id\":\"p\",\"name\":\"Ana\"},\"data\":{\"text\":%q,\"final\":true,\"zoom_timestamp\":1760550000123}}\n\n", seq+1, seq+1, 1000*(seq+1), 1000*(seq+1)+1, text)
			}
		}
		_, _ = fmt.Fprint(w, "event: end\ndata: {}\n\n")
	}))
}

func TestZoomConnectRecordThroughRunningHostSurvivesHostRestart(t *testing.T) {
	server := zoomStream(t)
	defer server.Close()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	startHost := func() (*web.Instance, func()) {
		ctx, cancel := context.WithCancel(context.Background())
		instance, err := web.NewInstance(ctx, filepath.Join(base, ".gimbal"), []string{base}, web.WithNoWeb())
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		return instance, func() { cancel(); instance.Wait() }
	}
	instance, stop := startHost()
	defer func() { stop() }()
	args := []string{"zoom-connect", "record", "--work-dir", base, "--url", server.URL, "--call", "call1"}
	var stdout, stderr bytes.Buffer
	if err := run(args, &stdout, &stderr, func(string) string { return "" }); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "through the running gimbal instance") {
		t.Fatalf("did not record through the host: %s", stdout.String())
	}
	// The host still owns the project, and its own manager holds the recording.
	project, err := instance.Owner.Project(base)
	if err != nil {
		t.Fatal(err)
	}
	items := project.Conversations().List()
	if len(items) != 1 || len(items[0].Messages) != 2 || !strings.HasPrefix(items[0].Messages[1].Text, "Ana: second words\n(") || !strings.Contains(items[0].Messages[1].Text, "zoom_timestamp=1760550000123") {
		t.Fatalf("host conversations = %+v", items)
	}
	// Recording the same call again resumes the same conversation without duplicates.
	if err := run(args, &stdout, &stderr, func(string) string { return "" }); err != nil {
		t.Fatal(err)
	}
	if again := project.Conversations().List(); len(again) != 1 || len(again[0].Messages) != 2 {
		t.Fatalf("after second run: %+v", again)
	}

	stop()
	reopened, stopReopened := startHost()
	stop = stopReopened
	project, err = reopened.Owner.Project(base)
	if err != nil {
		t.Fatal(err)
	}
	items = project.Conversations().List()
	if len(items) != 1 || len(items[0].Messages) != 2 || items[0].Title != "Zoom call "+strings.TrimPrefix(server.URL, "http://")+"/call1" {
		t.Fatalf("after host restart: %+v", items)
	}
	if err := run(append(args, "--conversation", "missing"), &stdout, &stderr, func(string) string { return "" }); !errors.Is(err, conversation.ErrNotFound) {
		t.Fatalf("missing conversation: %v", err)
	}
}
