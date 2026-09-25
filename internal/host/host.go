// Package host owns projects and hosted runs independently of web assembly.
package host

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/binding"
	"github.com/tylergannon/gimbal/internal/conversation"
	"github.com/tylergannon/gimbal/internal/live"
	"github.com/tylergannon/gimbal/internal/observation"
)

type Owner struct {
	ctx        context.Context
	dir        string
	mu         sync.RWMutex
	projects   map[string]*Project
	activeRuns sync.WaitGroup
	controlID  string
	socket     string
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
	cli, err := os.Executable()
	if err != nil {
		return nil, err
	}
	conversations, err := conversation.New(projectCtx, state, binding.Adapter, cli, o.dir, path, registry)
	if err != nil {
		return nil, err
	}
	p.ctx = conversation.WithManager(projectCtx, conversations)
	p.conversations = conversations
	if err := o.writeProjectDiscovery(path); err != nil {
		conversations.Close()
		return nil, err
	}
	o.projects[path] = p
	claimed = true
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
	projects := make([]*Project, 0, len(o.projects))
	for _, p := range o.projects {
		projects = append(projects, p)
	}
	return projects
}

func (o *Owner) Close() {
	o.activeRuns.Wait()
	for _, p := range o.Projects() {
		p.conversations.Close()
		_ = os.Remove(filepath.Join(p.dir, "control", o.controlID+".json"))
		_ = p.ownerLock.Close()
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
	hook := p.runs.Hook
	if started, _ := ctx.Value(runStartedKey{}).(func(string)); started != nil {
		hook = func(id string, run live.Controller) func() {
			release := p.runs.Hook(id, run)
			started(id)
			return release
		}
	}
	runCtx = live.WithHook(runCtx, hook)
	err = gimbal.Run(gimbal.Project(runCtx, p.dir), name, models, body)
	if err == nil && context.Cause(runCtx) != nil {
		return context.Cause(runCtx)
	}
	return err
}

type runStartedKey struct{}

var ErrStartFailed = errors.New("run could not start")
var ErrStopped = errors.New("instance stopped")

// Start registers a server-owned run before returning its ID. The supplied
// body is a concrete workflow call; request cancellation cannot end it.
func (p *Project) Start(name, workdir, conversationID string, models map[gimbal.WorkflowRole]gimbal.ModelBinding, body func(context.Context) error) (string, error) {
	if !filepath.IsAbs(workdir) {
		return "", errors.New("work_dir must be absolute")
	}
	if info, err := os.Stat(workdir); err != nil || !info.IsDir() {
		return "", errors.New("work_dir must be an existing directory")
	}
	if conversationID != "" {
		worktree, ok := p.conversations.Worktree(conversationID)
		if !ok || worktree != workdir {
			return "", errors.New("conversation does not belong to this project and worktree")
		}
	}
	started := make(chan string, 1)
	ctx := context.WithValue(p.ctx, runStartedKey{}, func(id string) { started <- id })
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx, name, models, body) }()
	var id string
	finished := false
	select {
	case id = <-started:
	case err := <-done:
		finished = true
		select {
		case id = <-started:
		default:
		}
		if id == "" {
			return "", fmt.Errorf("%w: %v", ErrStartFailed, err)
		}
	case <-p.ctx.Done():
		return "", ErrStopped
	}
	if conversationID != "" {
		if err := p.conversations.Associate(conversationID, id, name, workdir); err != nil {
			return "", fmt.Errorf("%w: %v", ErrStartFailed, err)
		}
		if finished {
			p.conversations.RefreshRun(id)
		} else {
			go func() { <-done; p.conversations.RefreshRun(id) }()
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
