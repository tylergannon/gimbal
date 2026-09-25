# Topic 008: Project Admission Concurrency, Path Aliasing, and Run Lifetime

## Index and Question Routing

This topic establishes the local evidence for context hierarchies, decoupled asynchronous run lifetimes, canonical project path resolution, cross-process and in-process admission locking, and multi-project isolation under a single host process.

### Question 1: How must the context hierarchy in `web/runtime.go` and `web/src/project.go` be structured so that completing the HTTP request does not terminate the asynchronous workflow run context?

- **Local Evidence**:
  - [`source-001-runtime-admission-lifecycle.md`](sources/source-001-runtime-admission-lifecycle.md): Excerpt of `web/runtime.go:417-471` (`Runtime.Run` context hierarchy with `runCtx, cancel := context.WithCancelCause(ctx)`, `context.AfterFunc(r.instance.ctx, ...)`).
  - [`source-002-control-submission-async-run.md`](sources/source-002-control-submission-async-run.md): Excerpt of `web/control.go:119-158` (goroutine spawning, `conversationRunStartedKey` channel wait, detached execution beyond HTTP response).
  - [`source-003-web-src-project-context.md`](sources/source-003-web-src-project-context.md): Excerpt of `web/src/project.go:1-36` (`WithProjectDir`, `ProjectDir`, `WithProjects`, `Projects`).
  - [`clip-context-hierarchy.md`](clips/clip-context-hierarchy.md): Context inheritance analysis between transient HTTP request and long-lived workflow run.
