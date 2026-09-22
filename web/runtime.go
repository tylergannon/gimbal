package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sys/unix"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/binding"
	"github.com/tylergannon/gimble/internal/conversation"
	"github.com/tylergannon/gimble/internal/live"
	"github.com/tylergannon/gimble/internal/observation"
	hooks "github.com/tylergannon/gimble/web/src"
)

// Runtime is one admitted project's runs and conversations. Its Instance owns
// the listeners and remains active until its startup context is cancelled.
type Runtime struct {
	instance *Instance
	ctx      context.Context
	project  string
	id       string
	dir      string

	// runs in progress by id, for Steer, KillScope, and KillTurn. The table
	// is in the runtime's context too, so the page's remote functions steer
	// the same runs these methods do.
	runs *live.Runs
	// registry is where every run the runtime starts registers its store.
	// Run puts it on the caller's context, which is the run's context.
	registry *observation.Registry
	// conversations owns the chat worktrees and live harness sessions for the
	// same lifetime as this runtime.
	conversations *conversation.Manager
	ownerLock     *os.File
}

// Instance owns one web listener, one control socket, and any number of
// admitted projects. Project state and durable files live below each project's
// directory; instanceDir holds only the control endpoint and discovery.
type Instance struct {
	ctx           context.Context
	cancel        context.CancelCauseFunc
	dir           string
	done          chan struct{}
	shutdown      sync.WaitGroup
	address       string
	mu            sync.RWMutex
	projects      map[string]*Runtime
	workflows     map[string]WorkflowEntry
	controlSocket string
	controlID     string
}

// WorkflowEntry executes one built-in workflow with its submitted parameters.
type WorkflowEntry func(context.Context, gimble.Env, json.RawMessage) error

// WithWorkflows makes the binary's built-in entries available to admitted projects.
func WithWorkflows(entries map[string]WorkflowEntry) Option {
	return func(c *config) error {
		c.workflows = entries
		return nil
	}
}

// Option configures the instance's web listener.
type Option func(*config) error

type config struct {
	workflows map[string]WorkflowEntry
	network   string
	port      int
	uds       string
	explicit  string
}

// WithPort serves the web application on the given loopback TCP port. Port 0
// asks the operating system for an available port.
func WithPort(port int) Option {
	return func(c *config) error {
		if port < 0 || port > 65535 {
			return fmt.Errorf("gimble: invalid web port %d", port)
		}
		if err := c.selectListener("port"); err != nil {
			return err
		}
		c.network, c.port = "tcp", port
		return nil
	}
}

// WithUDS serves the web application on a Unix-domain socket at path.
func WithUDS(path string) Option {
	return func(c *config) error {
		if strings.TrimSpace(path) == "" {
			return errors.New("gimble: UDS path must not be blank")
		}
		if err := c.selectListener("UDS"); err != nil {
			return err
		}
		c.network, c.uds = "unix", path
		return nil
	}
}

// WithNoWeb disables the web listener. The control socket, runs, and their
// durable records remain available.
func WithNoWeb() Option {
	return func(c *config) error {
		if err := c.selectListener("no web"); err != nil {
			return err
		}
		c.network = "none"
		return nil
	}
}

func (c *config) selectListener(name string) error {
	if c.explicit != "" {
		return fmt.Errorf("gimble: runtime listener options %q and %q conflict", c.explicit, name)
	}
	c.explicit = name
	return nil
}

