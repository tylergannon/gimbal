# Source: web/runtime.go - Project Admission, Canonicalization, and Run Context Lifecycle

- **Origin**: `/Users/tyler/.codex/worktrees/d798/gimble/web/runtime.go`
- **Commit**: `40dc82947eed99202fd9cb1dd377b6a3c2abbccc`
- **Retrieval Date**: 2026-09-23
- **Scope**: Canonical path resolution, exclusive flock project admission, and the runtime context hierarchy in `Run`.

---

### Project Canonicalization and Admission (`web/runtime.go:218-296`)

```go
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
```

### Context Hierarchy and Active Run Tracking in `Run` (`web/runtime.go:417-471`)

```go
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
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("gimble: hosted workflow panic: %v", value)
		}
	}()
	r.instance.mu.Lock()
	if r.instance.ctx.Err() != nil {
		r.instance.mu.Unlock()
		return context.Cause(r.instance.ctx)
	}
	r.instance.activeRuns.Add(1)
	r.instance.mu.Unlock()
	defer r.instance.activeRuns.Done()

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
```
