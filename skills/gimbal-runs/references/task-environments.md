# Local and Docker workflow and task environments

A workflow remains an independent Gimbal process. `--task-endpoint` sends its
agent sessions, commands, and foreground services to a selected task daemon.
The workflow retains its existing observations and controls. Environment setup,
effect requests, and observation delivery are separate responsibilities; results remain authoritative when event delivery is unavailable.

Start a local daemon with `gimbal daemon --listen 127.0.0.1:8081`. Central can
also include one with `--task-listen 127.0.0.1:8081`. Use a separate daemon when
its work must survive central restarting. Configure providers in the task
daemon's environment. Use loopback or a private container network: the endpoint
accepts trusted execution requests.

Start with the existing `review` workflow and add
`--task-endpoint http://127.0.0.1:8081` to its invocation. The selected daemon
uses its own workspace and `--workdir`. Context and command artifacts move as
bytes; the workflow and task daemon need no shared run directory. Provision
repositories explicitly, using Docker mounts when appropriate. Host provider
login directories are not copied into the container.

The Docker provisioner is a native Go plugin built with the same Go toolchain
and sources as Gimbal:

```sh
go build -buildmode=plugin -o bin/docker.so ./plugins/docker
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/gimbal-linux ./cmd/gimbal
docker build -f plugins/docker/Dockerfile -t gimbal-task .
gimbal environment start --plugin ./bin/docker.so --config /abs/environment.json --receipt /abs/docker-receipt.json
```

Use the Docker engine's architecture for `GOARCH`. The starter image includes
Python, zsh, and the native Pi adapter. Extend it for other agent executables.
The cross-compiled `CGO_ENABLED=0` binary above supports built-in workflows and
tasks, but cannot load native workflow plugins. To load plugins, build the Linux
Gimbal binary and project plugins with matching Go toolchain, dependencies and
build flags, with CGO enabled and a Linux C compiler. A Linux build environment
can run `CGO_ENABLED=1 go build -o bin/gimbal-linux ./cmd/gimbal` after the console
build, then `CGO_ENABLED=1 go build -buildmode=plugin` in the external project.
Install the resulting Linux `.so` files in the image or mount them; host-native
macOS plugins cannot be loaded inside Linux containers.
Published CLIs use `-trimpath`. Build their plugins with that flag and the
released source workspace described in
[distribution](../../gimbal-release/references/distribution.md#plugin-builds).

Configuration names a container, image, available host port, mounts, and only
the environment variables explicitly permitted to enter it:

```json
{"Name":"gimbal-task","Image":"gimbal-task","Port":8081,"Mounts":["type=bind,src=/abs/repo,dst=/workspace"],"Workdir":"/workspace","Env":["DIFFUSION_API_KEY"]}
```

`environment start` prints the endpoint and retains the created container ID
and ownership label in the receipt, including after readiness failure.
`gimbal environment stop --plugin ./bin/docker.so --receipt /abs/docker-receipt.json`
removes only that receipt-owned container and interrupts its work. A same-name
unowned container is preserved. Keep receipts outside Git.
Scope cleanup releases resources without removing the environment. Task-daemon
crash recovery is outside this slice; release active scopes before stopping a
local daemon.

`gimbal run service-check --help` describes the small complete exercise. Use a
fresh work directory per concurrent run. An agent writes a file, `RunCommand`
reads it through the scope-owned HTTP service, and a parent command verifies
that scope exit stopped the service. Scope exit on success, failure, panic, or
cancellation requests daemon cleanup. Cleanup closes owned sessions and stops
owned commands and services, preserving enclosing scopes and other runs.
Failed cleanup remains visible and retryable with `gimbal cancel RUN_ID` or the
console's **Retry cleanup** control. Original workflow failures remain in history.

`PluginDirs`, `Workdir` and `WorkspaceRoot` pass the corresponding daemon
settings using absolute paths **inside the container**. `PluginDirs` may name
multiple directories. `WorkspaceRoot` restricts admitted workflow projects;
`Workdir` selects the repository for task execution. Repositories and plugins
must already exist at those paths through the image or configured mounts.
Use a Docker volume for workflow state on Docker Desktop, for example
`type=volume,src=gimbal-workspace,dst=/workspace`. Keep central and workflow
run directories separate. In a live macOS Docker Desktop check, a host bind
mount for the workflow workspace caused an active runner to be classified as
exited; the same run completed with a Docker volume. Plugin files can still
be mounted read-only from the host.

For a workflow on the host consuming a Docker service, add these settings:

```json
{"PublicHost":"127.0.0.1","ServicePorts":[19091]}
```

Merge these fields into the environment configuration before starting it.
Each service port is published on host loopback with the same container port.
The service command must listen on `0.0.0.0` or another reachable interface,
not just container loopback. Port 8081 is reserved for the daemon. Service ports
must be distinct from the configured host daemon port and one another.

For a workflow in a different container, use an existing private Docker network
shared by the two containers. Configure the service environment with
`"Network":"gimbal-work"` and `"PublicHost":"gimbal-task"`, where gimbal-task
is its container name. Services are reachable through that name within the
network without publishing service ports. Network creation and removal remain
the caller's responsibility. PublicHost is the address reachable by the chosen
consumer; a container's `127.0.0.1` is not another container or the host.

Load the Docker plugin in central and in any workflow daemon using the docker
backend. Use Start's receipt as a route's Configuration, for example:

```json
{"Name":"docker","Configuration":{"ID":"returned-container-id","Label":"returned-owner-label","Endpoint":"http://127.0.0.1:8081","PublicHost":"127.0.0.1","ServicePorts":[19091]},"Environment":"tasks"}
```

Construct this from the actual receipt; do not invent its ID or label. Pass the
route with `--task-route @/abs/task-route.json`; use `--workflow-route` and
`--workflow-project` for a workflow on a daemon. If the route is used from
another container, its Endpoint must also be reachable from that container,
such as `http://gimbal-task:8081` on the configured network. A route Endpoint
and the returned service address serve different consumers.

The Docker backend refuses unpublished service ports before starting the
service. Ordinary commands and `Service` still work without service routing.
`FulfillService` returns an address after launch; the workflow checks readiness.
Scope cleanup stops the service while preserving the environment and other
runs. The default local backend returns loopback in the workflow environment
and uses ordinary native scope-owned services.
