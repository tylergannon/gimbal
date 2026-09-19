package observation

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestEventsFlushHeadersAtCurrentPositionWhileAnOpenRunIsIdle(t *testing.T) {
	registry := NewRegistry(t.TempDir())
	store, err := Open(registry, "run-1", "idle", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	server := httptest.NewUnstartedServer(Routes(http.NotFoundHandler()))
	server.Config.BaseContext = func(net.Listener) context.Context {
		return WithRegistry(context.Background(), registry)
	}
	server.Start()
	defer server.Close()

	snapshot, err := registry.Snapshot("run-1")
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/runs/run-1/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	query := request.URL.Query()
	query.Set("stream", snapshot.Stream)
	query.Set("position", strconv.FormatUint(snapshot.Position, 10))
	request.URL.RawQuery = query.Encode()
	client := server.Client()
	client.Timeout = 2 * time.Second
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET the event stream: status %d", response.StatusCode)
	}
	if got := response.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
}
