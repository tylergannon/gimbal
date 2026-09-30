# A consumer-owned Temporal compiler

This is a separate Go module (`example.com/temporal-consumer`). Its source imports
Gimbal's public packages. It owns its Temporal emitter, scheduler, activity
registration, local workspace and process-local scope/session handles. Gimbal
supplies agent execution, context semantics, lifecycle operations and the console.
This is an example to maintain as consumer code, not a supported Gimbal Temporal
backend or a failover system.

The small authored workflow in `authored/workflow.go` records an assignment,
asks an agent for a summary, records the response, and checks a local deliverable
inside a scope. The command creates `delivery.txt` and verifies it. The default
provider is a **deterministic substitute**, which returns a fixed text response;
it demonstrates the compiler, runtime, Temporal and console integration without
claiming fresh model execution.

## Run it

Use the Go version in `go.mod` (including generic method support), a POSIX shell,
and a Temporal development server. The repository development checkout uses a
local Gimbal replacement in this module's `go.mod`; outside the checkout, remove
that replacement and require the published Gimbal revision providing these APIs.
The module includes its own dependencies and generated files.

The example pins published revision `7788f5e9a1ea` as
`v0.12.2-0.20260929211000-7788f5e9a1ea`. To detach a copied example from
this checkout, run these commands in the copied module:

```sh
go mod edit -dropreplace=github.com/tylergannon/gimbal
go mod tidy
go generate ./...
go test ./...
```

To adopt a newer pushed revision, use `go get github.com/tylergannon/gimbal@COMMIT`
and regenerate. The resulting `go.mod` records its exact pseudo-version. Do not use an older
released version that lacks the compiler/runtime surface. Discover its public
contracts directly from the selected module version:

```sh
go doc github.com/tylergannon/gimbal/compiler
go doc github.com/tylergannon/gimbal/contextdata
go doc github.com/tylergannon/gimbal OpenRun
go doc github.com/tylergannon/gimbal Session.GenerateResponse
go doc github.com/tylergannon/gimbal/web Instance.OpenCompiledRun
```

From this directory:

```sh
temporal server start-dev --ip 127.0.0.1 --port 7233
```

In another terminal, from this directory:

```sh
go generate ./...
go test ./...
TYPESAFE_API_KEY=demo-unused go run . -state /tmp/gimbal-temporal-example -port 8084 -keep-open
```

Gimbal requires a nonempty `TYPESAFE_API_KEY` only when a workflow uses
`WithSupervisor` for Jev supervision. This workflow makes no Jev supervision
calls. For real supervision, use a valid configured key.

Open the printed Gimbal console URL, normally <http://127.0.0.1:8084>. Open the
project and its `delivery` run. The graph shows the `coder` agent turn,
`verify` scope and `delivery` check; the assignment and summary context are
available in the run details. The command writes
`/tmp/gimbal-temporal-example/project/delivery.txt`. `-keep-open` leaves the
console serving after the run finishes; interrupt the process to exit. Each
invocation starts one run with a dedicated Temporal queue. Choose a fresh state
directory for a clean example, or reuse it to retain run history. Keep `-state`
outside the module source directory, as in these commands: runtime state contains
sockets and is not input to the source-copying maintenance test.

For a long enough turn to inspect activity and cancel through the console:

```sh
TYPESAFE_API_KEY=demo-unused go run . -state /tmp/gimbal-temporal-cancel -delay 60s -keep-open
```

The whole-run cancel control calls Temporal's cancellation endpoint and then
cancels the local runtime. The activity owner stops admitting work, joins active
operations, closes child scopes and finishes the root. Backend cancellation
acceptance and local cleanup are separate observations; a delivery failure is
not evidence that Temporal accepted cancellation.

For an actual provider turn, use existing configured Claude Code credentials:

```sh
go run . -provider claude -state /tmp/gimbal-temporal-claude -keep-open
```

This selects the public Claude adapter and `claude-haiku-4-5`. No provider login
state is created or copied by this example. The text prompt asks for a summary;
the authored shell check owns creation and validation of the deliverable.

Retained runs can be viewed later without a Temporal server:

```sh
TYPESAFE_API_KEY=demo-unused go run . -view -state /tmp/gimbal-temporal-example -port 8084
```

The context store and host cache are configured again on restart. Keep the
entire state directory, including context objects and project run records, for
as long as retained runs should remain readable.

## Make a workflow change

In `authored/workflow.go`, change the Check key `"delivery"` to
`"maintenance"`, and change both uses of `delivery.txt` in its command to
`maintained.txt`. In `workflow_test.go`, update the intended-output assertions:

```go
const expectedDeliverable = "maintained.txt"
const expectedCheck = "maintenance"
```

These expectations are the behavior you intend to deliver; keep them explicit
instead of deriving them from generated code. Then:

```sh
go generate ./...
go test ./...
TYPESAFE_API_KEY=demo-unused go run . -state /tmp/gimbal-temporal-maintained -keep-open
```

