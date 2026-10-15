package effects

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/backend"
)

func TestWaitCancellationRequiresNativeTerminalInterruption(t *testing.T) {
	acknowledged, terminal := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tasks/interrupt":
			close(acknowledged)
			_ = json.NewEncoder(w).Encode(backend.Reply{})
		case "/tasks/result":
			select {
			case <-terminal:
				_ = json.NewEncoder(w).Encode(backend.Result{Done: true, Interrupted: true})
			default:
				_ = json.NewEncoder(w).Encode(backend.Result{})
			}
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	done := make(chan error, 1)
	go func() { _, err := Wait(ctx, server.URL, Target{Run: "run", ID: "original"}, nil); done <- err }()
	<-acknowledged
	select {
	case err := <-done:
		t.Fatalf("acknowledgement settled Stop: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(terminal)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("native interruption=%v", err)
	}
}

func TestWaitPreservesCompletionThatWinsStopRace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tasks/interrupt" {
			_ = json.NewEncoder(w).Encode(backend.Reply{})
			return
		}
		_ = json.NewEncoder(w).Encode(backend.Result{Done: true, Result: json.RawMessage(`{"output":"completed"}`)})
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	raw, err := Wait(ctx, server.URL, Target{Run: "run", ID: "original"}, nil)
	if err != nil || string(raw) != `{"output":"completed"}` {
		t.Fatalf("completion lost: %s %v", raw, err)
	}
}

func TestWaitUnconfirmedStopKeepsDeliveryUncertain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "unreachable", 503) }))
	defer server.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := Wait(ctx, server.URL, Target{Run: "run", ID: "original"}, nil)
	var uncertain *backend.DeliveryUncertain
	if !errors.As(err, &uncertain) || uncertain.RequestID != "original" {
		t.Fatalf("stop error=%v", err)
	}
}

func TestDetachedReaderCancellationNeverSendsInterrupt(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := Wait(ObserveDetached(ctx), server.URL, Target{Run: "run", ID: "original"}, nil)
	var uncertain *backend.DeliveryUncertain
	if !errors.As(err, &uncertain) || calls.Load() != 0 {
		t.Fatalf("detachment caused native effect: calls=%d err=%v", calls.Load(), err)
	}
}
