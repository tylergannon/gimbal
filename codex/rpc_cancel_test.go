package codex

import (
	"context"
	"errors"
	"testing"
)

func TestCancelledWriterDoesNotRetireSharedConnection(t *testing.T) {
	ad, _ := newRecoveryDaemon(t, "survive")
	conn, err := ad.conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	conn.writeMu.Lock()
	done := make(chan error, 1)
	go func() { done <- conn.sendContext(ctx, map[string]any{"method": "initialized"}) }()
	cancel()
	conn.writeMu.Unlock()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("write = %v", err)
	}
	if conn.dead() {
		t.Fatal("caller cancellation retired shared connection")
	}
	if _, err := conn.call(t.Context(), "thread/start", map[string]any{"model": "coding"}); err != nil {
		t.Fatalf("unrelated session = %v", err)
	}
}
