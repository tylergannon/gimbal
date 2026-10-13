package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
	p.handle = func(ctx context.Context, r backend.MessageRequest) error {
		return handle(&producer{bus: p.bus, ctx: ctx}, r)
	}
	server := httptest.NewServer(p.handler())
	p.bus.URL = server.URL
	t.Cleanup(func() { cancel(); server.Close() })
	return p, server
}
func request(kind string) backend.MessageRequest {
	r := backend.MessageRequest{ID: ulid.Make().String(), Created: time.Now().UTC(), Run: "run", Scope: "root", Task: ulid.Make().String(), Kind: kind}
	switch kind {
	case "turn":
		r.SessionID = "session"
		r.Turn = &backend.Turn{Owner: r.Owner, Session: r.SessionID}
	case "process":
		r.Process = &backend.Process{Owner: r.Owner}
	case "cleanup":
		r.ScopeTarget = &backend.Scope{Run: r.Run, Scope: r.Scope}
	}
	return r
}
func TestAssignedDaemonAcceptsAllKindsAndRetainsIndependentObservations(t *testing.T) {
	var count atomic.Int32
	p, server := newProducer(t, func(p *producer, r backend.MessageRequest) error {
		count.Add(1)
		return p.fact(r, "completed", &backend.MessageResult{Output: []byte{0, 255}})
	})
	bus := &connection{URL: server.URL, client: server.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, kind := range []string{"turn", "process", "cleanup"} {
		r := request(kind)
		if err := bus.PublishRequest(ctx, r); err != nil {
			t.Fatal(err)
		}
		if err := bus.PublishRequest(ctx, r); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			sentinel := errors.New("observed")
			err := bus.ListenEvents(ctx, backend.MessageSelection{RequestID: r.ID}, r.Created, func(e backend.MessageEvent) error {
				if e.Result == nil || len(e.Result.Output) != 2 || e.Result.Output[1] != 255 {
					t.Fatalf("native output lost: %+v", e)
				}
				return sentinel
			})
			if !errors.Is(err, sentinel) {
				t.Fatal(err)
			}
		}
	}
	if count.Load() != 3 {
		t.Fatalf("duplicate admission: %d", count.Load())
	}
	_ = p
}
func TestPublicationDoesNotWaitForNativeWork(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	_, server := newProducer(t, func(*producer, backend.MessageRequest) error { <-block; return nil })
	bus := &connection{URL: server.URL, client: server.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, kind := range []string{"process", "cleanup"} {
		if err := bus.PublishRequest(ctx, request(kind)); err != nil {
			t.Fatal(err)
		}
	}
}
func TestConfigUsesOneExplicitDaemon(t *testing.T) {
	for _, raw := range []string{`{"URL":"http://127.0.0.1:8081"}`, `{"URL":"http://worker:8081","Listen":"0.0.0.0:8081","EventsDir":"/private/events"}`} {
		if _, err := parse(json.RawMessage(raw)); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{`{}`, `{"URL":"http://user:pass@localhost"}`, `{"URL":"http://localhost","Listen":":8081"}`} {
		if _, err := parse(json.RawMessage(raw)); err == nil {
			t.Fatalf("invalid config accepted: %s", raw)
		}
	}
}