// NewInstance starts one instance with independently configurable state and
// endpoints. AdmitProject adds projects without opening another listener.
// The web application listens on loopback port 8080 unless configured otherwise.
func NewInstance(ctx context.Context, instanceDir string, opts ...Option) (*Instance, error) {
	if ctx == nil {
		return nil, errors.New("gimble: runtime context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dir, err := filepath.Abs(instanceDir)
	if err != nil {
		return nil, fmt.Errorf("gimble: instance directory: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("gimble: instance directory: %w", err)
	}
	cfg := config{network: "tcp", port: 8080}
	for _, option := range opts {
		if option == nil {
			return nil, errors.New("gimble: nil runtime option")
		}
		if err := option(&cfg); err != nil {
			return nil, err
		}
	}
	runtimeCtx, cancel := context.WithCancelCause(ctx)
	instance := &Instance{
		ctx: runtimeCtx, cancel: cancel, dir: dir, done: make(chan struct{}), projects: make(map[string]*Runtime), workflows: cfg.workflows,
	}
	if err := instance.startControl(); err != nil {
		cancel(err)
		return nil, err
	}
	if cfg.network == "none" {
		instance.waitForShutdown()
		return instance, nil
	}
	if err := instance.startWeb(cfg); err != nil {
		cancel(err)
		instance.shutdown.Wait()
		return nil, err
	}
	instance.waitForShutdown()
	return instance, nil
}

// NewRuntime starts an instance for a repository. Its records and instance
// discovery live in that repository's .gimble directory.
func NewRuntime(ctx context.Context, projectDir string, opts ...Option) (*Runtime, error) {
	project, err := canonicalProject(projectDir)
	if err != nil {
		return nil, err
	}
	i, err := NewInstance(ctx, filepath.Join(project, ".gimble"), opts...)
	if err != nil {
		return nil, err
	}
	p, err := i.AdmitProject(project)
	if err != nil {
		i.cancel(err)
		<-i.done
		return nil, err
	}
	return p, nil
}

// AdmitProject admits a repository once. Another instance cannot admit the
// same canonical project until this instance ends. Its .gimble state and the
// execution workdirs supplied by workflows are separate from its identity.
func (i *Instance) AdmitProject(dir string) (*Runtime, error) {
	if i == nil || i.ctx.Err() != nil {
		return nil, errors.New("gimble: instance is closed")
	}
	path, err := canonicalProject(dir)
	if err != nil {
		return nil, err
	}
	state := filepath.Join(path, ".gimble")
	if err := os.MkdirAll(state, 0o755); err != nil {
		return nil, err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.ctx.Err() != nil {
		return nil, errors.New("gimble: instance is closed")
	}
	if p := i.projects[path]; p != nil {
		return p, nil
	}
	// Keep the file itself after release. Removing it would let a new owner
	// lock a different inode while another contender still holds the old one.
	ownerLock, err := os.OpenFile(filepath.Join(state, "owner.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("gimble: claim project %s: %w", path, err)
	}
	if err := unix.Flock(int(ownerLock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = ownerLock.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, fmt.Errorf("gimble: project %s is already owned by another instance", path)
		}
		return nil, fmt.Errorf("gimble: claim project %s: %w", path, err)
	}
	claimed := false
	defer func() {
		if !claimed {
			_ = ownerLock.Close()
		}
	}()
	projectCtx := hooks.WithProjectDir(i.ctx, state)
	registry := observation.NewRegistry(state)
	projectCtx = observation.WithRegistry(projectCtx, registry)
	runs := live.NewRuns()
	projectCtx = live.WithRuns(projectCtx, runs)
	p := &Runtime{instance: i, ctx: projectCtx, project: path, id: projectID(path), dir: state, runs: runs, registry: registry, ownerLock: ownerLock}
	cli, err := os.Executable()
	if err != nil {
		return nil, err
	}
	conversations, err := conversation.New(projectCtx, state, binding.Adapter, cli, i.dir, path, registry)
	if err != nil {
		return nil, err
	}
	p.ctx = conversation.WithManager(projectCtx, conversations)
	p.conversations = conversations
	if err := i.writeProjectDiscovery(path); err != nil {
		return nil, err
	}
	i.projects[path] = p
	claimed = true
	return p, nil
}

func canonicalProject(dir string) (string, error) {
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

func (i *Instance) startWeb(cfg config) error {
	address := cfg.uds
	if cfg.network == "tcp" {
		address = net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.port))
	}
	listener, err := net.Listen(cfg.network, address)
	if err != nil {
		return fmt.Errorf("gimble: listen on %s: %w", address, err)
	}
	i.shutdown.Add(1)
	origin := ""
	if cfg.network == "tcp" {
		origin = "http://" + listener.Addr().String()
	}
	if configured := os.Getenv("GIMBLE_WEB_ORIGIN"); configured != "" {
		origin = configured
	}
	dist, err := fs.Sub(Build, "build")
	if err != nil {
		_ = listener.Close()
		i.shutdown.Done()
		return fmt.Errorf("gimble: web application: %w", err)
	}
	handler, mode, err := NewHandler(dist, os.Getenv("GIMBLE_WEB_PROXY"), origin)
	if err != nil {
		_ = listener.Close()
		i.shutdown.Done()
		return fmt.Errorf("gimble: assemble web application: %w", err)
	}
	server := &http.Server{
		Handler: i.projectRequest(handler),
		BaseContext: func(net.Listener) context.Context {
			return i.ctx
		},
	}
	i.address = listener.Addr().String()
	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			i.cancel(fmt.Errorf("gimble: serve web application: %w", err))
		}
	}()
	context.AfterFunc(i.ctx, func() {
		_ = server.Close()
		<-serveDone
		if cfg.network == "unix" {
			_ = os.Remove(address)
		}
		i.shutdown.Done()
	})
	log.Printf("gimble: web application listening on %s (%s)", listener.Addr(), mode)
	return nil
}

func (i *Instance) waitForShutdown() {
	go func() {
		i.shutdown.Wait()
		close(i.done)
	}()
}

func (i *Instance) projectRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/_app/") {
			// Kit's remote endpoint is shared by all pages. The browser supplies
			// its project path in Referer for each tab's request.
			path = r.Header.Get("Referer")
			if parsed, err := url.Parse(path); err == nil {
				path = parsed.Path
			}
		}
		var p *Runtime
		i.mu.RLock()
		choices := make([]hooks.ProjectChoice, 0, len(i.projects))
		for _, candidate := range i.projects {
			choices = append(choices, hooks.ProjectChoice{ID: candidate.id, Path: candidate.project})
			if strings.HasPrefix(path, "/projects/"+candidate.id) &&
				(len(path) == len("/projects/")+len(candidate.id) || path[len("/projects/")+len(candidate.id)] == '/') {
				p = candidate
				break
			}
		}
		i.mu.RUnlock()
		slices.SortFunc(choices, func(a, b hooks.ProjectChoice) int { return strings.Compare(a.Path, b.Path) })
		if p == nil {
			if strings.HasPrefix(r.URL.Path, "/projects/") || strings.Contains(r.URL.Path, "/remote/") || strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(hooks.WithProjects(r.Context(), choices)))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/projects/"+p.id+"/api/") {
			r = r.Clone(r.Context())
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/projects/"+p.id)
		}
		next.ServeHTTP(w, r.WithContext(p.requestContext(r.Context())))
	})
}

