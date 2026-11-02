//go:build !windows

package codex

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartupRecheckProcess(t *testing.T) {
	if os.Getenv("GIMBAL_TEST_RECHECK_CHILD") != "1" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := startDaemonLocked(ctx, os.Getenv("GIMBAL_TEST_STARTUP_SOCKET")); err != nil {
		t.Fatal(err)
	}
}

// A fake CLI records starts while real child processes race through the production lock and recheck.
func TestStartupRechecksUnderProcessLock(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
if [ "$3" = version ]; then
  if [ -f "$TEST_STATE/ready" ]; then status=running; else status=stopped; fi
  printf '{"status":"%s","socketPath":"%s"}\n' "$status" "$GIMBAL_TEST_STARTUP_SOCKET"
elif [ "$3" = start ]; then
  printf 'start\n' >> "$TEST_STATE/starts"
  sleep 0.3
  touch "$TEST_STATE/ready"
else
  exit 1
fi
`
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	var children []*exec.Cmd
	for range 2 {
		cmd := exec.Command(os.Args[0], "-test.run=^TestStartupRecheckProcess$")
		cmd.Env = append(os.Environ(), "GIMBAL_TEST_RECHECK_CHILD=1", "GIMBAL_TEST_STARTUP_SOCKET="+filepath.Join(dir, "daemon.sock"), "TEST_STATE="+dir, "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		children = append(children, cmd)
	}
	for _, cmd := range children {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	starts, err := os.ReadFile(filepath.Join(dir, "starts"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(starts), "start\n"); got != 1 {
		t.Fatalf("daemon starts = %d, want 1", got)
	}
}
