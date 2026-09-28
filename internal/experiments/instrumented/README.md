# Hand-written Temporal workflow specimen

This is the runnable output specimen for a future Gimbal source-to-source
compiler. **There is no transformer here yet.** `plain.go` is the ordinary
Gimbal source counterpart. The existing Gimbal generator reads it to produce
the UI graph; `workflow.go` is the hand-written Temporal orchestration.

The specimen prepares a small Go project with four independent defects. Two
iterations each schedule two concurrent Claude Haiku repairs, join the branches,
run the pair's tests, and pass the check result into the next iteration. A final
suite checks the combined workspace. Each principal has a Pi supervisor.

Temporal owns the ordinary loop, activity futures, joins and error propagation.
Each generated scope has an immediately invoked function and an awaited cleanup
activity in its defer. Branch futures settle after branch cleanup; the group
joins both before closing, and each iteration closes before the next starts.
The activity worker retains keyed scope/resource records, with no suspended
Run/Scope callbacks, current-iteration field, or worker-side Group. Its operation
leases prevent cleanup from closing resources still in use. An internal bridge
shares the ordinary runtime's entry/cleanup implementation; it is not a new
workflow-author API. There is no whole-workflow activity or generic interpreter.

Both branches run in **one activity container and one shared workspace**. Their
assignments name different source files; isolation is by instruction, not a
filesystem sandbox. Fan-in joins work and collects reports; it does not merge
Git branches. Failed work can leave partial edits. Files persist across iterations
and container cleanup in a host bind mount. One prepared image serves both the
control worker and activity worker, with Go and both agent harnesses installed.
The existing filesystem event store and web UI remain the observer/control path.

Supervision uses existing Jev-only code without changing its behavior. The opt-in
integration test below supplies controlled positive Jev HTTP responses, but uses
real Claude events and a real Pi supervisor through the normal supervision path.
It checks the principal context, transcript reference and valid review response;
an empty objections list is acceptable. It does not prove organic detection or
corrective feedback. Normal workflow runs use the real Jev service.

Temporal replay executes no Gimbal recording calls. Automatic activity retries,
worker restart recovery, isolated worktrees, distributed fan-out, adaptive loops,
and a source transformer are outside this specimen. This revision changes activity history; replay earlier histories with their
original revision. Live worker recovery remains unimplemented.

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

Supply `CLAUDE_CODE_OAUTH_TOKEN` (or `ANTHROPIC_API_KEY`),
`DIFFUSION_API_KEY`, and `TYPESAFE_API_KEY` in the control worker's environment.
This machine's Doppler `gimbal/dev_personal` config supplies all three. Neither
credentials nor provider state belong in workflow input, history, source, or Git.

With the credentials already in the calling process environment:

```sh
mkdir -p /tmp/gimbal-instrumented-state
docker run -d --name gimbal-instrumented-control --init \
  --mount type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock \
  --mount type=bind,src=/tmp/gimbal-instrumented-state,dst=/tmp/gimbal-instrumented-state \
  --env SPECIMEN_STATE_ROOT=/tmp/gimbal-instrumented-state \
  --env CLAUDE_CODE_OAUTH_TOKEN --env ANTHROPIC_API_KEY \
  --env DIFFUSION_API_KEY --env TYPESAFE_API_KEY \
  gimbal-instrumented:local -mode control -temporal host.docker.internal:7233
bin/instrumented -task 'Repair the four assigned defects, preserving tests.'
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
doppler run --project gimbal --config dev_personal -- bin/instrumented -mode view -project /tmp/gimbal-instrumented-state/CONTAINER_NAME
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

Tests cover actual parallel scheduling, ordering, carried check results, sibling
cancellation after failure, hosted scope cleanup on root cancellation, and
cross-origin control rejection. Replay checks a separately supplied real history.
Browser legibility and live control claims require observing the real run.

The optional paid supervision integration test can run inside the same prepared
image. Build the test executable for the Docker host architecture, then mount it:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go test -c -o /tmp/instrumented.test ./internal/experiments/instrumented
docker run --rm --init --user 1000:1000 \
  --env HOME=/tmp/agent-home --env SPECIMEN_LIVE_TEST=1 \
  --env CLAUDE_CODE_OAUTH_TOKEN --env ANTHROPIC_API_KEY --env DIFFUSION_API_KEY \
  --mount type=bind,src=/tmp/instrumented.test,dst=/tmp/instrumented.test,readonly \
  --entrypoint /tmp/instrumented.test gimbal-instrumented:local \
  -test.run TestLiveControlledSupervision -test.v -test.timeout 3m
```

This is explicitly a controlled routing check in a separate test container using
the activity image. It substitutes no production behavior and requires no
production supervision switch. Its local Jev HTTP stub receives real principal
events; both agent harnesses contact their actual providers.
