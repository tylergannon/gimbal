// Package host owns projects and hosted runs independently of web assembly.
package host

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/binding"
	"github.com/tylergannon/gimbal/internal/conversation"
	"github.com/tylergannon/gimbal/internal/environments"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/internal/live"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/internal/promises"
	"github.com/tylergannon/gimbal/internal/runner"
	"github.com/tylergannon/gimbal/internal/runstate"
	"github.com/tylergannon/gimbal/internal/transcript"
	"github.com/tylergannon/gimbal/runstore"
	"golang.org/x/sys/unix"
)

type Owner struct {
	ctx              context.Context
	dir              string
	mu               sync.RWMutex
	projects         map[string]*Project
	order            []string // project paths in the order people arranged them
	activeRuns       sync.WaitGroup
	controlID        string
	socket           string
	pluginDirs       []string
	environmentsOnce sync.Once
	environments     *environments.Service
	environmentsErr  error
}

type Project struct {
	owner         *Owner
	ctx           context.Context
	project       string
	id            string
	dir           string
	runs          *live.Runs
	registry      *observation.Registry
	conversations *conversation.Manager
	recordings    *transcript.DurableStore
	promises      *promises.Store
	stopPromises  context.CancelFunc
	promisesDone  chan struct{}
	ownerLock     *os.File
}

func New(ctx context.Context, instanceDir string) *Owner {
	return &Owner{ctx: ctx, dir: instanceDir, projects: make(map[string]*Project)}
}

// SetControl supplies the discovery endpoint before projects are admitted.
func (o *Owner) SetControl(id, socket string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.controlID, o.socket = id, socket
}

func CanonicalProject(dir string) (string, error) {
	path, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

func projectID(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:16])
}

