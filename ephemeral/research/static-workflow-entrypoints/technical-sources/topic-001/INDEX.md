# Workflow, Host, and Remote Handler Boundaries

Goal: Resolve the package boundary and lifecycle ownership needed to eliminate cyclic imports between workflows, host runtime, and web remote handlers.

## Local Primary Sources

- [web/runtime.go (primary excerpt)](sources/web-runtime.go.txt) — lines 34-70, 147-197, 222-283, 417-471: `Instance`, `Runtime`, `AdmitProject`, `Run`, and active run tracking.
- [web/control.go (primary excerpt)](sources/web-control.go.txt) — lines 26-67, 69-158, 230-304: `controlDiscovery`, `controlHandler`, `submit`, `Submission`, `Admission`, and control socket initialization.
- [web/src/project.go](sources/web-src-project.go.txt) — lines 5-35: `projectDirKey`, `projectsKey`, `ProjectChoice`, `WithProjectDir`, `ProjectDir`, `WithProjects`, `Projects`.
- [web/server.go](sources/web-server.go.txt) — lines 27-127: `NewHandler`, assembly of loads, remotes, endpoints, and origin verification.

Additional detail and package mapping is documented in [host-package-boundary.md](clips/host-package-boundary.md).

## Route Map & Evidence by Question

### 1. Concrete Types and Lifecycle Functions Moving to Host Package
- **Supported facts**:
  - `Instance` ([`sources/web-runtime.go.txt:57-70`](sources/web-runtime.go.txt)) and `Runtime` ([`sources/web-runtime.go.txt:34-52`](sources/web-runtime.go.txt)) manage project state, the filesystem lock `ownerLock` via `unix.Flock`, active runs, and listener shutdown.
  - Lifecycle functions `AdmitProject` ([`sources/web-runtime.go.txt:222-283`](sources/web-runtime.go.txt)), `canonicalProject` ([`sources/web-runtime.go.txt:285-291`](sources/web-runtime.go.txt)), and `Run` ([`sources/web-runtime.go.txt:417-471`](sources/web-runtime.go.txt)) contain project admission, panic containment, and detached run execution.
  - Control functions `startControl`, `writeProjectDiscovery`, and steering methods (`Steer`, `SteerLoop`, `KillScope`, `KillTurn`) operate on `*Instance` and `*Runtime` ([`sources/web-runtime.go.txt:475-523`](sources/web-runtime.go.txt), [`sources/web-control.go.txt:230-304`](sources/web-control.go.txt)).
  - Generic submission types `Submission`, `Admission`, and function `submit` ([`sources/web-control.go.txt:69-158`](sources/web-control.go.txt)) are deleted per the accepted plan.
- **Inference**: Moving `Instance`, `Runtime`, project admission, and control lifecycle into a new package `internal/host` leaves `web/` solely responsible for HTTP server assembly (`NewHandler` in [`sources/web-server.go.txt`](sources/web-server.go.txt)), breaking the import chain.

### 2. Context Keys and Types Currently in web/src/
- **Supported facts**:
  - `web/src/project.go` defines `projectDirKey`, `projectsKey`, `ProjectChoice`, `WithProjects`, `Projects`, `WithProjectDir`, and `ProjectDir` in `package hooks` ([`sources/web-src-project.go.txt:5-35`](sources/web-src-project.go.txt)).
  - `web/runtime.go` imports `hooks "github.com/tylergannon/gimbal/web/src"` ([`sources/web-runtime.go.txt:29`](sources/web-runtime.go.txt)) to attach the project state directory (`.gimbal`) and choice lists to request contexts ([`sources/web-runtime.go.txt:261,407`](sources/web-runtime.go.txt)).
  - In `run.go:151-165`, `gimbal.Run` requires `gimbal.Project(ctx, dir)` which is defined independently in the root `gimbal` package.
- **Inference**: `ProjectChoice` and the project directory context helpers must move from `web/src` into `internal/host`. Route loads in `web/src/routes/*.server.go` will import `internal/host` instead of `web/src` for project directory and choice inspection. Host ownership will no longer import `web/src`.

### 3. Concrete Remote Handlers Referencing Host Project Admission
- **Supported facts**:
  - Routes in SvelteKit/SKGO are located under `web/src/routes/` and scanned by `skgo generate` ([`sources/web-server.go.txt:102-116`](sources/web-server.go.txt)).
  - Handlers receive `ctx context.Context` from SKGO dispatch.
- **Inference**: Remote handlers placed in `web/src/routes/` import `internal/host` (which sits below `web` and does not import `web`) and concrete workflow packages (e.g. `internal/workflows/review`). Handlers retrieve `*host.Instance` via `host.FromContext(ctx)`, call `inst.AdmitProject(path)`, and pass a concrete Go closure (`review.Review(...)`) to `runtime.StartRun`. Web assembly (`web/server.go`) does not import concrete workflow packages.

## Contradictions and Unresolved Questions
- **Contradiction with superseded assessment**: `assessment.md` originally recommended a separate JSON launch endpoint on the control socket and keeping SKGO remotes for browser only; this was explicitly superseded by `implementation-plan.md`, which mandates mounting the same generated SKGO Form start remotes on both the browser listener and the control socket.
- **Unresolved question**: Whether `internal/host` should provide an asynchronous `StartRun(ctx, name, models, body)` returning `(runID string, err error)` directly, or whether callers invoke `go p.Run(...)` with an admission hook channel as currently done in `submit()` ([`sources/web-control.go.txt:119-144`](sources/web-control.go.txt)). Providing a first-class `StartRun` on `*host.Runtime` avoids duplicating goroutine management across five start remotes.
