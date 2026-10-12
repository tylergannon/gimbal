package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/backend/messagefiles"
)

func newProducer(t *testing.T, handle func(*producer, backend.MessageRequest) error) (*producer, *httptest.Server) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	store, err := messagefiles.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := &producer{ctx: ctx, bus: &connection{client: http.DefaultClient, store: store}, admitted: map[string]*admission{}, errors: make(chan error, 1)}
	p.handle = func(_ context.Context, r backend.MessageRequest) error { return handle(p, r) }
	server := httptest.NewServer(p.handler())
	t.Cleanup(func() { cancel(); server.Close() })
	return p, server
}
func pair(t *testing.T, a, b func(*producer, backend.MessageRequest) error) (*connection, *producer, *producer) {
	t.Helper()
	pa, sa := newProducer(t, a)
	pb, sb := newProducer(t, b)
	c := config{AgentURL: sa.URL, ProcessURL: sb.URL}
	pa.bus.config = c
	pb.bus.config = c
	return &connection{config: c, client: http.DefaultClient}, pa, pb
}
func request(kind string) backend.MessageRequest {
	return backend.MessageRequest{ID: ulid.Make().String(), Created: time.Now().UTC(), Run: "test-run", Scope: "scope", Task: "task", Kind: kind}
}
func receive(t *testing.T, ch <-chan backend.MessageEvent) backend.MessageEvent {
	t.Helper()
	select {
	case e := <-ch:
		return e
	case <-time.After(3 * time.Second):
		t.Fatal("event not delivered")
		return backend.MessageEvent{}
	}
}

func TestHTTPRequestsAndRetainedFanInHaveIndependentListeners(t *testing.T) {
	var agentCalls, processCalls atomic.Int32
	completed := make(chan struct{}, 2)
	output := []byte{' ', 0xff, '{', 'b', 'a', 'd'}
	complete := func(count *atomic.Int32) func(*producer, backend.MessageRequest) error {
		return func(p *producer, r backend.MessageRequest) error {
			count.Add(1)
			if err := p.fact(r, "accepted", nil); err != nil {
				return err
			}
			err := p.fact(r, "completed", &backend.MessageResult{Output: output, InvalidOutput: true, Error: "invalid JSON", Stdout: []byte{0, 255}, Stderr: []byte("stderr\n")})
			completed <- struct{}{}
			return err
		}
	}
	b, _, _ := pair(t, complete(&agentCalls), complete(&processCalls))
	a, p := request("turn"), request("process")
	for _, r := range []backend.MessageRequest{a, p, a, p} {
		if err := b.PublishRequest(context.Background(), r); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		select {
		case <-completed:
		case <-time.After(3 * time.Second):
			t.Fatal("completion not published")
		}
	}
	if agentCalls.Load() != 1 || processCalls.Load() != 1 {
		t.Fatalf("duplicate execution: agent=%d process=%d", agentCalls.Load(), processCalls.Load())
	}
	// Both listeners begin after authoritative completion. Neither consumes it.
	for range 2 {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		got := map[string]bool{}
		stop := errors.New("complete")
		err := b.ListenEvents(ctx, backend.MessageSelection{Run: "test-run"}, a.Created, func(e backend.MessageEvent) error {
			if e.Kind != "completed" {
				return nil
			}
			if e.Result == nil || !reflect.DeepEqual(e.Result.Output, output) || !e.Result.InvalidOutput || e.Result.Error != "invalid JSON" || !reflect.DeepEqual(e.Result.Stdout, []byte{0, 255}) {
				t.Errorf("result bytes/outcome changed: %+v", e.Result)
			}
			got[e.RequestID] = true
			if len(got) == 2 {
				return stop
			}
			return nil
		})
		cancel()
		if !errors.Is(err, stop) || !got[a.ID] || !got[p.ID] {
			t.Fatalf("fan-in: %v, requests %v", err, got)
		}
	}
}

