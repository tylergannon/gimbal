package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/executor"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/runstore"
)

// WorkflowDaemon admits independent runners in its own configured environment.
// Its durable run index is separate from central's route sidecars.
type WorkflowDaemon struct {
	StateDir   string
	Root       string
	PluginDirs []string
	mu         sync.Mutex
	projects   map[string]string
	requests   map[string]invocation.Request
	recovering map[string]bool
	registries map[string]*observation.Registry
}

func (d *WorkflowDaemon) project(id string) (string, error) {
	if id == "" || filepath.Base(id) != id || strings.Contains(id, "..") {
		return "", fmt.Errorf("invalid run ID")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	project := d.projects[id]
	if project == "" {
		var request invocation.Request
		if err := executor.Read(filepath.Join(d.stateDir(), id+".json"), &request); err == nil && request.RunID == id {
			if d.projects == nil {
				d.projects = map[string]string{}
			}
			if d.requests == nil {
				d.requests = map[string]invocation.Request{}
			}
			d.projects[id], d.requests[id], project = request.Project, request, request.Project
		}
	}
	if project == "" {
		return "", fmt.Errorf("workflow not found")
	}
	return project, nil
}
func (d *WorkflowDaemon) admit(v backend.Workflow) backend.WorkflowStatus {
	status := backend.WorkflowStatus{ID: v.ID, State: "failed"}
	if v.ID == "" || filepath.Base(v.ID) != v.ID || strings.Contains(v.ID, "..") {
		status.Error = "invalid run ID"
		return status
	}
	project, err := filepath.Abs(v.Project)
	if err != nil || !filepath.IsAbs(v.Project) {
		status.Error = "workflow project must be absolute"
		return status
	}
	if d.Root != "" {
		root, err := filepath.EvalSymlinks(d.Root)
		if err != nil {
			status.Error = err.Error()
			return status
		}
		canonical, err := filepath.EvalSymlinks(project)
		if err != nil {
			status.Error = err.Error()
			return status
		}
		rel, err := filepath.Rel(root, canonical)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			status.Error = "workflow project is outside daemon workspace"
			return status
		}
		project = canonical
	}
	if !filepath.IsAbs(v.Workdir) {
		status.Error = "workdir must be absolute"
		return status
	}
	if info, err := os.Stat(v.Workdir); err != nil || !info.IsDir() {
		status.Error = "workdir must exist in workflow environment"
		return status
	}
	request := invocation.Request{RunID: v.ID, Project: project, Workflow: v.Name, WorkDir: v.Workdir, Input: v.Input, TaskRoute: v.TaskRoute, PluginDirs: d.PluginDirs, Roles: map[gimbal.WorkflowRole]string{}}
	for role, spec := range v.Roles {
		request.Roles[gimbal.WorkflowRole(role)] = spec
	}
	if _, _, err := invocation.Prepare(request); err != nil {
		status.Error = err.Error()
		return status
	}
	d.mu.Lock()
	if d.projects == nil {
		d.projects = map[string]string{}
	}
	if d.requests == nil {
		d.requests = map[string]invocation.Request{}
	}
	if _, exists := d.projects[v.ID]; exists {
		prior, _ := json.Marshal(d.requests[v.ID])
		next, _ := json.Marshal(request)
		d.mu.Unlock()
		if string(prior) != string(next) {
			status.Error = "workflow identity conflicts with admitted request"
			return status
		}
		status.State = "running"
		if snapshot, e := observation.NewRegistry(filepath.Join(project, ".gimbal")).Snapshot(v.ID); e == nil {
			status.State = snapshot.Run.Status
		}
		return status
	}
	if err := os.MkdirAll(d.stateDir(), 0700); err != nil {
		d.mu.Unlock()
		status.Error = err.Error()
		return status
	}
	if err := executor.Write(filepath.Join(d.stateDir(), v.ID+".json"), request); err != nil {
		d.mu.Unlock()
		status.Error = err.Error()
		return status
	}
	// Commit the assigned invocation before spawning, including its original route.
	d.projects[v.ID], d.requests[v.ID] = project, request
	d.mu.Unlock()
	response, err := Spawn(request)
	if err != nil {
		status.State = "unknown"
		if response.Error != "" {
			status.State = "failed"
		}
		status.Error = err.Error()
		return status
	}
	status.ID = response.RunID
	status.State = "running"
	return status
}