func (o *Owner) AdmitProject(dir string) (*Project, error) {
	if o == nil || o.ctx.Err() != nil {
		return nil, errors.New("gimbal: instance is closed")
	}
	path, err := CanonicalProject(dir)
	if err != nil {
		return nil, err
	}
	state := filepath.Join(path, ".gimbal")
	if err := os.MkdirAll(state, 0o755); err != nil {
		return nil, err
	}
	placements, err := o.Environments()
	if err != nil {
		return nil, err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.ctx.Err() != nil {
		return nil, errors.New("gimbal: instance is closed")
	}
	if p := o.projects[path]; p != nil {
		return p, nil
	}
	ownerLock, err := os.OpenFile(filepath.Join(state, "owner.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("gimbal: claim project %s: %w", path, err)
	}
	if err := unix.Flock(int(ownerLock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = ownerLock.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, fmt.Errorf("gimbal: project %s is already owned by another instance", path)
		}
		return nil, fmt.Errorf("gimbal: claim project %s: %w", path, err)
	}
	claimed := false
	defer func() {
		if !claimed {
			_ = ownerLock.Close()
		}
	}()
	projectCtx := WithProjectDir(o.ctx, state)
	registry := observation.NewRegistry(state)
	projectCtx = observation.WithRegistry(projectCtx, registry)
	runs := live.NewRuns()

	projectCtx = live.WithRuns(projectCtx, runs)
	p := &Project{owner: o, ctx: projectCtx, project: path, id: projectID(path), dir: state, runs: runs, registry: registry, ownerLock: ownerLock}
	runs.Resolve = p.resolve
	registry.Remote = p.remote
	registry.Placement = p.placement
	cli, err := os.Executable()
	if err != nil {
		return nil, err
	}
	conversations, err := conversation.New(projectCtx, state, binding.Adapter, cli, o.dir, path, registry)
	if err != nil {
		return nil, err
	}
	p.conversations = conversations
	if err := placements.RegisterProject(p.id, path); err != nil {
		conversations.Close()
		return nil, err
	}
	conversations.SetEnvironments(placements, p.id)
	p.recordings, err = transcript.NewDurable(state)
	if err != nil {
		conversations.Close()
		return nil, err
	}
	p.ctx = p.RequestContext(conversation.WithManager(projectCtx, conversations))
	conversations.WorkflowStart = func(ctx context.Context, item conversation.Conversation) error {
		input, _ := json.Marshal(map[string]string{"purpose": conversations.Purpose(item)})
		spec := item.Model
		if item.Effort != "" {
			spec += ":" + item.Effort
		}
		_, err := p.Start(invocation.Request{RunID: item.WorkflowRun, Workflow: "conversation", WorkDir: item.Worktree, Input: input, Roles: map[gimbal.WorkflowRole]string{gimbal.RoleAgenticDialogues: spec}})
		return err
	}
	conversations.WorkflowInput = func(id, address, request string) error {
		controller, err := p.resolve(id)
		if err != nil {
			return runstate.SubmitInteractive(context.Background(), p.dir, id, address, request)
		}
		return controller.SubmitInteractive(address, request)
	}
	conversations.WorkflowObserve = func(id string) (gimbal.InteractiveTranscript, string, error) {
		snapshot, err := p.registry.Snapshot(id)
		if err != nil {
			return gimbal.InteractiveTranscript{}, "unavailable", err
		}
		transcript := gimbal.InteractiveTranscript{Messages: []gimbal.InteractiveMessage{}}
		status := "waiting"
		var verdict error
		rows := make([]observation.InteractiveRow, 0, len(snapshot.Interactions))
		for _, row := range snapshot.Interactions {
			rows = append(rows, row)
		}
		slices.SortFunc(rows, func(a, b observation.InteractiveRow) int { return cmp.Compare(a.Ordinal, b.Ordinal) })
		for _, row := range rows {
			for _, message := range row.Messages {
				transcript.Messages = append(transcript.Messages, gimbal.InteractiveMessage{Role: message.Role, Message: message.Message})
			}
			transcript.EndedBy = row.EndedBy
			if row.Status == "unresolved" {
				status = "unresolved"
				verdict = errors.New(row.Unresolved)
			}
			if row.Status == "runnable" {
				status = "runnable"
			}
			if row.Status == "active" {
				status = "working"
			}
		}
		if snapshot.Run.Status == observation.StatusCompleted {
			status = "completed"
		} else if snapshot.Run.Status == observation.StatusFailed || snapshot.Run.Status == observation.StatusCancelled {
			status = "error"
			verdict = errors.New(snapshot.Run.Error)
		}
		return transcript, status, verdict
	}

	p.promises, err = promises.Open(state)
	if err != nil {
		conversations.Close()
		return nil, err
	}
	model, err := promises.Model(os.Getenv("GIMBAL_PROMISES_MODEL"))
	if err != nil {
		conversations.Close()
		return nil, fmt.Errorf("gimbal: GIMBAL_PROMISES_MODEL: %w", err)
	}
	if err := o.writeProjectDiscovery(path); err != nil {
		conversations.Close()
		return nil, err
	}
	o.projects[path] = p
	o.order = append(o.order, path)
	if err := o.saveProjectsLocked(); err != nil {
		delete(o.projects, path)
		o.order = o.order[:len(o.order)-1]
		conversations.Close()
		return nil, err
	}
	claimed = true
	ctx, stop := context.WithCancel(p.ctx)
	p.stopPromises, p.promisesDone = stop, make(chan struct{})
	go func() { defer close(p.promisesDone); p.promises.Run(ctx, p.recordings, model) }()
	paths, _ := filepath.Glob(filepath.Join(state, "invocations", "*.json"))
	for _, file := range paths {
		raw, e := os.ReadFile(file)
		if e != nil {
			continue
		}
		var request invocation.Request
		if json.Unmarshal(raw, &request) != nil {
			continue
		}
		if snapshot, e := p.registry.Snapshot(request.RunID); e == nil && snapshot.Run.Status != observation.StatusRunning {
			_ = os.Remove(file)
			continue
		}
		go func() {
			for {
				if _, e := runner.Connect(state, request.RunID); e == nil {
					return
				}
				if _, e := runner.Spawn(request); e == nil {
					return
				} else if !errors.Is(e, runstore.ErrBusy) {
					log.Printf("gimbal: recover %s: %v", request.RunID, e)
					return
				}
				select {
				case <-p.ctx.Done():
					return
				case <-time.After(100 * time.Millisecond):
				}
			}
		}()
	}
	return p, nil
}

func (o *Owner) Project(path string) (*Project, error) {
	canonical, err := CanonicalProject(path)
	if err != nil {
		return nil, err
	}
	o.mu.RLock()
	defer o.mu.RUnlock()
	p := o.projects[canonical]
	if p == nil {
		return nil, fmt.Errorf("gimbal: project %s is not admitted", canonical)
	}
	return p, nil
}

func (o *Owner) Projects() []*Project {
	o.mu.RLock()
	defer o.mu.RUnlock()
	projects := make([]*Project, 0, len(o.order))
	for _, path := range o.order {
		projects = append(projects, o.projects[path])
	}
	return projects
}

// ReorderProjects arranges the catalog in the order of ids, which must name
// every admitted project once, and saves it.
func (o *Owner) ReorderProjects(ids []string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	paths := make(map[string]string, len(o.order))
	for _, path := range o.order {
		paths[o.projects[path].id] = path
	}
	if len(ids) != len(paths) {
		return errors.New("gimbal: a project order must list every project once")
	}
	order := make([]string, 0, len(ids))
	for _, id := range ids {
		path, ok := paths[id]
		if !ok {
			return errors.New("gimbal: a project order must list every project once")
		}
		delete(paths, id)
		order = append(order, path)
	}
	previous := o.order
	o.order = order
	if err := o.saveProjectsLocked(); err != nil {
		o.order = previous
		return err
	}
	return nil
}

func (o *Owner) Close() {
	o.activeRuns.Wait()
	for _, p := range o.Projects() {
		p.stopPromises()
		<-p.promisesDone
		p.conversations.Close()
		_ = os.Remove(filepath.Join(p.dir, "control", o.controlID+".json"))
		_ = p.ownerLock.Close()
	}
	o.environmentsOnce.Do(func() { o.environmentsErr = errors.New("gimbal: instance is closed") })
	if o.environments != nil {
		if err := o.environments.Close(); err != nil {
			log.Printf("gimbal: close environment connections: %v", err)
		}
	}
}

func (o *Owner) writeProjectDiscovery(project string) error {
	if o.controlID == "" {
		return nil
	}
	dir := filepath.Join(project, ".gimbal", "control")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	info := struct {
		PID     int    `json:"pid"`
		Socket  string `json:"socket"`
		Project string `json:"project"`
	}{os.Getpid(), o.socket, project}
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, o.controlID+".json"), data, 0o644)
}

