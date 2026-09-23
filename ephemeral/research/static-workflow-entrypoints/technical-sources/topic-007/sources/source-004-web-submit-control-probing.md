# Source: web/submit.go and web/control.go - Control Transport and Discovery Probing

- **Origin**: `/Users/tyler/.codex/worktrees/d798/gimble/web/submit.go` and `/Users/tyler/.codex/worktrees/d798/gimble/web/control.go`
- **Commit**: `40dc82947eed99202fd9cb1dd377b6a3c2abbccc`
- **Retrieval Date**: 2026-09-23
- **Scope**: Discovery probing requiring admitted project (`selectedInstance`), UDS transport, and control socket dispatch (`controlMux`).

---

### `web/submit.go:86-141`

```go
func selectedInstance(ctx context.Context, instanceDir, project string) (selectedClient, error) {
	instanceDir, err := filepath.Abs(instanceDir)
	if err != nil {
		return selectedClient{}, err
	}
	project, err = canonicalProject(project)
	if err != nil {
		return selectedClient{}, err
	}
	entries, err := os.ReadDir(filepath.Join(instanceDir, "control"))
	if errors.Is(err, os.ErrNotExist) {
		return selectedClient{}, fmt.Errorf("no running Gimble instance at %s; start gimble --instance-dir %s --project %s", instanceDir, instanceDir, project)
	}
	if err != nil {
		return selectedClient{}, err
	}
	var clients []selectedClient
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(instanceDir, "control", entry.Name()))
		if err != nil {
			continue
		}
		var discovery controlDiscovery
		if json.Unmarshal(data, &discovery) == nil && discovery.Socket != "" {
			candidate := selectedClient{socket: discovery.Socket, project: project}
			probe, err := candidate.request(ctx, http.MethodGet, "/control/runs", nil)
			if err == nil {
				_ = probe.Body.Close()
				if probe.StatusCode == http.StatusOK {
					clients = append(clients, candidate)
				}
			}
		}
	}
	if len(clients) != 1 {
		return selectedClient{}, fmt.Errorf("selected Gimble instance at %s has %d live endpoints admitting project %s; start or select one instance with --instance-dir", instanceDir, len(clients), project)
	}
	return clients[0], nil
}

func (c selectedClient) request(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, "http://gimble"+path, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Gimble-Project", c.project)
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", c.socket)
	}}
	response, err := (&http.Client{Transport: transport}).Do(request)
	transport.CloseIdleConnections()
	return response, err
}
```

### `web/control.go:32-52, 323-333`

```go
func (h controlHandler) project(r *http.Request) (*Runtime, error) {
	name := r.Header.Get("X-Gimble-Project")
	if name == "" {
		return nil, errors.New("gimble: project is required")
	}
	path, err := canonicalProject(name)
	if err != nil {
		return nil, err
	}
	h.instance.mu.RLock()
	defer h.instance.mu.RUnlock()
	p := h.instance.projects[path]
	if p == nil {
		return nil, fmt.Errorf("gimble: project %s is not admitted", path)
	}
	return p, nil
}

func controlMux(i *Instance) http.Handler {
	h := controlHandler{instance: i}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := h.project(r)
		if err != nil || p == nil {
			http.Error(w, "project is not admitted", http.StatusNotFound)
			return
		}
		observation.Routes(h).ServeHTTP(w, r.WithContext(p.requestContext(r.Context())))
	})
}
```