func (d *WorkflowDaemon) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /workflows/start", func(w http.ResponseWriter, r *http.Request) {
		var v backend.Workflow
		if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		_ = json.NewEncoder(w).Encode(d.admit(v))
	})
	mux.HandleFunc("POST /workflows/status", func(w http.ResponseWriter, r *http.Request) {
		var target backend.Target
		if err := json.NewDecoder(r.Body).Decode(&target); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		project, err := d.project(target.Run)
		status := backend.WorkflowStatus{ID: target.Run}
		if err != nil {
			status.Error = err.Error()
			_ = json.NewEncoder(w).Encode(status)
			return
		}
		snap, err := observation.NewRegistry(filepath.Join(project, ".gimbal")).Snapshot(target.Run)
		if err != nil {
			status.State = "unknown"
			status.Error = err.Error()
		} else {
			status.State = snap.Run.Status
		}
		_ = json.NewEncoder(w).Encode(status)
	})
	mux.HandleFunc("POST /workflows/control", func(w http.ResponseWriter, r *http.Request) {
		var v backend.Control
		if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		project, err := d.project(v.Run)
		reply := backend.Reply{}
		if err == nil {
			var c interfaceController
			c, err = Connect(filepath.Join(project, ".gimbal"), v.Run)
			if err == nil {
				reply.Value, err = control(r.Context(), c, v)
			}
		}
		if err != nil {
			reply.Error = err.Error()
		}
		_ = json.NewEncoder(w).Encode(reply)
	})
	mux.HandleFunc("POST /workflows/observation", d.observation)
	mux.HandleFunc("POST /workflows/bytes", d.bytes)
	return mux
}

type interfaceController interface {
	SubmitInteractive(string, string) error
	Steer(context.Context, string, string) (bool, error)
	SteerLoop(string, string) error
	CancelScope(string, error) error
	CancelTurn(string, error) error
}

func control(ctx context.Context, c interfaceController, v backend.Control) (bool, error) {
	cause := gimbal.Killed{Target: v.Target, By: v.By, Reason: v.Message}
	switch v.Action {
	case "interactive":
		return false, c.SubmitInteractive(v.Target, v.Message)
	case "steer":
		return c.Steer(ctx, v.Target, v.Message)
	case "loop":
		return false, c.SteerLoop(v.Target, v.Message)
	case "scope":
		return false, c.CancelScope(v.Target, cause)
	case "turn":
		return false, c.CancelTurn(v.Target, cause)
	case "live":
		return true, nil
	default:
		return false, fmt.Errorf("unknown workflow control %q", v.Action)
	}
}
func (d *WorkflowDaemon) observation(w http.ResponseWriter, r *http.Request) {
	var v struct {
		backend.Target
		Cursor string
	}
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	project, err := d.project(v.Run)
	if err != nil {
		_ = json.NewEncoder(w).Encode(backend.Observation{Unavailable: true})
		return
	}
	registry := d.registry(project)
	snap, err := registry.Snapshot(v.Run)
	if err != nil {
		_ = json.NewEncoder(w).Encode(backend.Observation{Unavailable: true})
		return
	}
	out := backend.Observation{Cursor: snap.Stream + ":" + strconv.FormatUint(snap.Position, 10)}
	stream, positionText, _ := strings.Cut(v.Cursor, ":")
	position, _ := strconv.ParseUint(positionText, 10, 64)
	if store, live := registry.Live(v.Run); live {
		replacement, suffix, sub, _ := store.Join(stream, position)
		if sub != nil {
			sub.Close()
		}
		if replacement != nil {
			out.Snapshot, _ = json.Marshal(replacement)
			out.Cursor = replacement.Stream + ":" + strconv.FormatUint(replacement.Position, 10)
			out.Gap = v.Cursor != ""
		} else {
			for _, delta := range suffix {
				raw, _ := json.Marshal(delta)
				out.Deltas = append(out.Deltas, raw)
				out.Cursor = delta.Stream + ":" + strconv.FormatUint(delta.Position, 10)
			}
		}
	} else if v.Cursor != out.Cursor {
		out.Snapshot, _ = json.Marshal(snap)
		out.Gap = v.Cursor != ""
	}
	_ = json.NewEncoder(w).Encode(out)
}
func (d *WorkflowDaemon) bytes(w http.ResponseWriter, r *http.Request) {
	var v backend.Artifact
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	project, err := d.project(v.Run)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	root, err := os.OpenRoot(filepath.Join(project, ".gimbal", "runs", v.Run))
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	defer func() { _ = root.Close() }()
	snapshot, err := d.registry(project).Snapshot(v.Run)
	if err != nil || (v.Name != "graph.json" && v.Name != "source.json" && (!strings.HasPrefix(v.Name, "artifacts/") || !observation.ReferencedArtifact(snapshot, v.Name))) {
		http.Error(w, "unreferenced artifact", 404)
		return
	}
	file, err := root.Open(v.Name)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.Error(w, "not an artifact", 404)
		return
	}
	_, _ = io.Copy(w, file)
}