- **Supported Facts**:
  - HTTP requests evaluated by SKGO `serveForm` run with context `ctx := withEvent(r.Context(), ev)`. `r.Context()` is cancelled by `net/http` when the HTTP response completes or if the client disconnects prematurely.
  - In `web/control.go:119-126`, workflow execution is spawned in a separate background goroutine running `p.Run(ctx, ...)`. Crucially, `ctx` is derived from `p.ctx` (the project's context, which descends from `i.ctx`), **not** `r.Context()`.
  - In `web/runtime.go:449-452`, `Run` creates `runCtx, cancel := context.WithCancelCause(ctx)` and binds instance termination with `context.AfterFunc(r.instance.ctx, func() { cancel(context.Cause(r.instance.ctx)) })`.
  - `web/control.go:119-144` uses `conversationRunStartedKey` to communicate the admitted run ID back to the HTTP handler through a channel (`started := make(chan string, 1)`). The HTTP handler waits on `select` for `id := <-started`, writes the response, and exits while the workflow continues running under `p.ctx`.
  - Context keys `projectDirKey` and `projectsKey` currently live in `web/src/project.go` (package `hooks`).
- **Inference & Implementation Consequence**:
  - For start remote Form handlers, the handler must spawn the workflow under the admitted project's context `p.ctx` (or host project context), completely decoupled from `r.Context()`.
  - The handler blocks on `started` channel wait until the run ID is assigned (or admission fails), then returns `AdmissionResult{ID: id}`.
  - If the client aborts or cancels its HTTP request, only its network wait ends; the accepted workflow continues executing under `p.ctx`.
  - Relocating project admission and run ownership to an internal host package below `web` requires moving the context keys and helpers (`WithProjectDir`, `ProjectDir`) out of `web/src/` into the new host package to prevent cyclic dependencies.
- **Contradictions & Unresolved Questions**:
  - None. Both existing code and the accepted plan agree that workflow runs must descend from project/instance context rather than HTTP request context.

---

### Question 2: How does canonical project path resolution (handling symlinks, relative paths, and trailing slashes) operate under the admission lock to guarantee a single project owner per repository?

- **Local Evidence**:
  - [`source-001-runtime-admission-lifecycle.md`](sources/source-001-runtime-admission-lifecycle.md): Excerpt of `web/runtime.go:218-296` (`canonicalProject` and `AdmitProject`).
  - [`source-004-project-ownership-isolation-tests.md`](sources/source-004-project-ownership-isolation-tests.md): Excerpts of `web/project_ownership_test.go:83-127` and `web/instance_test.go:54-65`.
- **Supported Facts**:
  - **Path Canonicalization**: `web/runtime.go:285-291` defines `canonicalProject(dir string) (string, error)`:
    ```go
    path, err := filepath.Abs(dir)
    if err != nil {
        return "", err
    }
    return filepath.EvalSymlinks(path)
    ```
    `filepath.Abs` cleans `.` and `..` components and normalizes trailing slashes. `filepath.EvalSymlinks` resolves all symbolic link segments to the absolute target directory.
  - **In-Process Admission Deduplication**: In `web/runtime.go:234-241`, under `i.mu.Lock()`, `i.projects[path]` is checked. If already present, the existing `*Runtime` is returned immediately. As tested in `web/instance_test.go:54-65`, symlinks (`alias`), relative paths (`b/../a`), and direct paths return the identical `*Runtime` pointer (`again == a`).
  - **Cross-Process Admission Lock**: In `web/runtime.go:244-254`, if the project is not in `i.projects`, the host opens `filepath.Join(state, "owner.lock")` and executes non-blocking exclusive flock:
    `unix.Flock(int(ownerLock.Fd()), unix.LOCK_EX|unix.LOCK_NB)`.
    If another process holds the lock, flock fails with `unix.EWOULDBLOCK`, and `AdmitProject` returns `"project %s is already owned by another instance"`.
  - **Inode Race Avoidance**: As commented in `web/runtime.go:242-243`, `owner.lock` is retained on disk and never deleted on close, preventing race conditions where a new owner locks a different inode while a contender holds the old one.
  - **Shutdown Coordination**: As tested in `web/project_ownership_test.go:129-186`, `owner.lock` is held until all active runs completely unwind and finish (`owner.Wait()`). A second instance cannot claim the repository while runs are unwinding.
- **Inference & Implementation Consequence**:
  - Canonical resolution occurs *before* acquiring `i.mu` and before taking `owner.lock`.
  - Any valid directory path, whether supplied as a relative path, trailing-slash path, or symlink alias, resolves to the same unique canonical key in `i.projects` and the exact same `owner.lock` file on disk.
- **Contradictions & Unresolved Questions**:
  - None. The dual guarantee (in-process mutex + map; cross-process kernel `flock`) is verified by deterministic tests.

---

### Question 3: How does the host track concurrent active runs across multiple admitted projects under a single PID while ensuring isolation of project-scoped observation and control endpoints?

- **Local Evidence**:
  - [`source-001-runtime-admission-lifecycle.md`](sources/source-001-runtime-admission-lifecycle.md): Excerpt of `web/runtime.go:57-70` (`Instance` struct with `activeRuns sync.WaitGroup` and `projects map[string]*Runtime`), and lines 442-444 (`activeRuns.Add(1)` / `activeRuns.Done()`).
  - [`source-002-control-submission-async-run.md`](sources/source-002-control-submission-async-run.md): Excerpt of `web/control.go:335-361` (`controlRuns`).
  - [`source-004-project-ownership-isolation-tests.md`](sources/source-004-project-ownership-isolation-tests.md): Excerpts of `web/instance_test.go:167-180` and `web/control_ownership_test.go:46-60`.
- **Supported Facts**:
  - **Single PID Concurrency Tracking**:
    - `Instance` contains `activeRuns sync.WaitGroup` (`web/runtime.go:63`).
    - Every run initiated via `Runtime.Run` calls `r.instance.activeRuns.Add(1)` before starting, and `defer r.instance.activeRuns.Done()` upon termination (`web/runtime.go:442-444`).
    - During server shutdown (`web/runtime.go:286`), the instance calls `i.activeRuns.Wait()`, ensuring the server process remains alive until all runs across all projects have cleanly terminated.
  - **State Isolation Per Project**:
    - Each admitted project has its own dedicated `Runtime` struct with an independent `.gimbal` directory (`p.dir`), its own `observation.Registry` (`p.registry`), its own active run table (`p.runs = live.NewRuns()`), and its own `conversation.Manager` (`web/runtime.go:261-276`).
    - Projects do not share live run tables or observation registries (`web/instance_test.go:66-68`).
  - **Endpoint Isolation on Web Listener**:
    - URLs are scoped by project ID: `/projects/<id>/...` where ID is `sha256(canonicalPath)[:16]`.
    - `projectRequest` (`web/runtime.go:398-403`) validates project IDs and attaches that project's specific request context (`p.requestContext`).
    - Requesting project A's runs under project B's URL prefix (e.g. `/projects/<b.id>/runs/<a.id>`) yields `404 Not Found` (`web/instance_test.go:174, 197-198`).
  - **Endpoint Isolation on Control Socket**:
    - Requests on the control socket supply `X-Gimbal-Project: <project-path>`.
    - `controlHandler.project` (`web/control.go:36-52`) looks up the canonical project.
    - `controlRuns` (`web/control.go:335-361`) scans only `filepath.Join(r.dir, "runs")` and filters through `r.runs.InProgress(entry.Name())`.
    - `web/control_ownership_test.go:46-60` proves project A and project B report only their own active runs, with zero leakage across project boundaries.
- **Inference & Implementation Consequence**:
  - While start remotes are instance-scoped (allowing admission of new projects), all subsequent observation and control endpoints (e.g. snapshots, event streams, steer, kill) must remain strictly project-scoped.
  - Moving runtime ownership to an internal host package preserves this isolation model: `Instance` tracks global active runs and admitted projects, while `Runtime` encapsulates project-isolated state.
- **Contradictions & Unresolved Questions**:
  - None. Concurrency and endpoint isolation are supported by primary source implementations and unit/integration tests.
