package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/tylergannon/gimbal"
	"go.temporal.io/sdk/temporal"
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

// TestCloseOnADeadConnectionDropsLocalRoutingState: when the shared
// connection's reader has already failed, Close must still release
// everything the adapter holds locally (the session entry and the thread's
// routing channel on that connection) before it tries to reach the daemon.
// The cleanup context here is already cancelled, so the redial cannot
// succeed: Close must then report the archive it could not do rather than
// return nil, and the local state must be gone regardless.
func TestCloseOnADeadConnectionDropsLocalRoutingState(t *testing.T) {
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
	if n := len(ad.sessions); n != 0 {
		t.Fatalf("sessions after Close = %d, want 0", n)
	}
	conn.threadsMu.Lock()
	n := len(conn.threads)
	conn.threadsMu.Unlock()
	if n != 0 {
		t.Fatalf("connection threads after Close = %d, want 0", n)
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
		t.Fatalf("Close = %v, want nil: a stopped daemon holds nothing to release", err)
	}
	if n := len(ad.sessions); n != 0 {
		t.Fatalf("sessions after Close = %d, want 0", n)
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

// appServer is a scripted Codex app-server on a real WebSocket. It records
// every message the adapter sends and hands each to the test's handler.
type appServer struct {
	t       *testing.T
	writeMu sync.Mutex
	ws      *websocket.Conn

	mu   sync.Mutex
	seen []rpcMessage
}

// startAppServer returns an adapter whose shared connection is the scripted
// server, with the session "thread" registered.
func startAppServer(t *testing.T, handle func(s *appServer, message rpcMessage)) (*adapter, *appServer) {
	t.Helper()
	s := &appServer{t: t}
	accepted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ws, err := websocket.Accept(w, req, nil)
		if err != nil {
			return
		}
		ws.SetReadLimit(readLimit)
		s.ws = ws
		close(accepted)
		defer func() { _ = ws.CloseNow() }()
		for {
			_, data, err := ws.Read(context.Background())
			if err != nil {
				return
			}
			var message rpcMessage
			if err := json.Unmarshal(data, &message); err != nil {
				t.Errorf("adapter sent invalid JSON: %s", data)
				return
			}
			s.mu.Lock()
			s.seen = append(s.seen, message)
			s.mu.Unlock()
			handle(s, message)
		}
	}))
	t.Cleanup(server.Close)
	ws, _, err := websocket.Dial(context.Background(), strings.Replace(server.URL, "http:", "ws:", 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	ws.SetReadLimit(readLimit)
	t.Cleanup(func() { _ = ws.CloseNow() })
	<-accepted
	conn := &connection{ws: ws, threads: make(map[string]chan rpcMessage), pending: make(map[int64]chan rpcMessage), readDone: make(chan struct{})}
	go conn.read()
	ad := New().(*adapter)
	ad.sharedConn = conn
	ad.sessions["thread"] = &session{model: "model"}
	return ad, s
}

func (s *appServer) send(frame string) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := s.ws.Write(context.Background(), websocket.MessageText, []byte(frame)); err != nil {
		s.t.Errorf("app-server write: %v", err)
	}
}

func (s *appServer) reply(message rpcMessage, result string) {
	s.send(fmt.Sprintf(`{"id":%s,"result":%s}`, message.ID, result))
}

func (s *appServer) reject(message rpcMessage, text string) {
	s.send(fmt.Sprintf(`{"id":%s,"error":{"code":-32600,"message":%q}}`, message.ID, text))
}

// notify sends a notification for turn t1 of the thread.
func (s *appServer) notify(method, body string) {
	s.send(fmt.Sprintf(`{"method":%q,"params":{"threadId":"thread","turnId":"t1",%s}}`, method, body))
}

func (s *appServer) completed(status string) {
	s.notify("turn/completed", fmt.Sprintf(`"turn":{"id":"t1","status":%q}`, status))
}

func (s *appServer) sent(method string) []rpcMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	var matched []rpcMessage
	for _, message := range s.seen {
		if message.Method == method {
			matched = append(matched, message)
		}
	}
	return matched
}

func turnCtx(t *testing.T) (context.Context, context.CancelFunc) {
	return context.WithTimeout(t.Context(), 30*time.Second)
}