func (p *Runtime) requestContext(ctx context.Context) context.Context {
	ctx = hooks.WithProjectDir(ctx, p.dir)
	ctx = observation.WithRegistry(ctx, p.registry)
	ctx = live.WithRuns(ctx, p.runs)
	return conversation.WithManager(ctx, p.conversations)
}

// Run starts one workflow run and blocks until body returns. models binds
// every role the workflow names. The run ends when either ctx or the runtime
// context ends. A workflow panic is recorded as a failed run and returned as
// an error; direct gimble.Run callers still receive the panic.
func (r *Runtime) Run(ctx context.Context, name string, models map[gimble.WorkflowRole]gimble.ModelBinding, body func(context.Context) error) (err error) {
	if r == nil {
		return errors.New("gimble: nil runtime")
	}
	if ctx == nil {
		return errors.New("gimble: run context is nil")
	}
	if body == nil {
		return errors.New("gimble: run body is nil")
	}
	// Run writes the failed terminal record before re-raising a workflow panic.
	// A hosted run reports that failure to its caller without taking down the
	// instance's other runs or listeners.
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("gimble: hosted workflow panic: %v", value)
		}
	}()
	// A runtime that has already ended starts no run: the body must never
	// see a live context under a dead runtime.
	if r.ctx.Err() != nil {
		return context.Cause(r.ctx)
	}
	// The run's context descends from the caller's, so what the caller put
	// on it reaches the run. What the runtime owns is put on it here
	// instead of being inherited, and the runtime's own end cancels it the
	// way the caller's does.
	runCtx, cancel := context.WithCancelCause(ctx)
	stop := context.AfterFunc(r.instance.ctx, func() { cancel(context.Cause(r.instance.ctx)) })
	defer stop()
	defer cancel(nil)
	runCtx = observation.WithRegistry(runCtx, r.registry)
	runCtx = live.WithRuns(runCtx, r.runs)
	// The run puts itself in the runtime's table under its id for as long
	// as its body runs, so Steer, KillScope, and KillTurn can reach it.
	hook := r.runs.Hook
	if started, _ := ctx.Value(conversationRunStartedKey{}).(func(string)); started != nil {
		hook = func(id string, run live.Controller) func() {
			release := r.runs.Hook(id, run)
			started(id)
			return release
		}
	}
	runCtx = live.WithHook(runCtx, hook)
	err = gimble.Run(gimble.Project(runCtx, r.dir), name, models, body)
	if err == nil && context.Cause(runCtx) != nil {
		return context.Cause(runCtx)
	}
	return err
}

type conversationRunStartedKey struct{}

// Steer sends message into the turn running on session sessionID of the run
// runID, as the person watching the page: the run log records it with
// Source "person", and landed reports whether a turn received it. An
// unknown or finished run or session is an error.
func (r *Runtime) Steer(ctx context.Context, runID, sessionID, message string) (landed bool, err error) {
	run, err := r.runs.InProgress(runID)
	if err != nil {
		return false, err
	}
	return run.Steer(ctx, sessionID, message)
}

// SteerLoop holds message for the planner of the loop scopeKey of the run
// runID, as the person watching the page. It reaches the planner at its
// next planning decision, whether or not a turn is running when it is
// sent, and the loop's record says whether the planner read it. An unknown
// or finished run, or a scope that is not a loop still dispatching, is an
// error.
func (r *Runtime) SteerLoop(runID, scopeKey, message string) error {
	run, err := r.runs.InProgress(runID)
	if err != nil {
		return err
	}
	return run.SteerLoop(scopeKey, message)
}

// KillScope ends the scope scopeKey of the run runID, and everything under
// it, with a Killed cause naming who did it and why. The run log records
// the kill on the scope. An unknown or finished run or scope is an error.
func (r *Runtime) KillScope(runID, scopeKey, by, reason string) error {
	run, err := r.runs.InProgress(runID)
	if err != nil {
		return err
	}
	return run.CancelScope(scopeKey, gimble.Killed{Target: scopeKey, By: by, Reason: reason})
}

// KillTurn ends only the turn turnID of the run runID with a Killed cause
// naming who did it and why; the turn's session and scope keep running.
// The run log records the kill on the turn. An unknown or finished run or
// turn is an error.
func (r *Runtime) KillTurn(runID, turnID, by, reason string) error {
	run, err := r.runs.InProgress(runID)
	if err != nil {
		return err
	}
	return run.CancelTurn(turnID, gimble.Killed{Target: turnID, By: by, Reason: reason})
}
