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

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/binding"
	"github.com/tylergannon/gimble/internal/host"
	"github.com/tylergannon/gimble/internal/observation"
)

// controlDiscovery is the small file a local client finds below the instance
// state directory. The file is only a hint: clients must dial Socket and
// ask the runtime for its live state before treating the instance as running.
type controlDiscovery struct {
	PID     int    `json:"pid"`
	Socket  string `json:"socket"`
	Project string `json:"project"`
}

type controlHandler struct {
	instance *Instance
}

func (h controlHandler) project(r *http.Request) (*host.Project, error) {
	name := r.Header.Get("X-Gimble-Project")
	if name == "" {
		return nil, errors.New("gimble: project is required")
	}
	return h.instance.Owner.Project(name)
}

func (h controlHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/control/runs":
		h.runs(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/control/submit":
		h.submit(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/control/steer":
		h.steer(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/control/steer-loop":
		h.steerLoop(w, r)
	default:
		http.NotFound(w, r)
	}
}

// Submission carries choices across the process boundary without binding a
// harness in the client process. The instance resolves models and executables.
type Submission struct {
	Name         string                         `json:"name"`
	Params       json.RawMessage                `json:"params"`
	Models       map[gimble.WorkflowRole]string `json:"models"`
	WorkDir      string                         `json:"work_dir"`
	Conversation string                         `json:"conversation,omitempty"`
}

type Admission struct {
	ID string `json:"id"`
}

func (h controlHandler) submit(w http.ResponseWriter, r *http.Request) {
	p, err := h.project(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var request Submission
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	entry := h.instance.workflows[request.Name]
	if entry == nil {
		http.Error(w, "unknown built-in workflow "+request.Name, http.StatusBadRequest)
		return
	}
	models, err := binding.Roles(request.Models)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id, err := p.Start(request.Name, request.WorkDir, request.Conversation, models, func(ctx context.Context) error {
		return entry(ctx, gimble.Env{WorkDir: request.WorkDir}, request.Params)
	})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, host.ErrStartFailed) {
			status = http.StatusInternalServerError
		} else if errors.Is(err, host.ErrStopped) {
			status = http.StatusServiceUnavailable
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(Admission{ID: id})
}

func (h controlHandler) runs(w http.ResponseWriter, r *http.Request) {
	p, err := h.project(r)
	if err != nil || p == nil {
		http.Error(w, "project is not admitted", http.StatusNotFound)
		return
	}
	rows, err := controlRuns(p)
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
	p, err := h.project(r)
	if err != nil || p == nil {
		http.Error(w, "project is not admitted", http.StatusNotFound)
		return
	}
	landed, err := p.Steer(r.Context(), request.Run, request.Session, request.Message)
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
	p, err := h.project(r)
	if err != nil || p == nil {
		http.Error(w, "project is not admitted", http.StatusNotFound)
		return
	}
	if err := p.SteerLoop(request.Run, request.Scope, request.Message); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Queued bool `json:"queued"`
	}{Queued: true})
}

func (i *Instance) startControl() error {
	controlDir := filepath.Join(i.dir, "control")
	if err := os.MkdirAll(controlDir, 0o755); err != nil {
		return fmt.Errorf("gimble: control directory: %w", err)
	}

	var rawID [4]byte
	if _, err := rand.Read(rawID[:]); err != nil {
		return fmt.Errorf("gimble: control identity: %w", err)
	}
	identity := hex.EncodeToString(rawID[:])
	socket := filepath.Join(i.dir, identity+".sock")
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
	info := controlDiscovery{PID: os.Getpid(), Socket: socket, Project: i.dir}
	i.Owner.SetControl(identity, socket)
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
		Handler: controlMux(i),
		BaseContext: func(net.Listener) context.Context {
			return host.WithOwner(i.ctx, i.Owner)
		},
	}
	i.shutdown.Add(1)
	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			i.cancel(fmt.Errorf("gimble: serve control socket: %w", err))
		}
	}()
	context.AfterFunc(i.ctx, func() {
		_ = server.Close()
		<-serveDone
		i.Owner.Close()
		_ = os.Remove(socket)
		_ = os.Remove(discovery)
		i.shutdown.Done()
	})
	log.Printf("gimble: control socket listening on %s", socket)
	return nil
}

func controlMux(i *Instance) http.Handler {
	h := controlHandler{instance: i}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := h.project(r)
		if err != nil || p == nil {
			http.Error(w, "project is not admitted", http.StatusNotFound)
			return
		}
		observation.Routes(h).ServeHTTP(w, r.WithContext(p.RequestContext(r.Context())))
	})
}

func controlRuns(r *host.Project) ([]observation.RunRow, error) {
	entries, err := os.ReadDir(filepath.Join(r.Dir(), "runs"))
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
		if _, err := r.Runs().InProgress(entry.Name()); err != nil {
			continue
		}
		snapshot, err := r.Registry().Snapshot(entry.Name())
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
