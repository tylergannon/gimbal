package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCloseIsIdempotentForAnUnknownSession covers acceptance item 4: Close
// on an id the adapter never registered (no session, no connection) returns
// nil both times, and needs no daemon: the adapter's session map does not
// contain the id, so Close returns before it ever touches the connection.
func TestCloseIsIdempotentForAnUnknownSession(t *testing.T) {
	ad := New().(*adapter)
	if err := ad.Close(context.Background(), "unknown-thread"); err != nil {
		t.Fatalf("first Close = %v, want nil", err)
	}
	if err := ad.Close(context.Background(), "unknown-thread"); err != nil {
		t.Fatalf("second Close = %v, want nil", err)
	}
}

// TestCloseOnADeadConnectionRetainsOwnership: when the shared
// connection's reader has already failed and archive cannot be delivered,
// Close reports the failure and retains the session and routing ownership.
// A later cleanup attempt must still reach the exact native thread.
func TestCloseOnADeadConnectionRetainsOwnership(t *testing.T) {
	ad := New().(*adapter)
	conn := &connection{threads: make(map[string]chan rpcMessage), readDone: make(chan struct{})}
	conn.registerThread("thread-1")
	close(conn.readDone) // the reader has failed: conn.dead() is true
	ad.sharedConn = conn
	ad.sessions["thread-1"] = &session{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ad.Close(ctx, "thread-1"); err == nil {
		t.Fatal("Close = nil, want an error: the archive could not run on a cancelled cleanup context")
	}
	if n := len(ad.sessions); n != 1 {
		t.Fatalf("sessions after failed Close = %d, want retained ownership", n)
	}
	conn.threadsMu.Lock()
	n := len(conn.threads)
	conn.threadsMu.Unlock()
	if n != 1 {
		t.Fatalf("connection threads after failed Close = %d, want 1", n)
	}
}

// TestCloseNeverStartsAStoppedDaemon: Close reaches the daemon through the
// redial path, and that path can start a daemon that is not running. Close
// must not: a stopped daemon holds nothing for the thread, and starting one
// from scope cleanup violates the shared-daemon contract. A fake `codex` on
// PATH reports the daemon stopped and records every invocation; Close must
// return nil, drop local state, and never run `daemon start`.
func TestCloseNeverStartsAStoppedDaemon(t *testing.T) {
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	script := "#!/bin/sh\necho \"$@\" >> " + calls + "\n" +
		"case \"$*\" in *'daemon version'*) echo '{\"status\":\"stopped\",\"socketPath\":\"" + filepath.Join(dir, "none.sock") + "\"}';; esac\n"
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ad := New().(*adapter)
	conn := &connection{threads: make(map[string]chan rpcMessage), readDone: make(chan struct{})}
	conn.registerThread("thread-1")
	close(conn.readDone)
	ad.sharedConn = conn
	ad.sessions["thread-1"] = &session{}

	if err := ad.Close(context.Background(), "thread-1"); err != nil {
		t.Fatalf("Close = %v, want nil for confirmed stopped daemon", err)
	}
	if n := len(ad.sessions); n != 0 {
		t.Fatalf("sessions after Close = %d, want zero", n)
	}
	recorded, _ := os.ReadFile(calls)
	if !strings.Contains(string(recorded), "daemon version") {
		t.Fatalf("Close never asked the daemon's status; fake codex saw:\n%s", recorded)
	}
	if strings.Contains(string(recorded), "daemon start") {
		t.Fatalf("Close started the daemon; fake codex saw:\n%s", recorded)
	}
}

func TestDetachNeverStartsAStoppedDaemon(t *testing.T) {
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	script := "#!/bin/sh\necho \"$@\" >> " + calls + "\n" +
		"case \"$*\" in *'daemon version'*) echo '{\"status\":\"stopped\",\"socketPath\":\"" + filepath.Join(dir, "none.sock") + "\"}';; esac\n"
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ad := New().(*adapter)
	conn := &connection{threads: make(map[string]chan rpcMessage), readDone: make(chan struct{})}
	conn.registerThread("thread-1")
	close(conn.readDone)
	ad.sharedConn = conn
	ad.sessions["thread-1"] = &session{}

	if err := ad.DetachSession(context.Background(), "thread-1"); err != nil {
		t.Fatalf("DetachSession = %v, want nil: a stopped daemon has no subscription to release", err)
	}
	if n := len(ad.sessions); n != 0 {
		t.Fatalf("sessions after DetachSession = %d, want 0", n)
	}
	recorded, _ := os.ReadFile(calls)
	if strings.Contains(string(recorded), "daemon start") {
		t.Fatalf("DetachSession started the daemon; fake codex saw:\n%s", recorded)
	}
}

func TestResumeSessionRejectsBlankNativeIDWithoutConnecting(t *testing.T) {
	ad := New().(*adapter)
	if err := ad.ResumeSession(t.Context(), "", "gpt-5.6-luna", "low", t.TempDir()); err == nil {
		t.Fatal("ResumeSession with blank id = nil, want an error")
	}
}

// TestSteerWithNoTurnRunningIsDropped: a steer on a thread with no turn
// running has nothing to land in. The adapter reports it dropped, false and
// no error, without a call to the daemon: there is no active turn to route
// it to, so no connection is needed. A steer on an unknown session is the
// error it always was. The other dropped path, turn/steer failing because
// the turn ended while the steer was on its way, needs a live daemon; the
// Session-level outcome of an adapter reporting that is covered with the
// fake adapter in the root package.
func TestSteerWithNoTurnRunningIsDropped(t *testing.T) {
	ad := New().(*adapter)
	ad.sessions["thread-1"] = &session{}
	landed, err := ad.Steer(context.Background(), "thread-1", "change course")
	if err != nil {
		t.Fatalf("Steer = %v, want nil: a dropped steer is not an error", err)
	}
	if landed {
		t.Fatal("Steer landed with no turn running")
	}
	if _, err := ad.Steer(context.Background(), "unknown-thread", "change course"); err == nil {
		t.Fatal("Steer on an unknown session = nil, want an error")
	}
}
