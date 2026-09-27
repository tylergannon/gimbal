# Execution backend API and limits

## Contract

Workflows select an environment with `gimbal.InEnvironment`. `Run` owns the
backend supplied through `gimbal.WithExecution` and closes it after scope
services stop and native sessions close. A resolved environment supplies the
role's `HarnessAdapter` and starts commands or services in the same live
worktree.

The small public seam is:

```go
type ExecutionBackend interface {
    Resolve(context.Context, string) (ExecutionEnvironment, error)
    Close() error
}

type ExecutionEnvironment interface {
    Harness(WorkflowRole) HarnessAdapter
    Start(context.Context, ExecutionCommand, io.Writer, io.Writer) (ExecutionProcess, error)
}

type ExecutionCommand struct {
    Operation, Session, Role, Workdir, Command string
    Args []string
}

type ExecutionProcess interface {
    Workdir() string
    Wait() (int, error)
    Stop() error
}
```

`ExecutionCommand` carries the absolute work directory inside the environment.
`Start` receives stdout/stderr writers on the controller and must relay output
while the process runs, including for resident services. The Temporal backend
privately spools output under its mounted artifact directory and incrementally
copies it to those writers; it sends no output bytes in Temporal payloads.
Gimbal keeps the complete run artifacts and exposes its existing bounded 64
KiB head/tail excerpt. A nonzero exit is a result; start, transport,
cancellation, and required cleanup failures remain errors.

The backend owns infrastructure and provider configuration. The root package
knows only named environments, role harnesses, process start/wait/stop, and
paths. The current backend uses Temporal activities and Docker workers with
shared project, Git metadata, and run-artifact mounts. Event persistence is
best-effort observation: recording failures are surfaced but do not replace a
successful command or harness result.

The current execution backend supports Codex roles only. A built-in workflow
using this backend must select Codex for every role, including roles whose
normal default is Claude or another harness.

```text
Gimbal workflow / Run
  ├─ InEnvironment(name) ──> ExecutionBackend.Resolve(name)
  │                           ├─ Harness(role) ──> Temporal proxy ──> worker adapter
  │                           └─ Start(command + writers) ──> Temporal activity
  │                                                           └─ worker process
  │                                                              └─ private spool → live relay
  └─ scope end: stop services, close sessions, then backend.Close
```

## Local startup

These commands start local infrastructure and the hosted runtime; provider
secret files are operator supplied and must exist at the configured paths.
Run the foreground services in separate terminals:

```sh
# Terminal 1: stays in the foreground
temporal server start-dev --ip 0.0.0.0
# Terminal 2: Postgres runs in the background
docker compose -f docker-compose.temporal.yaml up -d postgres
# Build once; the backend starts the worker image on demand.
docker build -f Dockerfile.gimbal-worker -t gimbal-command-worker:local .
```

Create a private config file, adjusting the host paths and secret file:

```sh
cat > /tmp/gimbal-execution.json <<'JSON'
{
  "environment": "local",
  "docker_image": "gimbal-command-worker:local",
  "temporal_address": "127.0.0.1:7233",
  "worker_temporal_address": "host.docker.internal:7233",
  "postgres_dsn": "postgres://gimbal:gimbal@127.0.0.1:5433/gimbal?sslmode=disable",
  "worker_postgres_dsn": "postgres://gimbal:gimbal@host.docker.internal:5433/gimbal?sslmode=disable",
  "secret_files": {"OPENAI_API_KEY": "/absolute/path/openai-api-key"}
}
JSON
chmod 600 /tmp/gimbal-execution.json
# Terminal 3: hosted Gimbal runtime stays in the foreground.
go run ./cmd/gimbal --execution-config /tmp/gimbal-execution.json
```

For real hosted runs, select a built-in workflow and its model bindings in the
web app. Do not treat a passing fake-environment test as Docker, Temporal,
worker-image, or provider qualification.

## Evidence observed

- Prior live checks, before Docker metadata I/O errors: a real Docker worker
  dispatched through host Temporal and Postgres, wrote a shared-worktree file,
  and a follow-up command read it. Postgres contained bootstrap and command
  event rows. Dropping the command-event table still allowed a successful
  command while reporting degraded observation. A later live command-only
  readiness check also passed. These runs did not qualify the Codex proxy.
- Current fake-environment checks exercise named environment use, controller
  output writers and bounded excerpts, the hosted built-in implementation path,
  service start/exit/cleanup, session role/environment binding, steering and
  cancellation, and backend close ordering. Host-process tests exercise actual
  local child-process output and process-group cleanup. These use fake
  execution/harness adapters; they do not start Docker or Temporal.
- After the authorized Docker Desktop restart, the worker image was rebuilt
  from revision `d46fde9f`. Both live readiness/command-output and Codex
  create/close lifecycle integration tests passed against Docker, Temporal, and
  Postgres. The Codex test used a deliberately fake API key: a native thread was
  created and closed and two lifecycle events were stored, without an
  authenticated model turn. The readiness command exercised the revised private
  output relay and returned `ready` with exit zero.
- The Codex state volume remained after backend cleanup removed its worker. A
  separate container mounted that volume, verified its dummy auth file, wrote a
  marker, restarted, and verified the marker and unchanged auth bytes. This
  demonstrates file retention, not OAuth refresh or native-session recovery.

## Remaining limits

- No authenticated model turn is established by Go tests or dummy-key login.
- A worker process loss does not reconstruct the ordinary Go workflow stack or
  native adapter's in-memory session map. Persistent storage alone is not
  transparent session recovery.
- Worktree consistency relies on the configured shared mounts, including Git
  worktree metadata and the common object directory.
