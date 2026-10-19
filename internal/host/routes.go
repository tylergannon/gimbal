package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/internal/live"
	"github.com/tylergannon/gimbal/internal/runner"
)

type routeRecord struct {
	ID, Workflow, Admission  string
	WorkflowRoute, TaskRoute backend.Route
	WorkflowProject, Workdir string
}

func (p *Project) routePath(id string) (string, error) {
	if id == "" || filepath.Base(id) != id || id == "." || id == ".." {
		return "", fmt.Errorf("invalid run ID")
	}
	return filepath.Join(p.dir, "routes", id+".json"), nil
}
func (p *Project) readRoute(id string) (routeRecord, error) {
	path, err := p.routePath(id)
	if err != nil {
		return routeRecord{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return routeRecord{}, err
	}
	var record routeRecord
	err = json.Unmarshal(raw, &record)
	return record, err
}
func (p *Project) writeRoute(record routeRecord) error {
	path, err := p.routePath(record.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, raw, 0600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
func (p *Project) remote(id string) (backend.Connection, error) {
	record, err := p.readRoute(id)
	if errors.Is(err, os.ErrNotExist) {
		return backend.Connection{}, nil
	}
	if err != nil {
		return backend.Connection{}, err
	}
	if record.WorkflowRoute.Name == "" || record.WorkflowRoute.Name == "local" {
		return backend.Connection{}, nil
	}
	return gimbal.DefaultRegistry.Connect(p.ctx, record.WorkflowRoute)
}
func (p *Project) resolve(id string) (live.Controller, error) {
	connection, err := p.remote(id)
	if err != nil {
		return nil, err
	}
	if connection.Execution == nil {
		return runner.Connect(p.dir, id)
	}
	return routedController{id: id, execution: connection.Execution}, nil
}
func (p *Project) placement(id string) (string, string) {
	record, err := p.readRoute(id)
	if err != nil {
		return "", ""
	}
	workflow, tasks := record.WorkflowRoute.Label, record.TaskRoute.Label
	if record.TaskRoute.Environment != "" {
		tasks = record.TaskRoute.Environment
	}
	if workflow == "" {
		workflow = "local"
	}
	if tasks == "" {
		tasks = workflow
	}
	return workflow, tasks
}
func (p *Project) startRouted(request invocation.Request) (string, error) {
	if request.RunID == "" {
		request.RunID = ulid.Make().String()
	}
	if _, err := ulid.ParseStrict(request.RunID); err != nil {
		return "", fmt.Errorf("gimbal: invalid run ID: %w", err)
	}
	record := routeRecord{ID: request.RunID, Workflow: request.Workflow, Admission: "pending", WorkflowRoute: request.WorkflowRoute, TaskRoute: request.TaskRoute, WorkflowProject: request.WorkflowProject, Workdir: request.WorkDir}
	if record.WorkflowRoute.Name == "" {
		record.WorkflowRoute.Name = "local"
	}
	if request.TaskEndpoint != "" && record.TaskRoute.Name == "" {
		raw, _ := json.Marshal(map[string]string{"endpoint": request.TaskEndpoint})
		record.TaskRoute = backend.Route{Name: "http", Configuration: raw, Label: request.TaskEndpoint}
	}
	if err := p.writeRoute(record); err != nil {
		return "", err
	}
	if record.WorkflowRoute.Name == "local" {
		response, err := runner.Spawn(request)
		if err != nil {
			record.Admission = "unknown"
			if response.Error != "" {
				record.Admission = "failed"
			}
			_ = p.writeRoute(record)
			return "", err
		}
		record.Admission = "accepted"
		if err := p.writeRoute(record); err != nil {
			return response.RunID, err
		}
		return response.RunID, nil
	}
	connection, err := gimbal.DefaultRegistry.Connect(p.ctx, record.WorkflowRoute)
	if err != nil {
		record.Admission = "failed"
		_ = p.writeRoute(record)
		return "", err
	}
	if connection.Execution == nil {
		record.Admission = "failed"
		_ = p.writeRoute(record)
		return "", fmt.Errorf("backend %s has no execution", record.WorkflowRoute.Name)
	}
	roles := map[string]string{}
	for role, spec := range request.Roles {
		roles[string(role)] = spec
	}
	status, err := connection.Execution.StartWorkflow(p.ctx, backend.Workflow{ID: request.RunID, Name: request.Workflow, Project: request.WorkflowProject, Workdir: request.WorkDir, Input: request.Input, TaskRoute: record.TaskRoute, Roles: roles})
	if err != nil {
		// Reconcile assigned identity without replaying an ambiguous mutation.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		for ctx.Err() == nil {
			known, e := connection.Execution.WorkflowStatus(ctx, backend.Target{Run: request.RunID, ID: request.RunID})
			if e == nil && known.Error == "" && known.State != "unknown" && known.State != "" {
				status = known
				err = nil
				break
			}
			select {
			case <-ctx.Done():
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	if err == nil && status.Error != "" {
		err = errors.New(status.Error)
	}
	if err != nil {
		record.Admission = "unknown"
		if status.State == "failed" {
			record.Admission = "failed"
		}
		_ = p.writeRoute(record)
		return "", err
	}
	record.Admission = "accepted"
	if err := p.writeRoute(record); err != nil {
		return request.RunID, err
	}
	return request.RunID, nil
}

type routedController struct {
	id        string
	execution backend.Execution
}

func (c routedController) call(ctx context.Context, action, target, message, by string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.execution.ControlWorkflow(ctx, backend.Control{Run: c.id, Action: action, Target: target, Message: message, By: by})
}
func (c routedController) AnswerInterview(id, answer string) error {
	_, err := c.call(context.Background(), "answer", id, answer, "")
	return err
}
func (c routedController) Steer(ctx context.Context, id, message string) (bool, error) {
	return c.call(ctx, "steer", id, message, "")
}
func (c routedController) SteerLoop(id, message string) error {
	_, err := c.call(context.Background(), "loop", id, message, "")
	return err
}
func (c routedController) cancel(action, id string, cause error) error {
	message, by := cause.Error(), ""
	if k, ok := errors.AsType[gimbal.Killed](cause); ok {
		message, by = k.Reason, k.By
	}
	_, err := c.call(context.Background(), action, id, message, by)
	return err
}
func (c routedController) CancelScope(id string, cause error) error {
	return c.cancel("scope", id, cause)
}
func (c routedController) CancelTurn(id string, cause error) error {
	return c.cancel("turn", id, cause)
}

// RestoreProjects restores only the central's retained catalog, without submitting work.
func (o *Owner) RestoreProjects() error {
	raw, err := os.ReadFile(filepath.Join(o.dir, "projects.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var paths []string
	if err := json.Unmarshal(raw, &paths); err != nil {
		return err
	}
	for _, path := range paths {
		if _, err := o.AdmitProject(path); err != nil {
			log.Printf("gimbal: skipped saved project %q: %v", path, err)
			continue
		}
	}
	return nil
}
func (o *Owner) saveProjectsLocked() error {
	if o.dir == "" {
		return nil
	}
	if err := os.MkdirAll(o.dir, 0700); err != nil {
		return err
	}
	raw, err := json.Marshal(o.order)
	if err != nil {
		return err
	}
	temporary := filepath.Join(o.dir, "projects.json.tmp")
	if err := os.WriteFile(temporary, raw, 0600); err != nil {
		return err
	}
	return os.Rename(temporary, filepath.Join(o.dir, "projects.json"))
}
