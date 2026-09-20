package researchdocument

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFeedbackLaunchReturnsWhileChildRunsAndRecordsFailure(t *testing.T) {
	root := t.TempDir()
	release := filepath.Join(root, "release")
	child := filepath.Join(root, "child.sh")
	// A blocked child proves the launcher returned independently; an explicit
	// release keeps this test independent of machine speed.
	if err := os.WriteFile(child, []byte("#!/bin/zsh\nwhile [ ! -f \"$1\" ]; do sleep 0.02; done\nexit 7\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.WriteFile(release, nil, 0o644) })
	cmd := exec.CommandContext(t.Context(), "zsh", "-c", launchIndexFeedback, "zsh", filepath.Join(root, "feedback space"), child, release)
	done := make(chan error, 1)
	var output []byte
	go func() { var err error; output, err = cmd.Output(); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("launcher waited for sidecar")
	}
	feedback := strings.TrimSpace(string(output))
	if data, err := os.ReadFile(feedback); err != nil || !strings.Contains(string(data), "starting") {
		t.Fatalf("initial feedback %q: %v", data, err)
	}
	if err := os.WriteFile(release, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(feedback)
		if err == nil && strings.Contains(string(data), "incomplete (exit 7)") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("sidecar failure did not reach feedback")
}
