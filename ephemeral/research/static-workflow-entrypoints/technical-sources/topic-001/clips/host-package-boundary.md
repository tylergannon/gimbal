# Host Package Boundary and Cyclic Import Elimination

Primary sources:
- [`../sources/web-runtime.go.txt`](../sources/web-runtime.go.txt) (web/runtime.go: `Instance`, `Runtime`, `AdmitProject`, `Run`)
- [`../sources/web-control.go.txt`](../sources/web-control.go.txt) (web/control.go: `controlDiscovery`, `controlHandler`, `Submission`, `submit`)
- [`../sources/web-src-project.go.txt`](../sources/web-src-project.go.txt) (web/src/project.go: `projectDirKey`, `projectsKey`, `ProjectChoice`, context helpers)
- [`../sources/web-server.go.txt`](../sources/web-server.go.txt) (web/server.go: `NewHandler`, `explainOriginRefusals`)

## 1. Concrete Types and Lifecycle Functions Moving to Host Package

To break the cyclic import chain (`web -> internal/skgo -> web/src/routes -> internal/workflows/... -> web`), a new host package below web assembly (e.g. `internal/host`) must own project admission, run lifetime, and control sockets:

### Types Moving to Host Package
- `host.Instance`: owns instance directory, control socket (`startControl`), active runs waitgroup, background shutdown coordination, and project runtime registry map (`map[string]*Runtime`).
- `host.Runtime`: owns an admitted repository's canonical directory, state directory (`.gimbal`), filesystem lock (`ownerLock *os.File`), live runs table (`*live.Runs`), observation registry (`*observation.Registry`), and conversation manager (`*conversation.Manager`).
- `host.ProjectChoice`: JSON model for admitted projects (`ID`, `Path`).
- `host.controlDiscovery`: JSON discovery metadata written to `.gimbal/control/<id>.json`.
- `host.controlHandler`: HTTP handler serving `/control/runs`, `/control/steer`, `/control/steer-loop`.

### Lifecycle Functions Moving to Host Package
- `NewInstance(ctx context.Context, instanceDir string, initialProjects []string, opts ...Option) (*Instance, error)`
- `NewRuntime(ctx context.Context, projectDir string, opts ...Option) (*Runtime, error)`
- `(i *Instance) AdmitProject(dir string) (*Runtime, error)`: handles canonicalization (`EvalSymlinks`), `.gimbal` creation, `unix.Flock` on `ownerLock`, registry/runs/conversation initialization, and project discovery persistence.
- `canonicalProject(dir string) (string, error)` and `projectID(path string) string`.
- `(r *Runtime) Run(ctx context.Context, name string, models map[gimbal.WorkflowRole]gimbal.ModelBinding, body func(context.Context) error) error`: handles panic containment (`recover`), active runs tracking, lifecycle context derivation from `instance.ctx`, and invocation of `gimbal.Run`.
- `(r *Runtime) Steer(...)`, `(r *Runtime) SteerLoop(...)`, `(r *Runtime) KillScope(...)`, `(r *Runtime) KillTurn(...)`.
- `(r *Runtime) controlRuns() ([]observation.RunRow, error)`.
- `(i *Instance) startControl() error` and `writeProjectDiscovery(project string) error`.

### Replaced / Deleted Functions and Types
- `WorkflowEntry`, `WithWorkflows`: deleted (no dynamic executable registry).
- `Submission`, `Admission`, `(h controlHandler) submit()`: deleted (superseded by typed SKGO Form start remotes).

## 2. Context Keys and Types Relocation

Currently, `web/src/project.go` defines context keys and helpers inside package `hooks` (`web/src`):
- `projectDirKey` and `WithProjectDir(ctx, dir)`, `ProjectDir(ctx)`
- `projectsKey` and `WithProjects(ctx, choices)`, `Projects(ctx)`
- `ProjectChoice`

If `internal/host` imported `web/src` to set these context keys, any route in `web/src/routes` importing `internal/host` would create an import cycle.
Relocation:
- Move `ProjectChoice`, `WithProjectDir`, `ProjectDir`, `WithProjects`, and `Projects` directly into `internal/host` (or an `internal/host/context` subpackage).
- The internal host context keys (`conversationRunStartedKey`) remain internal to `internal/host`.
- `web/src/routes/page.server.go` and other route handlers import `internal/host` to read `host.Projects(ctx)` and `host.ProjectDir(ctx)`.
- `internal/host` has zero imports of `web` or `web/src`.

## 3. Concrete Remote Handlers Referencing Host Ownership

Concrete remote handlers (e.g. `web/src/routes/review.remote.go` or `web/src/routes/start/*.remote.go`):
1. Import `internal/host` directly (allowed because `internal/host` is strictly below `web`).
2. Import the concrete workflow package directly (e.g. `github.com/tylergannon/gimbal/internal/workflows/review`), which itself only imports `gimbal` and `workflow`.
3. In the remote handler:
   ```go
   func StartReview(ctx context.Context, req ReviewRequest) (AdmissionResult, error) {
       inst := host.FromContext(ctx)
       p, err := inst.AdmitProject(req.Project)
       if err != nil {
           return AdmissionResult{}, err
       }
       // Validate models, workdir, conversation association...
       runID, err := p.StartRun(ctx, "review", models, func(runCtx context.Context) error {
           return review.Review(runCtx, gimbal.Env{WorkDir: req.WorkDir}, req.Params)
       })
       return AdmissionResult{ID: runID, Project: p.ID()}, err
   }
   ```
4. Web assembly (`web/server.go`) does NOT import concrete workflow packages or concrete remote implementations; it only mounts the SKGO registries produced by code generation.