func TestReconnectUsesOriginalBoundAndDoesNotBlockOtherProducer(t *testing.T) {
	since := time.Now().UTC().Add(-time.Second)
	older := backend.MessageEvent{ID: ulid.Make().String(), RequestID: "old", Created: since.Add(time.Millisecond), Kind: "completed", Result: &backend.MessageResult{Output: []byte("late old result")}}
	newer := backend.MessageEvent{ID: ulid.Make().String(), RequestID: "new", Created: since.Add(2 * time.Millisecond), Kind: "completed", Result: &backend.MessageResult{}}
	disconnect := make(chan struct{})
	reconnect := make(chan struct{})
	var calls atomic.Int32
	var wrongBound atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("since") != since.Format(time.RFC3339Nano) {
			wrongBound.Store(true)
		}
		n := calls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		send := func(e backend.MessageEvent) {
			data, _ := json.Marshal(e)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			_ = http.NewResponseController(w).Flush()
		}
		send(newer)
		if n == 1 {
			select {
			case <-disconnect:
				return
			case <-r.Context().Done():
				return
			}
		}
		select {
		case <-reconnect:
			send(older)
		case <-r.Context().Done():
			return
		}
		<-r.Context().Done()
	}))
	defer server.Close()
	process, processServer := newProducer(t, func(*producer, backend.MessageRequest) error { return nil })
	b := &connection{AgentURL: server.URL, ProcessURL: processServer.URL, client: http.DefaultClient}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan backend.MessageEvent, 10)
	done := make(chan error, 1)
	go func() {
		done <- b.ListenEvents(ctx, backend.MessageSelection{}, since, func(e backend.MessageEvent) error { events <- e; return nil })
	}()
	if got := receive(t, events); got.RequestID != "new" {
		t.Fatalf("first event: %+v", got)
	}
	close(disconnect)
	// The other producer advances while the agent stream is disconnected or
	// reconnecting. A global cursor would also incorrectly skip the older fact.
	processEvent := backend.MessageEvent{ID: ulid.Make().String(), RequestID: "process", Created: time.Now().UTC(), Kind: "established"}
	if err := process.bus.PublishEvent(ctx, processEvent); err != nil {
		t.Fatal(err)
	}
	if got := receive(t, events); got.RequestID != "process" {
		t.Fatalf("other stream blocked or duplicate emitted: %+v", got)
	}
	close(reconnect)
	if got := receive(t, events); got.RequestID != "old" {
		t.Fatalf("late older event missed: %+v", got)
	}
	select {
	case extra := <-events:
		t.Fatalf("duplicate: %+v", extra)
	case <-time.After(100 * time.Millisecond):
	}
	cancel()
	<-done
	if wrongBound.Load() {
		t.Fatal("reconnect advanced the original lower bound")
	}
	if calls.Load() < 2 {
		t.Fatal("stream did not reconnect")
	}
}

func TestBroadcastWaitsForBothProducers(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	var childrenMu sync.Mutex
	var children []string
	handle := func(wait bool) func(*producer, backend.MessageRequest) error {
		return func(p *producer, r backend.MessageRequest) error {
			childrenMu.Lock()
			children = append(children, r.ID)
			childrenMu.Unlock()
			if err := p.fact(r, "accepted", nil); err != nil {
				return err
			}
			go func() {
				if wait {
					close(started)
					select {
					case <-release:
					case <-p.ctx.Done():
						return
					}
				}
				_ = p.fact(r, "completed", &backend.MessageResult{})
			}()
			return nil
		}
	}
	b, _, _ := pair(t, handle(false), handle(true))
	original := request("cleanup")
	if err := b.PublishRequest(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan backend.MessageEvent, 10)
	done := make(chan error, 1)
	go func() {
		done <- b.ListenEvents(ctx, backend.MessageSelection{RequestID: original.ID}, original.Created, func(e backend.MessageEvent) error { events <- e; return nil })
	}()
	if got := receive(t, events); got.Kind != "accepted" {
		t.Fatalf("first event: %+v", got)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("second producer did not receive control")
	}
	select {
	case got := <-events:
		t.Fatalf("control completed before both producers: %+v", got)
	case <-time.After(150 * time.Millisecond):
	}
	close(release)
	got := receive(t, events)
	if got.Kind != "completed" || got.Result == nil || got.Result.Error != "" {
		t.Fatalf("aggregate completion: %+v", got)
	}
	childrenMu.Lock()
	defer childrenMu.Unlock()
	if len(children) != 2 || children[0] == original.ID || children[1] == original.ID || children[0] == children[1] {
		t.Fatalf("control identities: parent %s children %v", original.ID, children)
	}
	cancel()
	<-done
}