The new run must create `project/maintained.txt`, and its console graph must show
the `maintenance` check. Inspect the generated diffs in
`authored/workflow_gen.go` and `delivery_temporal_gen.go`: both should follow the
same source edit. `TestSourceMaintenance` makes a further distinct
`maintenance-probe` edit in a disposable module, updates its intended-output
assertion, regenerates both outputs, then executes the generated workflow with actual Gimbal
operations and shell execution under Temporal's test scheduler. That test is
not a real-server or browser claim.

Do not edit generated files. `go generate` invokes the consumer's
`cmd/generate`, which first calls `compiler.GenerateGraph`, then independently
loads typed authored Go and lowers it into Temporal scheduling. Graph generation
writes registration source in the authored package. The emitted backend imports
that package even when no result type requires it, so the serving binary
registers the graph without source analysis at runtime.

## Where the behavior lives

| File or public operation | Responsibility |
| --- | --- |
| `authored/workflow.go` | Ordinary authored control flow, prompts and checks |
| `internal/temporalgen` | Consumer-owned typed Go recognition and Temporal emission |
| `compiler.CheckSplitResponse` | Shared admission of the demonstrated split-response result contract |
| `compiler.GenerateGraph` | Independent graph extraction and registration source |
| `delivery_temporal_gen.go` | Visible scheduling, lexical snapshots, scoped cleanup and typed consumption |
| `activities.go`, `continuity_activities.go`, `planning_activities.go` | Consumer scope/session handle resolution, activity leases, drain and error transport |
| `main.go` | Worker/client setup, one local environment, console configuration, provider binding and cancellation delivery |
| `gimbal.OpenScope`, `OpenLoop`, `OpenTask` | Shared scope/task lifecycle, including initialized task context |
| `contextdata.Store`, `Encode`, `Snapshot` | Immutable context publication, verification and retained values |
| `gimbal.WriteContext`, `CheckContext` | Shared authored context/check operations and observations |
| `Session.GenerateResponse[T]`, `gimbal.ConsumeResponse[T]` | One shared agent operation, then typed consumption without another agent call |
| `gimbal.PlanNext`, `RecordPlan`, `TaskFeedback` | Shared planner dispatch, validation/recording and completed-task feedback |
| `web.WithContextStore`, `Instance.OpenCompiledRun` | Console access to retained context, graph registration checks and control routing |

A generated agent operation has this shape (names shortened):

```go
// Activity: resolve the consumer handle, lease the scope, execute Gimbal.
scoped, session, release, err := a.turnInput(ctx, in)
if err != nil { return responseResult{}, err }
defer release()
raw, err := session.GenerateResponse[gimbal.Text](scoped, in.Context, prompt)
return responseResult{Value: raw, Failure: failure(err)}, nil

// Temporal workflow: schedule, wait, then consume the accepted response.
var result responseResult
if err := workflow.ExecuteActivity(ops, "DeliveryGenerate1", input).Get(wait, &result); err != nil {
    return err
}
summary, err := gimbal.ConsumeResponse[gimbal.Text](result.Value, result.Err())
```

The controller carries immutable snapshot references explicitly. A child starts
with its parent's reference and advances its own variable; its writes do not
mutate the parent's snapshot. Scope finish happens after work is joined. Session
handles resolve only in their owning worker process. Gimbal checks reachability;
the consumer does not recreate agent retry, prompt rendering or lifecycle rules.

The retained full emitter also supports the existing bounded scope/fork/planner
specimen; its regression tests remain in Gimbal's `internal/temporalgen` and
`internal/experiments/instrumented` packages. There is one emitter implementation.
The local example's runtime glue is a consumer-owned copy of the reference
integration, not a second implementation of Gimbal semantics.

## Supported input and boundaries

The emitter accepts an entry with `(context.Context, gimbal.Env) error`, named
constant operations, straight-line assignments and supported conditionals,
`Scope`, session creation/forks, `Generate`, context writes, `Check`,
`RunCommand`, and the existing bounded `PromiseLoop` shape. Use the example's
explicit `err := operation(...); return err` style. This is a deliberately
bounded Go compiler: arbitrary helpers/effects, reflection, unsupported loops,
inaccessible constant types and unsupported result codecs produce source-located
diagnostics. A rejected lowering invalidates its previous backend output so it
cannot silently run stale code. Graph diagnostics are not permission to guess
executable behavior.

For structured results, run the shared admission check before emitting split
execution/consumption. It requires demonstrated Polytype-generated validation
and codecs; numeric result fields are currently rejected because validator/Go
decoder range and notation agreement is not established. `gimbal.Text` keeps
this example independent of a result-schema generator.

The example has one local worker and no owner crash recovery, deployment or
cross-process session migration. It uses Temporal's ordinary data converter;
business values and error text may appear in history. Context snapshots are
references to retained local objects, not an all-payload privacy policy. A
consumer adopting this code must choose its own storage/access, timeout/retry,
backend versioning and deployment policies. Adding another infrastructure target
is separate work.
