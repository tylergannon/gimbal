package codex

import (
	"encoding/json"
	"strings"
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
	gap := false
	answer, err := readTurn(t.Context(), conn, ch, "thread-1", "turn-1", newProjector("thread-1", "turn-1", "coding", func(event gimbal.AgentEvent) error {
		if event.Type == "session.connection" && strings.Contains(string(event.Data), "discarded") {
			gap = true
		}
		return nil
	}))
	if !gap {
		t.Fatal("display gap was not exposed in the session record")
	}
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

func TestNotificationBacklogKeepsToolAndResponseCompletion(t *testing.T) {
	conn := &connection{threads: make(map[string]chan rpcMessage), readDone: make(chan struct{})}
	ch := conn.registerThread("thread-1")
	var events []gimbal.AgentEvent
	p := newProjector("thread-1", "turn-1", "coding", func(event gimbal.AgentEvent) error { events = append(events, event); return nil })
	params := func(tail string) json.RawMessage {
		return json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1",` + tail + `}`)
	}
	if err := projectNotification(rpcMessage{Method: "item/started", Params: params(`"item":{"id":"tool","type":"commandExecution","command":"echo done","status":"inProgress"}`)}, p); err != nil {
		t.Fatal(err)
	}
	conn.dispatch(rpcMessage{Method: "item/completed", Params: params(`"item":{"id":"tool","type":"commandExecution","command":"echo done","status":"completed","aggregatedOutput":"done","exitCode":0}`)})
	conn.dispatch(rpcMessage{Method: "rawResponse/completed", Params: params(`"responseId":"response-1","usage":{"input_tokens":2,"output_tokens":3}`)})
	for range 600 {
		conn.dispatch(rpcMessage{Method: "rawResponse/outputText/delta", Params: params(`"delta":"stream"`)})
	}
	conn.dispatch(rpcMessage{Method: "item/completed", Params: params(`"item":{"id":"answer","type":"agentMessage","phase":"final_answer","text":"done"}`)})
	conn.dispatch(rpcMessage{Method: "turn/completed", Params: params(`"turn":{"id":"turn-1","status":"completed"}`)})
	answer, err := readTurn(t.Context(), conn, ch, "thread-1", "turn-1", p)
	if err != nil || answer != "done" || p.pendingTools != 0 || p.stepOpen {
		t.Fatalf("answer=%q err=%v pending=%d stepOpen=%v", answer, err, p.pendingTools, p.stepOpen)
	}
	gap, ended, success := false, false, false
	for _, event := range events {
		gap = gap || event.Type == "session.connection"
		ended = ended || event.Type == "session.step.ended"
		success = success || event.Type == "session.tool.success"
	}
	if !gap || !ended || !success {
		t.Fatalf("gap=%v step ended=%v tool success=%v", gap, ended, success)
	}
}
