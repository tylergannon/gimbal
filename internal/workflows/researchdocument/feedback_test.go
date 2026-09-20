package researchdocument

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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
	parent := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestFeedbackLauncherParent$")
	parent.Env = append(os.Environ(), "GIMBLE_FEEDBACK_TEST_ROOT="+root)
	parent.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = parent.Process.Kill(); _ = parent.Wait() })
	var output []byte
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		output, _ = os.ReadFile(filepath.Join(root, "started"))
		if len(output) != 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(output) == 0 {
		t.Fatal("launcher waited for sidecar")
	}
	feedback := strings.TrimSpace(string(output))
	if data, err := os.ReadFile(feedback); err != nil || !strings.Contains(string(data), "starting") {
		t.Fatalf("initial feedback %q: %v", data, err)
	}
	// Simulate the host cleaning up the originating execution's process group.
	// The sidecar must still be able to publish its result afterward.
	if err := syscall.Kill(-parent.Process.Pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = parent.Wait()
	if err := os.WriteFile(release, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(feedback)
		if err == nil && strings.Contains(string(data), "incomplete (exit 7)") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("sidecar failure did not reach feedback")
}

func TestFeedbackLauncherParent(t *testing.T) {
	root := os.Getenv("GIMBLE_FEEDBACK_TEST_ROOT")
	if root == "" {
		t.Skip("subprocess helper")
	}
	launch := exec.CommandContext(t.Context(), "zsh", "-c", launchIndexFeedback, "zsh", filepath.Join(root, "feedback space"), filepath.Join(root, "child.sh"), filepath.Join(root, "release"))
	launch.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	output, err := launch.CombinedOutput()
	if err != nil {
		t.Fatalf("launch: %v: %s", err, output)
	}
	if err := os.WriteFile(filepath.Join(root, "started"), output, 0o644); err != nil {
		t.Fatal(err)
	}
	// The test parent kills this process group while the sidecar is blocked.
	time.Sleep(time.Minute)
}
