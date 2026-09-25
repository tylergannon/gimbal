package web

import (
	"bufio"
	"context"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/observation"
)

// frame is one SSE frame: its event name and its data line.
type frame struct {
	name string
	data string
}

// TestRunEventsStreamTheStore is definition-of-done item 5 at the boundary
// the browser uses: the run's event stream opens with a complete snapshot,
// a step that reached a model pushes the recomputed roll-ups, and the stream
// ends when the run does, without the browser polling for any of it.
func TestRunEventsStreamTheStore(t *testing.T) {
	dist, err := fs.Sub(Build, "build")
	if err != nil {
		t.Fatal(err)
	}
	handler, _, err := NewHandler(dist, "", "")
	if err != nil {
		t.Fatal(err)
	}

	project := t.TempDir()
	registry := observation.NewRegistry(project)
	server := httptest.NewUnstartedServer(handler)
	server.Config.BaseContext = func(net.Listener) context.Context {
		return observation.WithRegistry(context.Background(), registry)
	}
	server.Start()
	defer server.Close()

	runDir := filepath.Join(project, "runs", "run-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := observation.Open(registry, "run-1", "demo", runDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range []json.RawMessage{
		json.RawMessage(`{"seq":1,"time":"2026-09-13T00:00:00Z","scope":"","event":{"kind":"scope_began","name":"."}}`),
		json.RawMessage(`{"seq":2,"time":"2026-09-13T00:00:00Z","scope":"lap.1","event":{"kind":"scope_began","name":"lap.1"}}`),
		json.RawMessage(`{"seq":3,"time":"2026-09-13T00:00:01Z","scope":"lap.1","session":"writer","event":{"kind":"session_created","name":"writer","adapter":"fixture","model":"test-model","workdir":"/w"}}`),
		json.RawMessage(`{"seq":4,"time":"2026-09-13T00:00:02Z","scope":"lap.1","session":"writer","turn":"turn-1","event":{"kind":"turn_started","prompt":"write","output_type":"gimbal.Text"}}`),
	} {
		if err := store.Lifecycle(record); err != nil {
			t.Fatalf("fold %s: %v", record, err)
		}
	}

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/runs/run-1/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET the event stream: status %d", response.StatusCode)
	}

	frames := make(chan frame, 32)
	go func() {
		defer close(frames)
		scanner := bufio.NewScanner(response.Body)
		scanner.Buffer(make([]byte, 0, 64<<10), 8<<20)
		name := ""
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				frames <- frame{name: name, data: strings.TrimPrefix(line, "data: ")}
			}
		}
	}()
	next := func(what string) frame {
		select {
		case got, ok := <-frames:
			if !ok {
				t.Fatalf("the stream ended before %s", what)
			}
			return got
		case <-time.After(5 * time.Second):
			t.Fatalf("no frame for %s", what)
			return frame{}
		}
	}

	// Every connection opens with a complete snapshot, so a browser that
	// joined late has the turn that is already going.
	opening := next("the opening snapshot")
	if opening.name != observation.FrameSnapshot {
		t.Fatalf("the stream opened with a %q frame", opening.name)
	}
	var snapshot observation.RunSnapshot
	if err := json.Unmarshal([]byte(opening.data), &snapshot); err != nil {
		t.Fatalf("decode the opening snapshot: %v", err)
	}
	if snapshot.Run.ID != "run-1" || snapshot.Turns["turn-1"].Prompt != "write" {
		t.Fatalf("the opening snapshot is not this run's: %+v", snapshot)
	}

	at := observation.Placement{Scope: "lap.1", Session: "writer", Turn: "turn-1"}
	step := json.RawMessage(`{"id":"evt1","type":"session.step.ended","created":13,` +
		`"data":{"sessionID":"ses_writer","assistantMessageID":"msg_writer","finish":"stop","cost":0.125,` +
		`"tokens":{"input":4321,"output":1,"reasoning":0,"cache":{"read":0,"write":0}}}}`)
	if err := store.Event(at, step, nil); err != nil {
		t.Fatal(err)
	}

	// The roll-ups arrive inside one transaction-granular delta, already
	// summed in Go. The cursor covers the event, row, and totals together.
	var totals observation.Totals
	for {
		got := next("the totals frame")
		if got.name != "delta" {
			continue
		}
		var delta observation.Delta
		if err := json.Unmarshal([]byte(got.data), &delta); err != nil {
			t.Fatalf("decode delta: %v", err)
		}
		if delta.Stream != snapshot.Stream || delta.Position != snapshot.Position+1 {
			t.Fatalf("delta cursor = %s/%d, snapshot = %s/%d", delta.Stream, delta.Position, snapshot.Stream, snapshot.Position)
		}
		for _, one := range delta.Frames {
			if one.Name == observation.FrameTotals && json.Unmarshal(one.Data, &totals) != nil {
				t.Fatal("decode totals")
			}
		}
		if totals.Scopes != nil {
			break
		}
	}
	if got := totals.Scopes["lap.1"].All.Input; got != 4321 {
		t.Fatalf("the totals frame says lap.1 spent %v input, want 4321", got)
	}
	if got := totals.Scopes[""].ByModel["test-model"].StatedCost; got != 0.125 {
		t.Fatalf("the root's per-model stated cost = %v, want 0.125", got)
	}

	// And the stream ends with the run, without the browser polling for it.
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case _, open := <-frames:
			if !open {
				return
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the stream did not end when the run did")
		}
	}
}
