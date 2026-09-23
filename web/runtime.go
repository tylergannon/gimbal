package web

import (
	"context"
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

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/host"
)

// Instance owns one web listener, one control socket, and any number of
// admitted projects. Project state and durable files live below each project's
// directory; instanceDir holds only the control endpoint and discovery.
type Instance struct {
	ctx       context.Context
	cancel    context.CancelCauseFunc
	dir       string
	done      chan struct{}
	shutdown  sync.WaitGroup
	address   string
	Owner     *host.Owner
	workflows map[string]WorkflowEntry
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
// endpoints. Initial projects are admitted before the web listener starts.
// AdmitProject can add more projects without opening another listener.
// The web application listens on loopback port 8080 unless configured otherwise.
func NewInstance(ctx context.Context, instanceDir string, initialProjects []string, opts ...Option) (*Instance, error) {
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
		ctx: runtimeCtx, cancel: cancel, dir: dir, done: make(chan struct{}), workflows: cfg.workflows, Owner: host.New(runtimeCtx, dir),
	}
	if err := instance.startControl(); err != nil {
		cancel(err)
		return nil, err
	}
	for _, project := range initialProjects {
		if _, err := instance.Owner.AdmitProject(project); err != nil {
			cancel(err)
			instance.waitForShutdown()
			<-instance.done
			return nil, err
		}
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

// Wait blocks until the instance has stopped its listeners and released its
// projects after their active work has ended.
func (i *Instance) Wait() {
	<-i.done
}

func (i *Instance) projectRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(host.WithOwner(r.Context(), i.Owner))
		path := r.URL.Path
		if strings.HasPrefix(path, "/_app/") {
			// Kit's remote endpoint is shared by all pages. The browser supplies
			// its project path in Referer for each tab's request.
			path = r.Header.Get("Referer")
			if parsed, err := url.Parse(path); err == nil {
				path = parsed.Path
			}
		}
		var p *host.Project
		projects := i.Owner.Projects()
		choices := make([]host.ProjectChoice, 0, len(projects))
		for _, candidate := range projects {
			choices = append(choices, host.ProjectChoice{ID: candidate.ID(), Path: candidate.Path()})
			if strings.HasPrefix(path, "/projects/"+candidate.ID()) &&
				(len(path) == len("/projects/")+len(candidate.ID()) || path[len("/projects/")+len(candidate.ID())] == '/') {
				p = candidate
			}
		}
		slices.SortFunc(choices, func(a, b host.ProjectChoice) int { return strings.Compare(a.Path, b.Path) })
		if p == nil {
			if strings.HasPrefix(r.URL.Path, "/projects/") || strings.Contains(r.URL.Path, "/remote/") || strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(host.WithProjects(r.Context(), choices)))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/projects/"+p.ID()+"/api/") {
			r = r.Clone(r.Context())
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/projects/"+p.ID())
		}
		next.ServeHTTP(w, r.WithContext(p.RequestContext(r.Context())))
	})
}
