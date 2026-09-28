# Hand-written Temporal workflow specimen

This is the runnable output specimen for a future Gimbal source-to-source
compiler. **There is no transformer here yet.** `plain.go` is the ordinary
Gimbal source counterpart. The existing Gimbal generator reads it to produce
the UI graph; `workflow.go` is the hand-written Temporal orchestration.

Temporal dispatches named activities in order: provision one container,
initialize its Gimbal run, run the command, generate a typed report, finish the
Gimbal run, then release the container. The complete effective task/check data
travels inline. The principal and supervisor execute together inside Generate.
There is no generic action interpreter or whole-workflow activity.

One prepared image serves the control/workflow worker and the activity worker.
The control worker owns Docker provisioning; the activity worker owns one live
Gimbal scope, sessions, event writer and existing web UI. Files persist in a
host bind mount. Temporal replay executes no Gimbal recording calls. Activities
have no automatic retries; native-session recovery is outside this specimen.

Supervision uses existing Gimbal code, without changing timer/Jev behavior.
Successful supervisory feedback is deliberately not an acceptance gate for
this experiment. The parallel Jev migration owns supervision changes.

## Build

Requires Go 1.27.1, Docker, Temporal CLI, and the existing consumer-prepared
`gimbal-browser-evaluator:claude-pi` image (Claude CLI, zsh, project dependencies).
The specimen extends that image with its executable and Docker CLI. It does not
copy provider login directories or credentials into the image.

From the Gimbal checkout root:

```sh
go generate ./internal/experiments/instrumented
go build -o bin/instrumented ./internal/experiments/instrumented
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/instrumented-linux ./internal/experiments/instrumented
docker build -f internal/experiments/instrumented/Dockerfile -t gimbal-instrumented:local .
```

Use the Docker host's architecture in the cross-compile command. The current
specimen uses Docker Desktop's `host.docker.internal` and Temporal port 7233.

## Run

Start a self-hosted development Temporal server:

```sh
temporal server start-dev --ip 0.0.0.0 --port 7233 --ui-port 8233
```

Supply `CLAUDE_CODE_OAUTH_TOKEN` (or `ANTHROPIC_API_KEY`) and
`DIFFUSION_API_KEY` in the control worker's environment. For this machine the
Doppler `gimbal/dev_personal` config supplies Claude and spells the Diffusion
key `DIFUSION_API_KEY`; map that spelling at bootstrap. Neither credentials nor
provider state belong in workflow input, history, source, or Git.

With the credentials already in the calling process environment:

```sh
mkdir -p /tmp/gimbal-instrumented-state
docker run -d --name gimbal-instrumented-control --init \
  --mount type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock \
  --mount type=bind,src=/tmp/gimbal-instrumented-state,dst=/tmp/gimbal-instrumented-state \
  --env SPECIMEN_STATE_ROOT=/tmp/gimbal-instrumented-state \
  --env CLAUDE_CODE_OAUTH_TOKEN --env ANTHROPIC_API_KEY --env DIFFUSION_API_KEY \
  gimbal-instrumented:local -mode control -temporal host.docker.internal:7233
bin/instrumented -task 'Verify the marker and command evidence.'
```

The control worker prints each activity container's loopback UI URL and host
workspace path. The existing UI supports live graph inspection and steering.
The same endpoint accepts `POST /control/steer` with JSON fields `Run`, `Session`,
and `Message`, returning `landed`. `POST /control/cancel` requests Temporal
cancellation. No durable steering queue is added. The browser's existing Cancel
run button cancels the Gimbal scope; that causes the active Temporal activity to
fail and cleanup to run. These are different Temporal terminal statuses.

The worker image is deliberately tied to one run and removed at completion.
Its live URL then disappears. To inspect persisted history afterward:

```sh
bin/instrumented -mode view -project /tmp/gimbal-instrumented-state/CONTAINER_NAME
```

This reuses the existing Gimbal UI on `http://127.0.0.1:8082`. Only open completed
workspaces: the runtime's owner lock prevents two writers from owning one.
Stop the view process with Ctrl-C. Stop and remove the control worker when done:

```sh
docker stop gimbal-instrumented-control
docker rm gimbal-instrumented-control
```

Cancellation/normal completion release the activity container, while host
workspace and event files remain. The specimen does not recover a crashed
control worker's ambiguous provisioning or a lost activity worker's sessions.

## Checks

```sh
go test -race ./internal/experiments/instrumented
go vet ./internal/experiments/instrumented
temporal workflow show --workflow-id WORKFLOW_ID --output json > /tmp/history.json
SPECIMEN_HISTORY=/tmp/history.json go test ./internal/experiments/instrumented -run TestRecordedHistoryReplay
```

Tests cover ordering, inline command output, error/cancellation cleanup, and
cross-origin control rejection. Replay checks a separately supplied real
history. They do not establish successful supervision or browser legibility;
those claims require their own observed behavior.
