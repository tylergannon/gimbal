# Handwritten Temporal compiler targets

These are source/output examples, not a source compiler. Authored Go stays in
`continuity/plain.go`, `fanout/plain.go`, and `planning/plain.go`.
Their generated Gimbal graphs power the UI; the Temporal counterparts are
handwritten. The examples address results/session continuity, two explicitly
authored parallel branches, and explicit planner/Check expansion. The default
is continuity. The retired four-defect repair example and its dedicated tests
were removed; its historical evidence and replay require revision `b5aeff81`.

Each Generate source site has a separate named activity. Temporal owns the Go
conditions, loops, futures, and joins. Worker scopes own sessions and processes;
activities lease them temporarily. Scope callbacks retain their authored
function boundary. The planner's task iteration uses explicit awaited cleanup.
Small source/target tests cover normal, return, helper-return, labeled and
unlabeled break/continue, caught errors and combined body/cleanup failure.

One prepared image serves one control worker and one activity container per run.
Branches share a workspace; file ownership is an instruction, not enforcement.
Files, immutable context and event records are retained on host bind mounts.
No worktree merge, per-operation migration or native-session recovery is claimed.

Context writes pass encoded JSON before transport, preserving exact integers.
Large results are externalized by a Temporal payload codec into immutable run
storage and decoded into complete typed Go values for consumers. Replay readers
need that same retained storage. Nonzero command exits remain data. Operation
errors preserve message and execution/cancellation/deadline classification;
arbitrary Go error types are not preserved. Check records its implicit context
write in the producing activity, including command cancellation, before release.

Planner expansion shares the runtime's validated plan type, prompt construction,
backlog persistence and loop steering hooks. The workflow selects tasks, schedules
their operations, carries immutable feedback and decides when to stop. No
whole-loop activity is used. Loop/task entry emits the corresponding metadata
before the scope begins, so the UI sees the correct kind and assignment.

Activity retries are disabled. A control-worker restart can replay against a
surviving activity worker. A dead activity container causes bounded failure;
after confirming container removal the control worker settles the run record
with an explicit unconfirmed-cleanup error. It never fabricates resource closes.

## Build and run

Requires Go 1.27.1, Docker, Temporal CLI and the consumer-prepared
`gimbal-browser-evaluator:claude-pi` image. No provider login directories enter
images. Supply the existing provider credentials through the process environment.

```sh
go generate ./internal/experiments/instrumented/...
go build -o bin/instrumented ./internal/experiments/instrumented
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/instrumented-linux ./internal/experiments/instrumented
docker build -f internal/experiments/instrumented/Dockerfile -t gimbal-instrumented:local .
temporal server start-dev --ip 0.0.0.0 --port 7233 --ui-port 8233
```

Use the Docker host's architecture when cross-compiling. The activity worker
currently reaches Temporal through Docker Desktop's `host.docker.internal`.
With `CLAUDE_CODE_OAUTH_TOKEN` (or `ANTHROPIC_API_KEY`), `DIFFUSION_API_KEY` and
`TYPESAFE_API_KEY` already supplied to the calling process:

```sh
mkdir -p /tmp/gimbal-instrumented-state
docker run -d --name gimbal-instrumented-control --init \
  --mount type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock \
  --mount type=bind,src=/tmp/gimbal-instrumented-state,dst=/tmp/gimbal-instrumented-state \
  --env SPECIMEN_STATE_ROOT=/tmp/gimbal-instrumented-state \
  --env CLAUDE_CODE_OAUTH_TOKEN --env ANTHROPIC_API_KEY \
  --env DIFFUSION_API_KEY --env TYPESAFE_API_KEY \
  gimbal-instrumented:local -mode control -temporal host.docker.internal:7233
bin/instrumented -workflow continuity
bin/instrumented -workflow fanout
bin/instrumented -workflow planning
```

`-items-json` supplies exactly two branch assignments; dispatch cardinality is fixed in source.
Dynamic group dispatch is unsupported and remains future work. `-task` supplies runtime context.
The control worker logs each live UI URL. That URL also accepts
`POST /control/steer` with `Run`, `Session`, `Message` and `POST /control/cancel`.
Immediate session steering has no durable delivery promise. Loop steering uses
Gimbal's existing queued planner controls. Cross-origin browser control is refused.

A run's activity container is removed after cleanup. To inspect retained records,
point the viewer at completed workspaces, optionally followed by more paths:

```sh
bin/instrumented -mode view -project /tmp/gimbal-instrumented-state/CONTAINER/workspace
```

The viewer listens on port 8082. Stop it before opening the same project from
another owner. Stop/remove the dedicated control worker when finished.

## Checks

```sh
go test ./internal/experiments/instrumented ./internal/compiledscope ./internal/observation
go test -race ./internal/experiments/instrumented
SPECIMEN_LIVE_TRANSPORT=1 go test ./internal/experiments/instrumented -run TestLiveLargeTypedResult -v
SPECIMEN_LIVE_WORKFLOW=1 go test ./internal/experiments/instrumented -run 'TestLive(Continuity|Fanout|Planning)$' -v
SPECIMEN_LIVE_CONTROLS=1 go test ./internal/experiments/instrumented -run TestLiveSelectedBranchSteering -v
SPECIMEN_LIVE_FAILURE=1 go test ./internal/experiments/instrumented -run TestLiveOperationalBoundary -v
```

The transport test uses deterministic adapter data over real Temporal, without
provider calls. Other opted-in tests use real Claude Haiku and prepared workers.
The failure test stops/restarts the dedicated control container and kills only
its own activity container. Do not share that control worker with unrelated work.
The earlier controlled-supervision test remains opt-in via `SPECIMEN_LIVE_TEST`:
it supplies a controlled positive Jev response but runs real Claude and Pi turns.
It proves dispatch/context access, not organic detection or corrective effect.

Export a history and replay it with the matching code revision and retained store:

```sh
temporal workflow show --workflow-id WORKFLOW_ID --output json > /tmp/history.json
SPECIMEN_HISTORY=/tmp/history.json go test ./internal/experiments/instrumented -run TestRecordedHistoryReplay
```

Do not replay older histories against changed handwritten scheduling. Evidence,
accepted promises and contract details live in Gimbal View's
`ephemeral/static-reassessment/`; passing tests alone do not certify UI legibility.
