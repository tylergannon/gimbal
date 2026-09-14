package web

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/live"
	"github.com/tylergannon/gimble/internal/observation"
)

// Runtime owns a project's runs and web application. It remains active until
// the context passed to NewRuntime is cancelled.
type Runtime struct {
	ctx     context.Context
	cancel  context.CancelCauseFunc
	dir     string
	done    chan struct{}
	address string

	mu      sync.Mutex
	running map[string]live.Controller // runs in progress by id, for Steer, KillScope, and KillTurn
}

// Option configures the project's web listener.
type Option func(*config) error

type config struct {
	network  string
	port     int
	uds      string
	explicit string
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

// WithNoWeb prevents the runtime from opening a listener. Runs and their
// durable records still work normally.
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

// NewRuntime creates the runtime for projectDir and starts its web application
// before returning. It listens on loopback port 8080 unless an option selects
// another port, a Unix-domain socket, or no listener.
func NewRuntime(ctx context.Context, projectDir string, opts ...Option) (*Runtime, error) {
	if ctx == nil {
		return nil, errors.New("gimble: runtime context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, fmt.Errorf("gimble: project directory: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("gimble: project directory: %w", err)
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
	// The observation registry lives in the runtime's context. Every run the
	// runtime starts finds it there and registers its store; the web server
	// serves requests from this same context through BaseContext, so its
	// routes and its page loads read the very same registry.
	runtimeCtx = observation.WithRegistry(runtimeCtx, observation.NewRegistry(dir))
	runtime := &Runtime{ctx: runtimeCtx, cancel: cancel, dir: dir, done: make(chan struct{}), running: make(map[string]live.Controller)}
	if cfg.network == "none" {
		context.AfterFunc(runtimeCtx, func() { close(runtime.done) })
		return runtime, nil
	}
	if err := runtime.startWeb(cfg); err != nil {
		cancel(err)
		return nil, err
	}
	return runtime, nil
}

func (r *Runtime) startWeb(cfg config) error {
	address := cfg.uds
	if cfg.network == "tcp" {
		address = net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.port))
	}
	listener, err := net.Listen(cfg.network, address)
	if err != nil {
		return fmt.Errorf("gimble: listen on %s: %w", address, err)
	}
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
		return fmt.Errorf("gimble: web application: %w", err)
	}
	handler, mode, err := NewHandler(dist, os.Getenv("GIMBLE_WEB_PROXY"), origin)
	if err != nil {
		_ = listener.Close()
		return fmt.Errorf("gimble: assemble web application: %w", err)
	}
	server := &http.Server{
		Handler: handler,
		BaseContext: func(net.Listener) context.Context {
			return r.ctx
		},
	}
	r.address = listener.Addr().String()
	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			r.cancel(fmt.Errorf("gimble: serve web application: %w", err))
		}
	}()
	context.AfterFunc(r.ctx, func() {
		_ = server.Close()
		<-serveDone
		if cfg.network == "unix" {
			_ = os.Remove(address)
		}
		close(r.done)
	})
	log.Printf("gimble: web application listening on %s (%s)", listener.Addr(), mode)
	return nil
}

// Run starts one workflow run and blocks until body returns. The run ends when
// either ctx or the runtime context ends.
func (r *Runtime) Run(ctx context.Context, name string, body func(context.Context) error) error {
	if r == nil {
		return errors.New("gimble: nil runtime")
	}
	if ctx == nil {
		return errors.New("gimble: run context is nil")
	}
	if body == nil {
		return errors.New("gimble: run body is nil")
	}
	runCtx, cancel := context.WithCancelCause(r.ctx)
	stop := context.AfterFunc(ctx, func() { cancel(context.Cause(ctx)) })
	defer stop()
	defer cancel(nil)
	// The run puts itself in the runtime's table under its id for as long
	// as its body runs, so Steer, KillScope, and KillTurn can reach it.
	runCtx = live.WithHook(runCtx, r.register)
	err := gimble.Run(gimble.Project(runCtx, r.dir), name, body)
	if err == nil && context.Cause(runCtx) != nil {
		return context.Cause(runCtx)
	}
	return err
}

func (r *Runtime) register(id string, run live.Controller) func() {
	r.mu.Lock()
	r.running[id] = run
	r.mu.Unlock()
	return func() {
		r.mu.Lock()
		delete(r.running, id)
		r.mu.Unlock()
	}
}

// inProgress returns the run in progress under runID; a finished or unknown run
// is an error.
func (r *Runtime) inProgress(runID string) (live.Controller, error) {
	r.mu.Lock()
	run := r.running[runID]
	r.mu.Unlock()
	if run == nil {
		return nil, fmt.Errorf("gimble: no run %q in progress", runID)
	}
	return run, nil
}

// Steer sends message into the turn running on session sessionID of the run
// runID, as the person watching the page: the run log records it with
// Source "person", and landed reports whether a turn received it. An
// unknown or finished run or session is an error.
func (r *Runtime) Steer(ctx context.Context, runID, sessionID, message string) (landed bool, err error) {
	run, err := r.inProgress(runID)
	if err != nil {
		return false, err
	}
	return run.Steer(ctx, sessionID, message)
}

// KillScope ends the scope scopeKey of the run runID, and everything under
// it, with a Killed cause naming who did it and why. The run log records
// the kill on the scope. An unknown or finished run or scope is an error.
func (r *Runtime) KillScope(runID, scopeKey, by, reason string) error {
	run, err := r.inProgress(runID)
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
	run, err := r.inProgress(runID)
	if err != nil {
		return err
	}
	return run.CancelTurn(turnID, gimble.Killed{Target: turnID, By: by, Reason: reason})
}