func (p *Project) Context() context.Context             { return p.ctx }
func (p *Project) Owner() *Owner                        { return p.owner }
func (p *Project) Path() string                         { return p.project }
func (p *Project) ID() string                           { return p.id }
func (p *Project) Dir() string                          { return p.dir }
func (p *Project) Runs() *live.Runs                     { return p.runs }
func (p *Project) Registry() *observation.Registry      { return p.registry }
func (p *Project) Conversations() *conversation.Manager { return p.conversations }

func (p *Project) RequestContext(ctx context.Context) context.Context {
	ctx = WithProjectDir(ctx, p.dir)
	ctx = observation.WithRegistry(ctx, p.registry)
	ctx = live.WithRuns(ctx, p.runs)
	return conversation.WithManager(ctx, p.conversations)
}

func (p *Project) Run(ctx context.Context, name string, models map[gimbal.WorkflowRole]gimbal.ModelBinding, body func(context.Context) error) (err error) {
	if p == nil {
		return errors.New("gimbal: nil runtime")
	}
	if ctx == nil {
		return errors.New("gimbal: run context is nil")
	}
	if runstate.Override(ctx) != nil {
		return errors.New("gimbal: application runs require the project-configured run store")
	}
	if body == nil {
		return errors.New("gimbal: run body is nil")
	}
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("gimbal: hosted workflow panic: %v", value)
		}
	}()
	p.owner.mu.Lock()
	if p.owner.ctx.Err() != nil {
		p.owner.mu.Unlock()
		return context.Cause(p.owner.ctx)
	}
	p.owner.activeRuns.Add(1)
	p.owner.mu.Unlock()
	defer p.owner.activeRuns.Done()
	runCtx, cancel := context.WithCancelCause(ctx)
	stop := context.AfterFunc(p.owner.ctx, func() { cancel(context.Cause(p.owner.ctx)) })
	defer stop()
	defer cancel(nil)
	runCtx = observation.WithRegistry(runCtx, p.registry)
	runCtx = live.WithRuns(runCtx, p.runs)
	runCtx = live.WithHook(runCtx, p.runs.Hook)
	id := string(runstore.NewRunID())
	for {
		err = gimbal.Run(gimbal.Project(runCtx, p.dir), id, name, models, body)
		if err != nil {
			break
		}
		controller, e := p.runs.InProgress(id)
		if e != nil {
			break
		}
		durable, ok := controller.(interface {
			Runnable() bool
			State() string
		})
		if !ok || durable.State() != "waiting" {
			break
		}
		for !durable.Runnable() {
			select {
			case <-runCtx.Done():
				_ = controller.CancelScope("", context.Cause(runCtx))
			case <-time.After(100 * time.Millisecond):
			}
			if runCtx.Err() != nil {
				break
			}
		}
	}

	if err == nil && context.Cause(runCtx) != nil {
		return context.Cause(runCtx)
	}
	return err
}

