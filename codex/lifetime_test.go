package codex

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/tylergannon/gimbal"
)

func TestTerminalRPCReadClosesSocketReaderAndPendingCalls(t *testing.T) {
	for _, malformed := range []bool{true, false} {
		t.Run(map[bool]string{true: "malformed frame", false: "peer closed"}[malformed], func(t *testing.T) {
			peerClosed := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ws, err := websocket.Accept(w, r, nil)
				if err != nil {
					return
				}
				defer func() { _ = ws.CloseNow(); close(peerClosed) }()
				if _, _, err := ws.Read(r.Context()); err != nil {
					return
				}
				if malformed {
					_ = ws.Write(r.Context(), websocket.MessageText, []byte("{"))
					_, _, _ = ws.Read(r.Context()) // observes our socket retirement
				}
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = ws.CloseNow() }()
			debugFile, err := os.CreateTemp(t.TempDir(), "frames-*.jsonl")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = debugFile.Close() }()
			conn := &connection{ws: ws, debug: &rawRecorder{file: debugFile}, pending: make(map[int64]chan rpcMessage), threads: make(map[string]chan rpcMessage), readDone: make(chan struct{}), readerDone: make(chan struct{})}
			conn.registerThread("owned")
			go conn.read()
			if _, err := conn.call(ctx, "await", nil); !isTransportError(err) {
				t.Fatalf("pending request did not fail with its transport: %v", err)
			}
			for _, done := range []<-chan struct{}{peerClosed, conn.readerDone} {
				select {
				case <-done:
				case <-ctx.Done():
					t.Fatal("terminal read retained socket or reader")
				}
			}
			conn.finishRead(errors.New("second retirement"))
			if _, err := conn.next(ctx, conn.registerThread("late parent")); !isTransportError(err) {
				t.Fatalf("late reader lost terminal transport error: %v", err)
			}
			conn.routeThread("late child", make(chan rpcMessage, 1))
			if len(conn.pending) != 0 || len(conn.threads) != 0 || len(conn.queues) != 0 {
				t.Fatal("terminal connection retained request or session routing")
			}
			if _, err := debugFile.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("reader retained its debug descriptor: %v", err)
			}
		})
	}
}

func TestClosedActiveSessionDoesNotRestoreRoutesWhileAnotherSessionLives(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)
	shortenControlTimeout(t)
	ad, _ := newRecoveryDaemon(t, "default")
	id, err := ad.CreateSession(t.Context(), "stalled", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	other, err := ad.CreateSession(t.Context(), "simple", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ad.DetachSession(t.Context(), other); err != nil {
			t.Error(err)
		}
	}()
	conn := ad.current()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := ad.RunTurn(ctx, id, "one assignment", nil, func(gimbal.AgentEvent) error { return nil })
		done <- err
	}()
	for {
		s, _ := ad.session(id)
		active := s.getActive()
		started := false
		if active != nil {
			active.mu.Lock()
			started = active.turnID != ""
			active.mu.Unlock()
		}
		if started {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("turn never started")
		case <-time.After(time.Millisecond):
		}
	}
	closing, stop := context.WithCancel(t.Context())
	stop()
	if err := ad.Close(closing, id); err == nil {
		t.Fatal("expected cancelled archive")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("scope cancellation lost: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("local turn retained")
	}
	conn.threadsMu.Lock()
	_, retained := conn.threads[id]
	routes, queues := len(conn.threads), len(conn.queues)
	conn.threadsMu.Unlock()
	if retained || routes != 1 || queues != 1 || conn.dead() {
		t.Fatalf("ended scope restored routing or disrupted other session: retained=%t routes=%d queues=%d", retained, routes, queues)
	}
}

func TestDuplicateRPCReplyDoesNotRetainTheReader(t *testing.T) {
	conn := &connection{pending: map[int64]chan rpcMessage{1: make(chan rpcMessage, 1)}, readDone: make(chan struct{})}
	conn.pending[1] <- rpcMessage{Result: json.RawMessage(`{}`)}
	done := make(chan struct{})
	go func() {
		conn.dispatch(rpcMessage{ID: json.RawMessage(`1`), Result: json.RawMessage(`{}`)})
		conn.finishRead(errors.New("owner ended"))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("duplicate reply blocked connection retirement")
	}
}