// A turn that finished as Gimbal interrupted it is still confirmed: its
// terminal notification arrives during the drain, though the interrupt
// reports that the turn was already over.
func TestRunTurnCompletionRacingInterruptIsConfirmed(t *testing.T) {
	ad, server := startAppServer(t, func(s *appServer, message rpcMessage) {
		switch message.Method {
		case "turn/start":
			s.reply(message, `{"turn":{"id":"t1"}}`)
			s.notify("item/started", `"item":{"id":"a1","type":"agentMessage"}`)
		case "turn/interrupt":
			s.notify("item/completed", `"item":{"id":"a1","type":"agentMessage","text":"done"}`)
			s.completed("completed")
			s.reject(message, "no active turn to interrupt")
		}
	})
	ctx, cancel := turnCtx(t)
	defer cancel()
	_, err := ad.RunTurn(ctx, "thread", "go", nil, func(gimbal.AgentEvent) error { cancel(); return nil })
	if _, unconfirmed := errors.AsType[*turnUnconfirmedError](err); unconfirmed || !errors.Is(err, context.Canceled) {
		t.Fatalf("RunTurn = %v, want a confirmed cancellation", err)
	}
	if n := len(server.sent("turn/interrupt")); n != 1 {
		t.Fatalf("interrupts = %d, want 1", n)
	}
}

// Cancellation from the terminal event's own callback returns at once: the
// terminal was seen, so there is nothing to interrupt or drain.
func TestRunTurnCancelledByTerminalCallbackDoesNotInterrupt(t *testing.T) {
	ad, server := startAppServer(t, func(s *appServer, message rpcMessage) {
		if message.Method == "turn/start" {
			s.reply(message, `{"turn":{"id":"t1"}}`)
			s.notify("item/started", `"item":{"id":"a1","type":"agentMessage"}`)
			s.notify("item/completed", `"item":{"id":"a1","type":"agentMessage","text":"done"}`)
			s.completed("completed")
		}
	})
	ctx, cancel := turnCtx(t)
	defer cancel()
	began := time.Now()
	_, err := ad.RunTurn(ctx, "thread", "go", nil, func(event gimbal.AgentEvent) error {
		if event.Type == "session.step.ended" {
			cancel()
		}
		return nil
	})
	if _, unconfirmed := errors.AsType[*turnUnconfirmedError](err); unconfirmed || !errors.Is(err, context.Canceled) {
		t.Fatalf("RunTurn = %v, want the cancellation of a finished turn", err)
	}
	if elapsed := time.Since(began); elapsed >= controlTimeout {
		t.Fatalf("RunTurn took %v, waiting for a second terminal", elapsed)
	}
	if n := len(server.sent("turn/interrupt")); n != 0 {
		t.Fatalf("interrupts = %d, want 0", n)
	}
}

// A refused interactive request ends the read with ctx still live. The turn
// is interrupted before RunTurn returns, and a connection lost before its
// terminal leaves it unconfirmed.
func TestRunTurnRefusalInterruptsAndReportsUnconfirmedStop(t *testing.T) {
	ad, server := startAppServer(t, func(s *appServer, message rpcMessage) {
		switch message.Method {
		case "turn/start":
			s.reply(message, `{"turn":{"id":"t1"}}`)
			s.send(`{"id":"srv-1","method":"item/tool/requestUserInput","params":{"threadId":"thread","turnId":"t1","questions":[]}}`)
		case "turn/interrupt":
			s.reply(message, `{}`)
			_ = s.ws.CloseNow()
		}
	})
	ctx, cancel := turnCtx(t)
	defer cancel()
	_, err := ad.RunTurn(ctx, "thread", "go", nil, func(gimbal.AgentEvent) error { return nil })
	unconfirmed, ok := errors.AsType[*turnUnconfirmedError](err)
	if !ok || errors.Is(err, context.Canceled) {
		t.Fatalf("RunTurn = %v, want an unconfirmed stop", err)
	}
	if unconfirmed.turn != "t1" || !strings.Contains(err.Error(), "t1") || !strings.Contains(err.Error(), "unexpected interactive request") {
		t.Fatalf("RunTurn = %v, want the turn and the refusal named", err)
	}
	if n := len(server.sent("turn/interrupt")); n != 1 {
		t.Fatalf("interrupts = %d, want 1", n)
	}
}