// Start validates before admitting execution, then receives the child's run ID.
func (p *Project) Start(request invocation.Request) (string, error) {
	if p.owner.ctx.Err() != nil {
		return "", context.Cause(p.owner.ctx)
	}
	request.Project = p.project
	if request.WorkflowRoute.Name == "" || request.WorkflowRoute.Name == "local" {
		request.PluginDirs = append([]string(nil), p.owner.pluginDirs...)
	}
	if request.WorkDir == "" {
		request.WorkDir = p.project
	}
	if !filepath.IsAbs(request.WorkDir) {
		return "", errors.New("work_dir must be absolute")
	}
	if request.WorkflowRoute.Name == "" || request.WorkflowRoute.Name == "local" {
		if info, err := os.Stat(request.WorkDir); err != nil || !info.IsDir() {
			return "", errors.New("work_dir must be an existing directory")
		}
	}
	if request.Conversation != "" {
		worktree, ok := p.conversations.Worktree(request.Conversation)
		if !ok || worktree != request.WorkDir {
			return "", errors.New("conversation does not belong to this project and worktree")
		}
	}
	if _, _, err := invocation.Prepare(request); err != nil {
		return "", err
	}
	id, err := p.startRouted(request)
	if err != nil {
		return "", err
	}
	if request.Conversation != "" {
		if err := p.conversations.Associate(request.Conversation, id, request.Workflow, request.WorkDir); err != nil {
			return "", err
		}
	}
	return id, nil
}

func (p *Project) Steer(ctx context.Context, runID, sessionID, message string) (bool, error) {
	run, err := p.runs.InProgress(runID)
	if err != nil {
		return false, err
	}
	return run.Steer(ctx, sessionID, message)
}

func (p *Project) SteerLoop(runID, scopeKey, message string) error {
	run, err := p.runs.InProgress(runID)
	if err != nil {
		return err
	}
	return run.SteerLoop(scopeKey, message)
}

func (p *Project) KillScope(runID, scopeKey, by, reason string) error {
	run, err := p.runs.InProgress(runID)
	if err != nil {
		return err
	}
	return run.CancelScope(scopeKey, gimbal.Killed{Target: scopeKey, By: by, Reason: reason})
}

func (p *Project) KillTurn(runID, turnID, by, reason string) error {
	run, err := p.runs.InProgress(runID)
	if err != nil {
		return err
	}
	return run.CancelTurn(turnID, gimbal.Killed{Target: turnID, By: by, Reason: reason})
}

func (o *Owner) SetPluginDirs(dirs []string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.pluginDirs = append([]string(nil), dirs...)
}

func (p *Project) Recordings() *transcript.DurableStore { return p.recordings }
func (p *Project) Promises() *promises.Store            { return p.promises }
