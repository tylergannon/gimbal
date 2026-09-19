package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tylergannon/gimble/internal/observation"
)

// controlDiscovery is the small file a local client finds below the project's
// .gimble directory. The file is only a hint: clients must dial Socket and
// ask the runtime for its live state before treating the instance as running.
type controlDiscovery struct {
	PID     int    `json:"pid"`
	Socket  string `json:"socket"`
	Project string `json:"project"`
}

type controlHandler struct {
	runtime *Runtime
}

func (h controlHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/control/runs":
		h.runs(w)
	case r.Method == http.MethodPost && r.URL.Path == "/control/steer":
		h.steer(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/control/steer-loop":
		h.steerLoop(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h controlHandler) runs(w http.ResponseWriter) {
	rows, err := h.runtime.controlRuns()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rows)
}

type controlSteer struct {
	Run     string `json:"run"`
	Session string `json:"session"`
	Message string `json:"message"`
}

func (h controlHandler) steer(w http.ResponseWriter, r *http.Request) {
	var request controlSteer
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	landed, err := h.runtime.Steer(r.Context(), request.Run, request.Session, request.Message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Landed bool `json:"landed"`
	}{Landed: landed})
}

type controlSteerLoop struct {
	Run     string `json:"run"`
	Scope   string `json:"scope"`
	Message string `json:"message"`
}

func (h controlHandler) steerLoop(w http.ResponseWriter, r *http.Request) {
	var request controlSteerLoop
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.runtime.SteerLoop(request.Run, request.Scope, request.Message); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Queued bool `json:"queued"`
	}{Queued: true})
}

func (r *Runtime) startControl() error {
	controlDir := filepath.Join(r.dir, "control")
	if err := os.MkdirAll(controlDir, 0o755); err != nil {
		return fmt.Errorf("gimble: control directory: %w", err)
	}

	var rawID [4]byte
	if _, err := rand.Read(rawID[:]); err != nil {
		return fmt.Errorf("gimble: control identity: %w", err)
	}
	identity := hex.EncodeToString(rawID[:])
	socket := filepath.Join(r.dir, identity+".sock")
	discovery := filepath.Join(controlDir, identity+".json")
	listener, err := net.Listen("unix", socket)
	if err != nil && strings.Contains(err.Error(), "invalid argument") {
		// macOS limits the total Unix socket path length. A long temporary or
		// checkout path cannot hold the socket itself, but its discovery file
		// still lives under the project and points to this short fallback.
		socket = filepath.Join("/tmp", "gimble-"+identity+".sock")
		listener, err = net.Listen("unix", socket)
	}
	if err != nil {
		return fmt.Errorf("gimble: listen on control socket: %w", err)
	}
	info := controlDiscovery{PID: os.Getpid(), Socket: socket, Project: r.dir}
	encoded, err := json.Marshal(info)
	if err != nil {
		_ = listener.Close()
		_ = os.Remove(socket)
		return fmt.Errorf("gimble: encode control discovery: %w", err)
	}
	if err := os.WriteFile(discovery, encoded, 0o644); err != nil {
		_ = listener.Close()
		_ = os.Remove(socket)
		return fmt.Errorf("gimble: write control discovery: %w", err)
	}

	server := &http.Server{
		Handler: controlMux(r),
		BaseContext: func(net.Listener) context.Context {
			return r.ctx
		},
	}
	r.shutdown.Add(1)
	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			r.cancel(fmt.Errorf("gimble: serve control socket: %w", err))
		}
	}()
	context.AfterFunc(r.ctx, func() {
		_ = server.Close()
		<-serveDone
		_ = os.Remove(socket)
		_ = os.Remove(discovery)
		r.shutdown.Done()
	})
	log.Printf("gimble: control socket listening on %s", socket)
	return nil
}

func controlMux(r *Runtime) http.Handler {
	return observation.Routes(controlHandler{runtime: r})
}

func (r *Runtime) controlRuns() ([]observation.RunRow, error) {
	entries, err := os.ReadDir(filepath.Join(r.dir, "runs"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []observation.RunRow{}, nil
		}
		return nil, err
	}
	rows := make([]observation.RunRow, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := r.runs.InProgress(entry.Name()); err != nil {
			continue
		}
		snapshot, err := r.registry.Snapshot(entry.Name())
		if err != nil {
			continue
		}
		rows = append(rows, snapshot.Run)
	}
	slices.SortFunc(rows, func(a, b observation.RunRow) int {
		return strings.Compare(a.ID, b.ID)
	})
	return rows, nil
}
