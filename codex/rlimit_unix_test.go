//go:build unix

package codex

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// TestConnectRaisesTheDescriptorLimitBeforeStartingTheDaemon covers the
// issue's acceptance item: a fake `codex` on PATH reports the daemon
// stopped and, on `daemon start`, prints its own `ulimit -n` (inherited
// from this test process at the moment of exec). connect(ctx, true) must
// raise this process's own soft RLIMIT_NOFILE before that exec, so the
// value the fake daemon observes is the raised one, not the low ceiling
// the test first sets to make the raise observable.
//
// This follows the fake-codex-on-PATH pattern in
// TestCloseNeverStartsAStoppedDaemon: the fake reports "stopped" once,
// then (once its own "daemon start" has run) "running" with a socketPath
// that has nothing listening on it, so connect's subsequent websocket
// dial fails fast instead of waiting out the 10s "did not come up"
// deadline. The test only needs the recorded ulimit; it does not need
// connect to succeed.
func TestConnectRaisesTheDescriptorLimitBeforeStartingTheDaemon(t *testing.T) {
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	ulimitFile := filepath.Join(dir, "ulimit")
	started := filepath.Join(dir, "started")
	sock := filepath.Join(dir, "none.sock")

	script := "#!/bin/sh\n" +
		"echo \"$@\" >> " + calls + "\n" +
		"case \"$*\" in\n" +
		"*'daemon start'*)\n" +
		"  touch " + started + "\n" +
		"  ulimit -n >> " + ulimitFile + "\n" +
		"  ;;\n" +
		"*'daemon version'*)\n" +
		"  if [ -f " + started + " ]; then\n" +
		"    echo '{\"status\":\"running\",\"socketPath\":\"" + sock + "\"}'\n" +
		"  else\n" +
		"    echo '{\"status\":\"stopped\",\"socketPath\":\"" + sock + "\"}'\n" +
		"  fi\n" +
		"  ;;\n" +
		"esac\n"
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Lower this test process's own soft limit first: a shell this process
	// execs inherits whatever the process's current soft limit is at that
	// moment, so unless the test starts below the daemon's normal ceiling,
	// nothing distinguishes a raised value from the value it always had.
	const lowered = 200
	var original syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &original); err != nil {
		t.Fatalf("Getrlimit: %v", err)
	}
	if original.Max < lowered {
		t.Skipf("hard limit %d is below the test's lowered floor %d", original.Max, lowered)
	}
	t.Cleanup(func() {
		_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &original)
	})
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &syscall.Rlimit{Cur: lowered, Max: original.Max}); err != nil {
		t.Fatalf("Setrlimit(lowered): %v", err)
	}

	// connect fails once it reaches the websocket dial (nothing is
	// listening on sock); that is expected and irrelevant to this test.
	_, _ = connect(context.Background(), true)

	recordedCalls, _ := os.ReadFile(calls)
	if !strings.Contains(string(recordedCalls), "daemon start") {
		t.Fatalf("connect never started the daemon; fake codex saw:\n%s", recordedCalls)
	}

	raw, err := os.ReadFile(ulimitFile)
	if err != nil {
		t.Fatalf("fake codex never recorded ulimit -n: %v", err)
	}
	observed, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("parse recorded ulimit -n %q: %v", raw, err)
	}
	if observed <= lowered {
		t.Fatalf("daemon start saw ulimit -n = %d, want > %d (the pre-raise floor): connect did not raise this process's descriptor limit before exec", observed, lowered)
	}
	t.Logf("daemon start observed ulimit -n = %d after connect raised the pre-exec floor of %d", observed, lowered)
}
