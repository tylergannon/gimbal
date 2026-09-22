package web

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimble/internal/conversation"
	"github.com/tylergannon/gimble/internal/observation"
)

func TestRuntimeListenerOptionsConflict(t *testing.T) {
	_, err := NewRuntime(t.Context(), t.TempDir(), WithPort(0), WithNoWeb())
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("conflicting listener options: %v", err)
	}
	if _, err := NewRuntime(t.Context(), t.TempDir(), WithPort(65536)); err == nil {
		t.Fatal("invalid port was accepted")
	}
	if _, err := NewRuntime(t.Context(), t.TempDir(), WithUDS("  ")); err == nil {
		t.Fatal("blank UDS path was accepted")
	}
}

func TestRuntimeServesWebApplicationOverUDSAndCleansUp(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	socketDir, err := os.MkdirTemp("/tmp", "gimble-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "gimble.sock")
	runtime, err := NewRuntime(ctx, t.TempDir(), WithUDS(socket))
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}}
	defer client.CloseIdleConnections()
	response, err := client.Get("http://gimble/")
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read response: %v; close response: %v", readErr, closeErr)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "Runs") {
		t.Fatalf("GET /: status %d, body %q", response.StatusCode, body)
	}
	cancel()
	<-runtime.done
	if _, err := os.Stat(socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket still exists after shutdown: %v", err)
	}
}

func TestRuntimeUsesSelectedArbitraryPortAndShutsDown(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	runtime, err := NewRuntime(ctx, t.TempDir(), WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Get("http://" + runtime.address + "/")
	if err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /: status %d", response.StatusCode)
	}
	cancel()
	<-runtime.done
	if conn, err := net.Dial("tcp", runtime.address); err == nil {
		_ = conn.Close()
		t.Fatal("TCP listener remained open after shutdown")
	}
}

func TestRuntimeRunsWithoutWeb(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	runtime, err := NewRuntime(ctx, t.TempDir(), WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Run(ctx, "headless", nil, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	cancel()
	<-runtime.done
}

func TestConversationWorkflowStartsInThisRuntimeAndReportsLaunchFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	startedBody := make(chan struct{})
	finishBody := make(chan struct{})
	wantWorktree := filepath.Join(t.TempDir(), "conversation-worktree")
	reviewEntry := func(ctx context.Context, runtime *Runtime, worktree string, _ conversation.LaunchRequest) error {
		if worktree != wantWorktree {
			return errors.New("review received the wrong worktree")
		}
		return runtime.Run(ctx, conversation.WorkflowReview, nil, func(context.Context) error {
			close(startedBody)
			<-finishBody
			return nil
		})
	}
	implementEntry := func(context.Context, *Runtime, string, conversation.LaunchRequest) error {
		return errors.New("implementation inputs were rejected")
	}
	runtime, err := NewRuntime(ctx, t.TempDir(), WithNoWeb(), WithConversationWorkflows(reviewEntry, implementEntry))
	if err != nil {
		t.Fatal(err)
	}

	launched, err := runtime.launchConversationWorkflow(wantWorktree, conversation.LaunchRequest{
		Workflow: conversation.WorkflowReview, Goal: "Inspect it",
	})
	if err != nil {
		t.Fatal(err)
	}
	<-startedBody
	if !strings.HasSuffix(launched.ID, ".review") {
		t.Fatalf("started run id = %q", launched.ID)
	}
	if _, live := runtime.registry.Live(launched.ID); !live {
		t.Fatal("conversation run is not live in the server's observation registry")
	}
	close(finishBody)
	if err := <-launched.Done; err != nil {
		t.Fatal(err)
	}
	snapshot, err := runtime.registry.Snapshot(launched.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Run.Status != observation.StatusCompleted {
		t.Fatalf("terminal run status = %q", snapshot.Run.Status)
	}

	failed, err := runtime.launchConversationWorkflow(wantWorktree, conversation.LaunchRequest{
		Workflow: conversation.WorkflowImplement, OutcomesFile: "outcomes.json",
	})
	if err == nil || !strings.Contains(err.Error(), "inputs were rejected") {
		t.Fatalf("failed implementation launch = (%+v, %v)", failed, err)
	}
}