// Cancellation while turn/start is outstanding still waits for its turn id,
// then stops that turn like any other.
func TestRunTurnCancelledDuringStartStopsTheAcknowledgedTurn(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	ad, server := startAppServer(t, func(s *appServer, message rpcMessage) {
		switch message.Method {
		case "turn/start":
			close(started)
			<-release
			s.reply(message, `{"turn":{"id":"t1"}}`)
		case "turn/interrupt":
			s.reply(message, `{}`)
			s.completed("interrupted")
		}
	})
	ctx, cancel := turnCtx(t)
	defer cancel()
	go func() {
		<-started
		cancel()
		close(release)
	}()
	_, err := ad.RunTurn(ctx, "thread", "go", nil, func(gimbal.AgentEvent) error { return nil })
	if _, unconfirmed := errors.AsType[*turnUnconfirmedError](err); unconfirmed || !errors.Is(err, context.Canceled) {
		t.Fatalf("RunTurn = %v, want a confirmed cancellation", err)
	}
	interrupts := server.sent("turn/interrupt")
	var params struct {
		TurnID string `json:"turnId"`
	}
	if len(interrupts) != 1 || json.Unmarshal(interrupts[0].Params, &params) != nil || params.TurnID != "t1" {
		t.Fatalf("interrupts = %+v, want one for t1", interrupts)
	}
}

// A turn/start that may have started a turn without identifying it is never
// a clean cancellation. A definite rejection is an ordinary failure.
func TestRunTurnAmbiguousStartIsUnconfirmed(t *testing.T) {
	for _, tc := range []struct {
		name        string
		answer      func(s *appServer, message rpcMessage)
		unconfirmed bool
	}{
		{"invalid acknowledgement", func(s *appServer, message rpcMessage) { s.reply(message, `{}`) }, true},
		{"connection lost", func(s *appServer, message rpcMessage) { _ = s.ws.CloseNow() }, true},
		{"rejected", func(s *appServer, message rpcMessage) { s.reject(message, "thread not loaded") }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			started, release := make(chan struct{}), make(chan struct{})
			ad, _ := startAppServer(t, func(s *appServer, message rpcMessage) {
				if message.Method == "turn/start" {
					close(started)
					<-release
					tc.answer(s, message)
				}
			})
			ctx, cancel := turnCtx(t)
			defer cancel()
			go func() {
				<-started
				cancel()
				close(release)
			}()
			_, err := ad.RunTurn(ctx, "thread", "go", nil, func(gimbal.AgentEvent) error { return nil })
			_, unconfirmed := errors.AsType[*turnUnconfirmedError](err)
			if err == nil || unconfirmed != tc.unconfirmed || errors.Is(err, context.Canceled) {
				t.Fatalf("RunTurn = %v, want unconfirmed=%v and no clean cancellation", err, tc.unconfirmed)
			}
		})
	}
}

// The drain outlasts nonterminal errors and interactive requests; the turn's
// terminal confirms the stop and the original provider failure is returned.
func TestRunTurnDrainOutlastsErrorsAndRequests(t *testing.T) {
	declined := make(chan struct{})
	ad, _ := startAppServer(t, func(s *appServer, message rpcMessage) {
		switch {
		case message.Method == "turn/start":
			s.reply(message, `{"turn":{"id":"t1"}}`)
			s.notify("error", `"error":{"message":"provider failed"},"willRetry":false`)
		case message.Method == "turn/interrupt":
			s.reply(message, `{}`)
			s.send(`{"id":"srv-2","method":"item/commandExecution/requestApproval","params":{"threadId":"thread","turnId":"t1","itemId":"c1"}}`)
			s.notify("error", `"error":{"message":"provider failed again"},"willRetry":false`)
			s.completed("failed")
		case string(message.ID) == `"srv-2"` && message.Method == "":
			close(declined)
		}
	})
	ctx, cancel := turnCtx(t)
	defer cancel()
	_, err := ad.RunTurn(ctx, "thread", "go", nil, func(gimbal.AgentEvent) error { return nil })
	if err == nil || err.Error() != "codex: provider failed" {
		t.Fatalf("RunTurn = %v, want the original provider failure alone", err)
	}
	select {
	case <-declined:
	case <-time.After(5 * time.Second):
		t.Fatal("the drain did not answer the interactive request")
	}
}

// The unconfirmed error keeps its type across Temporal's failure conversion,
// which is how the execution backend recognizes it on the controller.
func TestTurnUnconfirmedErrorCrossesTheActivityBoundary(t *testing.T) {
	original := &turnUnconfirmedError{thread: "thread", turn: "t1", err: context.Canceled}
	if errors.Is(original, context.Canceled) {
		t.Fatal("an unconfirmed stop unwraps to a clean cancellation")
	}
	converter := temporal.GetDefaultFailureConverter()
	err := converter.FailureToError(converter.ErrorToFailure(original))
	app, ok := errors.AsType[*temporal.ApplicationError](err)
	if !ok || app.Type() != "turnUnconfirmedError" || !strings.Contains(app.Error(), "t1") {
		t.Fatalf("converted = %#v, want application failure type turnUnconfirmedError naming the turn", err)
	}
}