func TestFailedSessionReleaseEndsLocalScopeAndPreservesOtherUsers(t *testing.T) {
	for _, method := range []string{"thread/archive", "thread/unsubscribe"} {
		t.Run(method, func(t *testing.T) {
			ad, daemon := newRecoveryDaemon(t, "default")
			owned, err := ad.CreateSession(t.Context(), "simple", "", t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			other, err := ad.CreateSession(t.Context(), "simple", "", t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			conn := ad.current()
			daemon.mu.Lock()
			daemon.rejectMethods = map[string]bool{method: true}
			daemon.mu.Unlock()
			if err := ad.releaseSession(t.Context(), owned, method); err == nil || !strings.Contains(err.Error(), "unconfirmed") {
				t.Fatalf("lost native failure: %v", err)
			}
			if _, err := ad.session(owned); err == nil {
				t.Fatal("failed release retained session")
			}
			conn.threadsMu.Lock()
			_, retained := conn.threads[owned]
			conn.threadsMu.Unlock()
			if retained || ad.current() != conn || conn.dead() {
				t.Fatal("release retained routing or disrupted another user")
			}
			if _, err := ad.RunTurn(t.Context(), other, "continue", nil, func(gimbal.AgentEvent) error { return nil }); err != nil {
				t.Fatalf("unrelated session failed: %v", err)
			}
			daemon.mu.Lock()
			daemon.rejectMethods = nil
			daemon.mu.Unlock()
			if err := ad.DetachSession(t.Context(), other); err != nil {
				t.Fatal(err)
			}
			if ad.current() != nil {
				t.Fatal("idle adapter retained its socket")
			}
			<-conn.readerDone
			// Detaching the human-owned thread preserved native identity/history.
			resumed := New().(*adapter)
			resumed.dial = ad.dial
			if err := resumed.ResumeSession(t.Context(), other, "simple", "", t.TempDir()); err != nil {
				t.Fatal(err)
			}
			if _, err := resumed.RunTurn(t.Context(), other, "resume", nil, func(gimbal.AgentEvent) error { return nil }); err != nil {
				t.Fatal(err)
			}
			if err := resumed.DetachSession(t.Context(), other); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLastSessionReleaseDoesNotCloseAnInFlightAdmission(t *testing.T) {
	ad, daemon := newRecoveryDaemon(t, "default")
	id, err := ad.CreateSession(t.Context(), "simple", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	conn := ad.current()
	entered, proceed := make(chan struct{}), make(chan struct{})
	daemon.mu.Lock()
	daemon.startEntered, daemon.startContinue = entered, proceed
	daemon.mu.Unlock()
	admitted := make(chan string, 1)
	go func() {
		id, err := ad.CreateSession(t.Context(), "simple", "", "/fixture")
		if err != nil {
			t.Error(err)
		}
		admitted <- id
	}()
	<-entered
	if err := ad.Close(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	if ad.current() != conn || conn.dead() {
		t.Fatal("last session closed the socket of an outstanding admission")
	}
	close(proceed)
	other := <-admitted
	if other == "" || ad.current() != conn || conn.dead() {
		t.Fatal("ending one scope disrupted a concurrent admission")
	}
	if err := ad.Close(t.Context(), other); err != nil {
		t.Fatal(err)
	}
}

func TestClosingAnActiveSessionReleasesLocalOwnerWithoutDuplicatingWork(t *testing.T) {
	shortenControlTimeout(t)
	ad, daemon := newRecoveryDaemon(t, "default")
	id, err := ad.CreateSession(t.Context(), "stalled", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	conn := ad.current()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		_, err := ad.RunTurn(ctx, id, "one assignment", nil, func(gimbal.AgentEvent) error { return nil })
		finished <- err
	}()
	for {
		daemon.mu.Lock()
		started := daemon.starts == 1
		daemon.mu.Unlock()
		if started {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("turn never started")
		case <-time.After(time.Millisecond):
		}
	}
	if _, err := ad.RunTurn(ctx, id, "must not duplicate", nil, nil); err == nil {
		t.Fatal("one native session admitted concurrent work")
	}
	daemon.mu.Lock()
	daemon.rejectMethods = map[string]bool{"thread/archive": true}
	daemon.ignoreInterrupt = true
	daemon.mu.Unlock()
	if err := ad.Close(ctx, id); err == nil {
		t.Fatal("archive failure was hidden")
	}
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "may still be running") {
			t.Fatalf("native uncertainty lost: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("ending scope retained an active local owner")
	}
	if ad.current() != nil {
		t.Fatal("active owner kept the ending adapter connected")
	}
	<-conn.readerDone
	daemon.mu.Lock()
	defer daemon.mu.Unlock()
	if daemon.starts != 1 {
		t.Fatalf("duplicate assignments: %d", daemon.starts)
	}
}
