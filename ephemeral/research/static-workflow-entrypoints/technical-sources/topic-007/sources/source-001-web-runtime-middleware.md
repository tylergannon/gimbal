# Source: web/runtime.go - Request Middleware and Listener Lifecycle

- **Origin**: `/Users/tyler/.codex/worktrees/d798/gimbal/web/runtime.go`
- **Commit**: `40dc82947eed99202fd9cb1dd377b6a3c2abbccc`
- **Retrieval Date**: 2026-09-23
- **Scope**: Listener configuration options (`WithPort`, `WithUDS`, `WithNoWeb`), optional browser listener startup (`startWeb`), control UDS startup (`startControl`), and middleware filtering (`projectRequest`).

---

### Listener Configuration for Optional Browser/Web Listener (`web/runtime.go:86-141`)

```go
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
			return fmt.Errorf("gimbal: invalid web port %d", port)
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
			return errors.New("gimbal: UDS path must not be blank")
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
```

### Control UDS Startup (Always Active) (`web/runtime.go:230-283`)

```go
func (i *Instance) startControl() error {
	controlDir := filepath.Join(i.dir, "control")
	if err := os.MkdirAll(controlDir, 0o755); err != nil {
		return fmt.Errorf("gimbal: control directory: %w", err)
	}

	var rawID [4]byte
	if _, err := rand.Read(rawID[:]); err != nil {
		return fmt.Errorf("gimbal: control identity: %w", err)
	}
	identity := hex.EncodeToString(rawID[:])
	i.controlID = identity
	socket := filepath.Join(i.dir, identity+".sock")
	discovery := filepath.Join(controlDir, identity+".json")
	listener, err := net.Listen("unix", socket)
	if err != nil && strings.Contains(err.Error(), "invalid argument") {
		// macOS limits the total Unix socket path length. A long temporary or
		// checkout path cannot hold the socket itself, but its discovery file
		// still lives under the project and points to this short fallback.
		socket = filepath.Join("/tmp", "gimbal-"+identity+".sock")
		listener, err = net.Listen("unix", socket)
	}
	if err != nil {
		return fmt.Errorf("gimbal: listen on control socket: %w", err)
	}
	info := controlDiscovery{PID: os.Getpid(), Socket: socket, Project: i.dir}
	i.controlSocket = socket
	encoded, err := json.Marshal(info)
	if err != nil {
		_ = listener.Close()
		_ = os.Remove(socket)
		return fmt.Errorf("gimbal: encode control discovery: %w", err)
	}
	if err := os.WriteFile(discovery, encoded, 0o644); err != nil {
		_ = listener.Close()
		_ = os.Remove(socket)
		return fmt.Errorf("gimbal: write control discovery: %w", err)
	}

	server := &http.Server{
		Handler: controlMux(i),
		BaseContext: func(net.Listener) context.Context {
			return i.ctx
		},
	}
	i.shutdown.Add(1)
	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			i.cancel(fmt.Errorf("gimbal: serve control socket: %w", err))
		}
	}()
...
```

### Browser Web Listener Startup (`web/runtime.go:298-351`)

```go
func (i *Instance) startWeb(cfg config) error {
	address := cfg.uds
	if cfg.network == "tcp" {
		address = net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.port))
	}
	listener, err := net.Listen(cfg.network, address)
	if err != nil {
		return fmt.Errorf("gimbal: listen on %s: %w", address, err)
	}
	i.shutdown.Add(1)
	origin := ""
	if cfg.network == "tcp" {
		origin = "http://" + listener.Addr().String()
	}
	if configured := os.Getenv("GIMBAL_WEB_ORIGIN"); configured != "" {
		origin = configured
	}
	dist, err := fs.Sub(Build, "build")
	if err != nil {
		_ = listener.Close()
		i.shutdown.Done()
		return fmt.Errorf("gimbal: web application: %w", err)
	}
	handler, mode, err := NewHandler(dist, os.Getenv("GIMBAL_WEB_PROXY"), origin)
	if err != nil {
		_ = listener.Close()
		i.shutdown.Done()
		return fmt.Errorf("gimbal: assemble web application: %w", err)
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
			i.cancel(fmt.Errorf("gimbal: serve web application: %w", err))
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
	log.Printf("gimbal: web application listening on %s (%s)", listener.Addr(), mode)
	return nil
}
```

### Project Scoping Middleware (`web/runtime.go:366-411`)

```go
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
```
