# Local and Docker task environments

A workflow remains an independent Gimbal process. `--task-endpoint` sends its
agent sessions, commands, and foreground services to a selected task daemon.
The workflow retains its existing observations and controls. Environment setup,
effect requests, and observation delivery are separate responsibilities; this
delivery uses one concrete HTTP effect path.

Start a local daemon with `gimbal daemon --listen 127.0.0.1:8081`. Central can
also include one with `--task-listen 127.0.0.1:8081`. Use a separate daemon when
its work must survive central restarting. Configure providers in the task
daemon's environment. Use loopback or a private container network: the endpoint
accepts trusted execution requests.

Start with the existing `review` workflow and add
`--task-endpoint http://127.0.0.1:8081` to its invocation. The selected daemon
must see the workspace and run artifact directory at the same absolute paths
as the workflow process. Docker needs explicit bind mounts for both. Host
provider login directories are not copied into the container.

The Docker provisioner is a native Go plugin built with the same Go toolchain
and sources as Gimbal:

```sh
go build -buildmode=plugin -o bin/docker.so ./plugins/docker
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/gimbal-linux ./cmd/gimbal
docker build -f plugins/docker/Dockerfile -t gimbal-task .
gimbal environment start --plugin ./bin/docker.so --config /abs/environment.json
```

Use the Docker engine's architecture for `GOARCH`. The starter image includes
Python, zsh, and the native Pi adapter. Extend it for other agent executables.
Configuration names a container, image, available host port, mounts, and only
the environment variables explicitly permitted to enter it:

```json
{"Name":"gimbal-task","Image":"gimbal-task","Port":8081,"Mounts":["type=bind,src=/abs/shared,dst=/abs/shared"],"Env":["DIFFUSION_API_KEY"]}
```

`environment start` prints the endpoint. `environment stop` with the same
plugin and configuration removes that named container and interrupts its work.
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

Message-bus separation is the next delivery after validating this slice; effect
message transport will be considered separately.
