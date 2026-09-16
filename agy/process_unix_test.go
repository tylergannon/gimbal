//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package agy

import (
	"context"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/tylergannon/gimble"
)

func TestAdapterReleasesDescendantThatInheritedStdout(t *testing.T) {
	adapter, record := testAdapter(t)
	sessionID, err := adapter.CreateSession(t.Context(), "gemini-test-low", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		result, err := adapter.RunTurn(context.Background(), sessionID, "RESULT_WITH_DESCENDANT", nil, func(gimble.AgentEvent) error { return nil })
		if err == nil && string(result.Output) != `"OK"` {
			err = &unexpectedResult{got: string(result.Output)}
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("terminal result stayed blocked on inherited stdout")
	}
	raw, err := os.ReadFile(record + ".child")
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for processExists(pid) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if processExists(pid) {
		t.Fatalf("descendant process %d survived the terminal result", pid)
	}
}

type unexpectedResult struct{ got string }

func (e *unexpectedResult) Error() string { return "unexpected result " + e.got }

func processExists(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