func (d *WorkflowDaemon) registry(project string) *observation.Registry {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.registries == nil {
		d.registries = map[string]*observation.Registry{}
	}
	r := d.registries[project]
	if r == nil {
		r = observation.NewRegistry(filepath.Join(project, ".gimbal"))
		d.registries[project] = r
	}
	return r
}

func (d *WorkflowDaemon) stateDir() string {
	if d.StateDir != "" {
		return d.StateDir
	}
	if d.Root != "" {
		return filepath.Join(d.Root, ".gimbal", "workflow-daemon")
	}
	root, _ := executor.Root("")
	return filepath.Join(root, "workflows")
}

// Recover discovers accepted invocations before serving ingress. Journaled inbox
// input remains runnable even if its sender died before receiving a response.
func (d *WorkflowDaemon) Recover(ctx context.Context) error {
	files, err := filepath.Glob(filepath.Join(d.stateDir(), "*.json"))
	if err != nil {
		return err
	}
	d.mu.Lock()
	if d.projects == nil {
		d.projects = map[string]string{}
	}
	if d.requests == nil {
		d.requests = map[string]invocation.Request{}
	}
	for _, file := range files {
		var request invocation.Request
		if err := executor.Read(file, &request); err != nil {
			d.mu.Unlock()
			return err
		}
		if !runstore.ValidID(request.RunID) || filepath.Base(file) != request.RunID+".json" || !filepath.IsAbs(request.Project) {
			d.mu.Unlock()
			return fmt.Errorf("invalid durable workflow invocation %s", file)
		}
		d.projects[request.RunID], d.requests[request.RunID] = request.Project, request
	}
	ids := make([]string, 0, len(d.requests))
	for id := range d.requests {
		ids = append(ids, id)
	}
	d.mu.Unlock()
	for _, id := range ids {
		d.recoverRun(ctx, id)
	}
	return nil
}
func (d *WorkflowDaemon) recoverRun(ctx context.Context, id string) {
	d.mu.Lock()
	request, ok := d.requests[id]
	if d.recovering == nil {
		d.recovering = map[string]bool{}
	}
	if !ok || d.recovering[id] {
		d.mu.Unlock()
		return
	}
	d.recovering[id] = true
	d.mu.Unlock()
	go func() {
		defer func() { d.mu.Lock(); delete(d.recovering, id); d.mu.Unlock() }()
		state := filepath.Join(request.Project, ".gimbal")
		if ctx.Err() != nil {
			return
		}
		if _, err := Connect(state, id); err == nil {
			return
		}
		if snapshot, err := observation.NewRegistry(state).Snapshot(id); err == nil && snapshot.Run.Status != observation.StatusRunning {
			return
		}
		if _, err := Spawn(request); err != nil {
			log.Printf("gimbal: startup remote run %s unavailable: %v", id, err)
		}
	}()
}
