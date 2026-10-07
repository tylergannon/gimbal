package codex

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
)

func TestNotificationBacklogPreservesRPCAndTerminalAnswer(t *testing.T) {
	conn := &connection{threads: make(map[string]chan rpcMessage), pending: make(map[int64]chan rpcMessage), readDone: make(chan struct{})}
	ch := conn.registerThread("thread-1")
	reply := make(chan rpcMessage, 1)
	conn.pending[99] = reply
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 1500 {
			conn.dispatch(rpcMessage{Method: "rawResponse/outputText/delta", Params: json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","delta":"stream"}`)})
		}
		conn.dispatch(rpcMessage{Method: "item/completed", Params: json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"id":"final","type":"agentMessage","phase":"final_answer","text":"native final"}}`)})
		conn.dispatch(rpcMessage{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed"}}`)})
		for range 1500 {
			conn.dispatch(rpcMessage{Method: "rawResponse/outputText/delta", Params: json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","delta":"late display"}`)})
		}
		conn.dispatch(rpcMessage{ID: json.RawMessage(`99`), Result: json.RawMessage(`{}`)})
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("unread thread blocked shared RPC reader")
	}
	select {
	case <-reply:
	default:
		t.Fatal("unrelated RPC response was blocked")
	}
	if conn.droppedNotifications.Load() == 0 {
		t.Fatal("display gap not counted")
	}
	answer, err := readTurn(t.Context(), conn, ch, "thread-1", "turn-1", newProjector("thread-1", "turn-1", "coding", func(gimbal.AgentEvent) error { return nil }))
	if err != nil || answer != "native final" {
		t.Fatalf("native answer = %q, %v", answer, err)
	}
}

func TestControlBacklogRetiresForNativeReconciliation(t *testing.T) {
	conn := &connection{threads: make(map[string]chan rpcMessage), readDone: make(chan struct{})}
	ch := conn.registerThread("thread-1")
	for range cap(ch) + 1 {
		conn.dispatch(rpcMessage{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed"}}`)})
	}
	if !conn.dead() || !isTransportError(conn.exitError()) {
		t.Fatal("control-only overflow was silently discarded")
	}
}
