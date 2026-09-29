# Experimental Temporal compiler target

Continuity and planning are generated from ordinary authored Go in
`continuity/plain.go` and `planning/plain.go`. `internal/temporalgen` reads their
syntax, Go types and bindings; it does not read visualization graphs or saved
handwritten answers. Generated `*_temporal_gen.go` files contain the workflows,
one activity method per agent/command/planner site, and workflow registration.
The activity worker registers these methods through its existing Activities
registration. Fixed fanout remains handwritten and outside this generator.

Run from the repository root:

```sh
go generate ./internal/experiments/instrumented/...
go test ./internal/temporalgen ./internal/experiments/instrumented ./internal/compiledscope ./internal/observation
```

Deleting the two `*_temporal_gen.go` files before generation is supported. The
entry command for one input is `go run ./internal/temporalgen/temporalgen -dir
./internal/experiments/instrumented/planning -entry Planning -name planning
-output ./internal/experiments/instrumented/planning_temporal_gen.go` (on one line).
The source packages separately generate the graphs used by the UI. Neither
compiler consumes the other's output. The default executable workflow remains
continuity. The retired repair example requires historical revision b5aeff81.

## Bounded source surface

Entries have `(context.Context, gimbal.Env) error` signatures. Supported constructs
are local assignments, exported typed results and struct fields/literals, scalar
literals and package constants, comparisons and arithmetic, if/else (including operation initializers),
explicit error returns, Set/SetJSON, NewSession, Generate, Fork, RunCommand, Check,
inline Scope callbacks with named context parameters, and one Tasks range per
root-level PromiseLoop binding. The task range currently declares its context
and discards its Task value. Generate and PromiseLoop options are not supported.

Go binding objects identify locals, sessions, imports and Gimbal calls; spelling
is not semantic. Source local names remain recognizable in generated variables.
Operation arguments are evaluated in workflow order and passed as activity
parameters. Generate prompts follow the existing constant-prompt rule and are
emitted directly into their per-site activities; dynamic prompts are rejected. Each write encodes its value before the next mutation. Completed
local writes append their keys in generated task control flow, so conditional
writes and newly authored entries require no separate feedback configuration.
Check records its implicit write inside its producing activity, including failure.

Scopes keep their real callback boundary. Task bodies use explicit awaited exit
on normal completion and return, without an artificial callback. Loop entry
occurs at iteration, and cleanup completes before code following the range.
The generated entries return the source's error; earlier handwritten observation
structs are no longer their return contract. Inspect events for results.

The admitted ordinary helper is fmt.Errorf with concrete data values. Opaque
interface arguments (including error wrapping) and data constructors that could
hide custom formatting/JSON behavior are outside this slice. Arbitrary helpers, imported mutable
package values, context values outside recognized lexical operation arguments,
function/closure expressions outside Scope, goroutines, arbitrary loops, named
task range values, break/continue, panic/recover/defer, services, groups and
unsupported options receive source-located diagnostics. Named fractional/complex
constants are rejected; typed string/integer/boolean constants preserve their
Go types. Custom formatting/JSON callbacks and opaque interface containers in replayed
value expressions are rejected, as is variadic helper expansion; the standard time.Duration scalar formatter is admitted. Schema and
validation methods execute in the activity's Generate. This is a bounded
compiler, not an effect analyzer for general Go.

Failed generation replaces the target file with an explicit build failure marker;
stale output cannot be mistaken for a successful translation. Fix source and
regenerate to recover. Identical inputs produce stable formatted output.

Each Generate source site has a separate named activity. Temporal owns the Go
conditions, loops, futures, and joins. Worker scopes own sessions and processes;
activities lease them temporarily. Scope callbacks retain their authored
function boundary. The planner's task iteration uses explicit awaited cleanup.
The paired tests cover generated scope/task returns and cleanup. Separate
handwritten lowering tests cover helper-return and labeled/unlabeled exits; those
support backend semantics, not claims that this compiler translates those exits.

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

The planning pair records the worker result as `implementation`, then mutates
its receipt and records that separately. A conditional `review` write shadows a
parent value only when the worker returns summary `reviewed`. Feedback selects
that key only after its write completes; a skipped branch must not include the
inherited parent value as task-local evidence. These source edits exercise the
translation obligations without changing the planner's `tasks`/`next` schema.

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

`TestSourceVariations` copies a disposable module, deletes and regenerates both
targets, and executes the paired comparison. With generator/backend fixed it
separately renames a context key, adds a write, reverses conditional shadowing,
changes a prompt, changes a command, adds another Generate, renames the loop, and
renames local/import bindings and adds an empty Scope. Each case executes all thirteen paired scenarios,
independently regenerates its source graph, and verifies repeatable generation.
Reverting the edits reproduces baseline output and execution. Expectations change
only through test case inputs; no output/activity/key-list edits are made.
`TestUnsupportedSourceInvalidatesOutput` checks source-located diagnostics,
invalidates old output and verifies recovery after fixing source.


`TestPairedWorkflows` executes the actual authored continuity/planning functions
through Project.Run and their generated targets through Temporal's test
scheduler with real activities. Both use the same deterministic response rules,
real commands and isolated temporary workspaces. It compares prompts, supported
errors, operation/resource order and semantic event fields after removing clocks,
durations and usage and normalizing workspace paths. It also asserts expected
activity names and branch outcomes independently of the comparison.

`TestPairedCheckFailureRecording` compares implicit records after missing-command
and active process cancellation, including the record returned by the producing
activity. `TestInfrastructureFailureStopsBeforeAuthoredOperations` checks the
workflow's handling of failed provisioning. These checks use no paid provider or
Docker container and do not establish fresh live UI or backend-operation proof.


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

Do not replay older histories against changed generated or handwritten scheduling. Evidence,
accepted promises and contract details live in Gimbal View's
`ephemeral/static-reassessment/`; passing tests alone do not certify UI legibility.
