package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/internal/observation"
)

// WorkflowDaemon admits independent runners in its own configured environment.
// Its run index belongs to A, separately from central's route sidecars.
type WorkflowDaemon struct {
	Root       string
	PluginDirs []string
	mu         sync.Mutex
	projects   map[string]string
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
		return "", fmt.Errorf("workflow not found")
	}
	return project, nil
}
func (d *WorkflowDaemon) admit(v backend.Workflow) backend.WorkflowStatus {
	status := backend.WorkflowStatus{ID: v.ID}
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
	if _, exists := d.projects[v.ID]; exists {
		d.mu.Unlock()
		status.Error = "workflow identity already admitted"
		return status
	}
	// Record the assigned identity before spawning; duplicate requests never run twice.
	d.projects[v.ID] = project
	d.mu.Unlock()
	response, err := Spawn(request)
	if err != nil {
		status.State = "unknown"
		status.Error = err.Error()
		return status
	}
	status.ID = response.RunID
	status.State = "running"
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			done, _ := CleanupExited(filepath.Join(project, ".gimbal"), v.ID)
			if done {
				return
			}
			snap, err := observation.NewRegistry(filepath.Join(project, ".gimbal")).Snapshot(v.ID)
			if err == nil && snap.Run.Status != observation.StatusRunning && !cleanupPending(snap.Run) {
				return
			}
		}
	}()
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
			status.CleanupPending = cleanupPending(snap.Run)
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
	AnswerInterview(string, string) error
	Steer(context.Context, string, string) (bool, error)
	SteerLoop(string, string) error
	CancelScope(string, error) error
	CancelTurn(string, error) error
	CleanupPending() bool
}

func control(ctx context.Context, c interfaceController, v backend.Control) (bool, error) {
	cause := gimbal.Killed{Target: v.Target, By: v.By, Reason: v.Message}
	switch v.Action {
	case "answer":
		return false, c.AnswerInterview(v.Target, v.Message)
	case "steer":
		return c.Steer(ctx, v.Target, v.Message)
	case "loop":
		return false, c.SteerLoop(v.Target, v.Message)
	case "scope":
		return false, c.CancelScope(v.Target, cause)
	case "turn":
		return false, c.CancelTurn(v.Target, cause)
	case "cleanup":
		return c.CleanupPending(), nil
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
	if err != nil || (v.Name != "graph.json" && (!strings.HasPrefix(v.Name, "artifacts/") || !observation.ReferencedArtifact(snapshot, v.Name))) {
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

func cleanupPending(row observation.RunRow) bool {
	return row.Cancellation.Present && row.Cancellation.Value.Cleanup != "completed"
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
