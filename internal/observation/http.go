package observation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// writeTimeout bounds one write to one connection. A peer that stops reading
// its socket must not hold a server goroutine forever; the subscription is
// closed and the browser reconnects to a fresh snapshot.
const writeTimeout = 10 * time.Second

// Routes answers the observation endpoints and passes every other request
// on. It takes its registry from the request context, which the web server
// supplies from the runtime context through BaseContext, so there is one
// composition and a test injects the same context.
func Routes(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", next)
	mux.HandleFunc("GET /api/runs/{runID}", serveSnapshot)
	mux.HandleFunc("GET /api/runs/{runID}/events", serveEvents)
	return mux
}

func serveSnapshot(w http.ResponseWriter, r *http.Request) {
	registry := FromContext(r.Context())
	if registry == nil {
		http.Error(w, "no observation registry in this server's context", http.StatusInternalServerError)
		return
	}
	snapshot, err := registry.Snapshot(r.PathValue("runID"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrNoRun) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snapshot)
}

func serveEvents(w http.ResponseWriter, r *http.Request) {
	registry := FromContext(r.Context())
	if registry == nil {
		http.Error(w, "no observation registry in this server's context", http.StatusInternalServerError)
		return
	}
	runID := r.PathValue("runID")
	store, live := registry.Live(runID)

	// An unknown run is a 404 before anything is streamed, so a browser sees
	// an ordinary failed request rather than an empty event stream.
	var snapshot RunSnapshot
	var sub *Subscription
	if live {
		var err error
		snapshot, sub, err = store.Subscribe()
		if err != nil {
			live = false
		}
	}
	if !live {
		var err error
		if snapshot, err = registry.Snapshot(runID); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, ErrNoRun) {
				status = http.StatusNotFound
			}
			http.Error(w, err.Error(), status)
			return
		}
	}
	if sub != nil {
		defer sub.Close()
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	control := http.NewResponseController(w)
	// Every connection starts with a complete replacement snapshot. There is
	// no cursor, no Last-Event-ID and no per-connection history replay.
	if err := writeFrame(w, control, Frame{Name: FrameSnapshot, Data: mustMarshal(snapshot)}); err != nil {
		return
	}
	if sub == nil {
		// A finished run has a complete snapshot and no suffix.
		return
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case <-sub.Done():
			// Overflow, or this reader was released. An overflowed suffix is
			// reset rather than delivered: the browser reconnects to a fresh
			// complete snapshot.
			return
		case frame, open := <-sub.Frames():
			if !open {
				// The run finished normally and its queue has been handed
				// over in full, the run's terminal row included.
				return
			}
			sub.Took(frame)
			if err := writeFrame(w, control, frame); err != nil {
				return
			}
		}
	}
}

func writeFrame(w io.Writer, control *http.ResponseController, frame Frame) error {
	if err := control.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil &&
		!errors.Is(err, http.ErrNotSupported) {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", frame.Name, frame.Data); err != nil {
		return err
	}
	return control.Flush()
}