func TestObserverDisconnectDoesNotEndAdmittedWork(t *testing.T) {
	release := make(chan struct{})
	b, agent, _ := pair(t, func(p *producer, r backend.MessageRequest) error {
		if err := p.fact(r, "accepted", nil); err != nil {
			return err
		}
		go func() {
			select {
			case <-p.ctx.Done():
				return
			case <-release:
				_ = p.fact(r, "completed", &backend.MessageResult{Output: []byte("still alive")})
			}
		}()
		return nil
	}, func(*producer, backend.MessageRequest) error { return nil })
	r := request("turn")
	ctx, cancel := context.WithCancel(context.Background())
	if err := b.PublishRequest(ctx, r); err != nil {
		t.Fatal(err)
	}
	// Cancelling both publication's context and the short observer does not end
	// the producer's admitted work; its later completion is retained.
	cancel()
	stop := errors.New("disconnect")
	err := b.ListenEvents(context.Background(), backend.MessageSelection{RequestID: r.ID}, r.Created, func(backend.MessageEvent) error { return stop })
	if !errors.Is(err, stop) {
		t.Fatal(err)
	}
	if agent.ctx.Err() != nil {
		t.Fatal("observer ended producer")
	}
	close(release)
	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = b.ListenEvents(ctx, backend.MessageSelection{RequestID: r.ID}, r.Created, func(e backend.MessageEvent) error {
		if e.Kind == "completed" {
			if string(e.Result.Output) != "still alive" {
				t.Errorf("output: %+v", e.Result)
			}
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) {
		t.Fatal(err)
	}
}

func TestConfigRequiresSeparateLocalProducers(t *testing.T) {
	for _, c := range []config{
		{AgentURL: "http://example.com", ProcessURL: "http://localhost:2"},
		{AgentURL: "http://localhost:1", ProcessURL: "http://localhost:1"},
		{AgentURL: "http://localhost:1", ProcessURL: "http://localhost:2", Listen: "0.0.0.0:3", EventsDir: "/tmp/events"},
	} {
		raw, _ := json.Marshal(c)
		if _, err := parse(raw); err == nil {
			t.Fatalf("accepted %+v", c)
		}
	}
}

func TestPublicationAndControlsDoNotWaitForSlowAdmission(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	started := make(chan struct{})
	handle := func(p *producer, r backend.MessageRequest) error {
		if r.Kind == "process" {
			close(started)
			select {
			case <-release:
			case <-p.ctx.Done():
				return p.ctx.Err()
			}
		}
		if err := p.fact(r, "accepted", nil); err != nil {
			return err
		}
		return p.fact(r, "completed", &backend.MessageResult{})
	}
	b, _, _ := pair(t, handle, handle)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := b.PublishRequest(ctx, request("process")); err != nil {
		t.Fatalf("publication waited for native admission: %v", err)
	}
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("native handler never started")
	}
	control := request("cleanup")
	if err := b.PublishRequest(ctx, control); err != nil {
		t.Fatal(err)
	}
	stop := errors.New("completed")
	err := b.ListenEvents(ctx, backend.MessageSelection{RequestID: control.ID}, control.Created, func(e backend.MessageEvent) error {
		if e.Kind == "completed" {
			if e.Result == nil || e.Result.Error != "" {
				t.Errorf("cleanup: %+v", e.Result)
			}
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) {
		t.Fatalf("control blocked behind native action admission: %v", err)
	}
}
