# Proposed API for consumer-owned compilers

September 29, 2026. Stage 1 design sketch, derived from the
[component ownership analysis](compiler-component-analysis.md). This is a proposal,
not implemented or approved public API. Signatures and snippets have not been
compiled. The inspected Gimbal revision is
`6aee0a14704389e88b4d630318aa3794f022483c`.

Claude Fable 5.1 API consensus reached **only nitpicks remain** in
[round 03](../ephemeral/reviews/compiler-api-round-03.md), after the separate
ownership review converged. The final text incorporates its small clarifications.
This is design consensus, not implementation or browser validation.

## Design choices

Expose typed semantic operations in `gimbal`, where `Session`, `Task`, `Output`,
model bindings and lifecycle state already live. This avoids exporting the
private init-installed callbacks and `any` handles merely to avoid an import
cycle. Keep compile-time Go/Polytype analysis in a separate `compiler` package;
keep snapshot data/storage primitives in a dependency-leaf `contextdata` package.
The existing `web` package supplies hosted entry/control integration.

These are logical package choices to test during extraction, not a new backend
framework. Consumers own AST traversal, lowering, emission, scheduler calls,
resource handles, transport and deployment. The graph remains independently
derived from authored source; it is never the input to backend compilation.

The initial runtime integration has one in-process owner of a run's scope tree
and sessions, as the specimen does. A remote controller schedules operations on
that owner. These APIs do not distribute a scope tree over independent processes
or reconstruct sessions after worker loss. Kubernetes or another scheduler can
host/route that owner without requiring a distributed Gimbal state manager.

## Immutable context store

`Store` is shared snapshot machinery around consumer-supplied physical object
storage. A filesystem implementation remains included. Illustrative signatures:

```go
// Package contextdata.
type ObjectID string
type Snapshot string
type Entry struct {
    Key string
    Value json.RawMessage
    File ObjectID
    Format string
}
type Objects interface {
    Put(ctx context.Context, id ObjectID, data []byte) error
    Get(ctx context.Context, id ObjectID) ([]byte, error)
}
func NewStore(objects Objects) *Store
func FileObjects(root string) (Objects, error)
func Encode(key string, value any) (Entry, error)
func (s *Store) Extend(ctx context.Context, base Snapshot, writes ...Entry) (Snapshot, error)
```

Gimbal computes/verifies content identities and implements manifest validation,
write-time capture, immutable extension, shadow resolution and supported entry
encoding. The physical store must publish a whole immutable object or fail; the
shared store publishes referenced content before its manifest. A missing or
damaged object fails explicitly. Empty snapshot means initial empty input.
`Encode` uses the same value-encoding path as supported authored context writes;
it does not define a second JSON type system. A consumer compiler still diagnoses
unsupported/implicit codec effects for its execution placement.

The submitter uses `Encode` and `Extend` to capture initial input before passing
the snapshot reference to its controller. Generated writes use Gimbal's runtime
context operation below to publish both the new snapshot and the corresponding
runtime observation. A store call alone does not record an authored `Set`.

Storage credentials, namespace and transport live in the consumer's `Objects`
implementation. Neither content addressing nor this interface supplies access
control or confidentiality. Snapshots/blobs and generated prompt/index artifacts
must remain accessible for as long as any retained run references them. Closing
a scope retires its resources, not those retained objects.

## Runtime entry and operations

Configure context access once at run entry. The consumer selects a local
directory visible at the advertised absolute path to its principals and
supervisors. Gimbal materializes/verifies complete values there and renders local
paths, including budgeted index files. Immutable objects remain separate from
mutable materializations; the consumer supplies read-only access where practical.

```go
// Package gimbal. Existing Output, AgentOption, Task and binding types remain.
type ContextAccess struct {
    Store *contextdata.Store
    LocalDir string
}
func OpenRun(ctx context.Context, name string,
    models map[WorkflowRole]ModelBinding, initial contextdata.Snapshot, access ContextAccess) (
    context.Context, func(error) error, error)
func OpenScope(ctx context.Context, name string) (context.Context, func(error) error, error)
func OpenLoop(ctx context.Context, name string) (context.Context, func(error) error, error)
func OpenTask(ctx context.Context, name string, task Task,
    inherited contextdata.Snapshot) (
    context.Context, contextdata.Snapshot, func(error) error, error)
func CancelRun(ctx context.Context, cause error) error

func WriteContext(ctx context.Context, base contextdata.Snapshot,
    writes ...contextdata.Entry) (contextdata.Snapshot, error)
func CheckContext(ctx context.Context, base contextdata.Snapshot,
    key, dir, command string, args ...string) (contextdata.Snapshot, error)

func (s *Session) GenerateResponse[T Output](ctx context.Context,
    input contextdata.Snapshot, prompt string, opts ...AgentOption) ([]byte, error)
func ConsumeResponse[T Output](raw []byte, operationErr error) (T, error)

type Plan struct {
    Tasks []Task `json:"tasks"`
    Next polytype.Nullable[int] `json:"next"`
}
func PlanNext(ctx context.Context, input contextdata.Snapshot, planner *Session, goal string,
    tasks []Task, previous string, opts ...AgentOption) (Plan, error)
func RecordPlan(ctx context.Context, goal string, plan Plan) error
func TaskFeedback(taskCtx context.Context) (string, error)
```

These operations retain the existing local implementation's mechanics. New
names expose split execution rather than wrap a workflow tactic. Ordinary
workflows still use `Run`, `Scope`, `PromiseLoop`, `Check` and `Generate[T]`.

- Entry returns a live process-local context and a finish function. One owner
  finishes once after cancelling/joining its outstanding work and closing child
  scopes. Gimbal checks legal scope/session use and ended state; consumer leases
  stop new admissions while draining. No retry-idempotent close is promised.
- `OpenRun` validates and records the submitted initial snapshot as root input,
  using shared snapshot loading and context recording internally. The controller
  keeps that same initial reference as its root snapshot. Initial input therefore
  crosses orchestration as a reference while still appearing in observations.
  Failure during entry cleans up the partial run and returns no live handle.
- `OpenTask` combines task scope entry and the task-context write, returning the
  initialized child snapshot. The consumer does not emit another `Set("task")`.
  If initialization fails after entry, Gimbal finishes the partially opened scope
  and returns the joined failure with no live handle for the caller to close.
- `WriteContext` updates the snapshot and records the authored write. The caller
  advances only its current lexical snapshot variable. Parent references remain
  unchanged; binding does not reconstruct values from live ancestor metadata.
- `CheckContext` shares local Check's preconditions, command execution and canonical
  result construction. It returns the updated snapshot when recording succeeds,
  including when the command fails. If publication fails, it retains the input
  reference and returns the failure; it never presents an empty replacement as a
  successfully recorded result. Observation failure remains visible too.
- `GenerateResponse` uses the shared agent operation. `ConsumeResponse` performs
  supported typed consumption with no agent calls. Compile-time admission proves
  the bounded split-response contract; the local `Generate[T]` retains its broader
  decoding/re-ask/partial-result behavior. The reference emitter's supported syntax
  is not widened just because these runtime methods accept existing options.
- Every operation that consumes effective scoped data takes its immutable
  snapshot explicitly: `GenerateResponse` and `PlanNext` bind it internally before
  rendering, including supervisors. Empty input explicitly selects the empty
  snapshot. There is no public ambient `BindContext` prerequisite and no fallback
  to live ancestor values on this compiled path. This prevents an omitted bind;
  choosing a wrong but valid reference remains the compiler's responsibility.
- `Plan` exposes the runtime's existing plan shape rather than a second consumer
  wire type. `PlanNext` dispatches once; `RecordPlan` validates and records it;
  the consumer schedules repetition and the task body. Separate plan/record calls
  retain the current scheduling/failure boundary. `PlanNext` takes its loop name
  from the loop scope opened by `OpenLoop`; a non-loop context is an error.
- `TaskFeedback` selects the completed task's local recorded writes before task
  cleanup. It does not select potentially written keys or subtract snapshots.
  In this initial one-owner integration, the next planner uses that same execution
  environment and retained materialized files. Moving this rendered string to
  another agent filesystem requires rematerializing referenced content; it is not
  a portable representation of task feedback by itself.
- `CancelRun` records/cancels the root before finish. It is idempotent for this
  cancellation transition: the first effective cause is retained, and repeated
  calls while live create no additional root-kill event. An ended run rejects new
  cancellation rather than inventing a new event. This does not make finish
  retry-idempotent. Operator attribution and request ordering follow the hosted
  protocol below. A cancellation-shaped body error alone is insufficient.

Session creation and forks continue to use existing `NewSession` and `Fork` on
those contexts. The consumer keeps serializable handles and resolves them to
actual sessions in the owning process. Gimbal verifies session reachability from
the active scope. Context references can outlive a scope; session handles cannot.

## Hosted runs, retained artifacts and controls

The current `gimbal.Project` context alone does not join the console's live
runtime. Provide a narrow method on the existing web instance instead of requiring
consumers to name `internal/host.Project`:

```go
// Package web.
func WithContextStore(project string, store *contextdata.Store, cacheDir string) Option
type CompiledControls struct {
    CancelRun func(context.Context, gimbal.Killed) error
    DeliveryTimeout time.Duration // zero selects 5 seconds; negative is invalid
}
func (i *Instance) OpenCompiledRun(ctx context.Context, project, name string,
    models map[gimbal.WorkflowRole]gimbal.ModelBinding,
    initial contextdata.Snapshot, localDir string, controls CompiledControls) (
    context.Context, func(error) error, error)
```

`WithContextStore` configures host-side access when constructing/starting the web
instance, including when serving old runs after a restart. The host cache need
not have the same path as the worker's local directory. Gimbal's context-artifact records
and handlers resolve logical object references through the configured project
store rather than an assumed symlink. The consumer must retain access to the
objects referenced by that project's retained runs; worker-local caches alone
do not satisfy this contract. Existing hosted project storage still retains run
records and command-output files; this context-store API does not automatically
upload arbitrary files or turn all observation storage into an object store.

Hosted entry uses exactly the project's `Store` registered by `WithContextStore`;
it accepts no second store argument and rejects missing/duplicate project-store
configuration. `localDir` is the agent-visible materialization directory; the
option's `cacheDir` is for the web host. These are two distinct physical uses,
not store-identity comparisons. Hosted entry records `initial` just as `OpenRun`
does, attaches observation/control hooks, and participates in instance shutdown.
It opens one run; do not also call `gimbal.OpenRun` for that same execution.
The advertised compiled workflow must have its authored graph registered in the
serving binary. Hosted entry rejects a missing graph with a useful diagnostic.
Validate this with the production binary, not a test binary's additional imports.

The cancellation callback is for the actual console/host control path. It is
bound to the consumer's backend run when the hosted run is opened. Require it for
a hosted compiled run; otherwise reject entry rather than silently falling back
to a console control that stops only the local runtime.

On a console request, Gimbal reserves the operator cause for that run and invokes
the callback with a bounded delivery context before cancelling local active work.
The consumer callback must honor that deadline; backend unavailability cannot
block the local stop indefinitely. After the delivery attempt, `CancelRun` stops
the local root **even if delivery failed, timed out, or the backend is already
closed**. Backend-first ordering handles the normal case without making local
activities fail before the request is sent; it is not a prerequisite for local
cancellation when delivery fails.
Gimbal owns enforcement of this bound. The consumer can configure a positive
`DeliveryTimeout`; zero selects a five-second initial default. Timeout checks use
an explicit short test duration. The control acknowledgement waits at most for
this delivery attempt before initiating local cancellation; it does not wait for
all cleanup. The UI must show cleanup as pending until its outcome is observed.

If a backend notification races that bounded attempt, runtime `CancelRun` uses
the reserved operator cause. Once the attempt finishes or times out, that cause
becomes the local cancellation cause and the temporary reservation ends. There
is no indefinite pending attribution: the event records who requested the local
stop, not a claim about who stopped the backend. Repeated notifications converge
on one first-cause root-kill event. A later `FinishCancelled` with generic
`context.Canceled` cannot erase attribution or record another kill.

Record backend delivery separately from the local stop: confirmed acceptance on
a nil callback result, or unconfirmed on error/timeout (including a rejection).
The returned error can explain the failure without a new universal backend error
taxonomy. A failed delivery cannot be reported
as confirmed cancellation merely because the local root stopped. Allow another
delivery attempt while relevant; root-event deduplication must not suppress it.
Backend-originated cancellation with no console request uses its own cause.

The next implementation should record a Gimbal cancellation-delivery observation
(proposed event name `CancellationDelivery`) containing accepted/unconfirmed
delivery and any error detail, separate from the local root's `Killed` event.
Show an unconfirmed result beside the run's cancellation status and retain it in
history; a hidden log entry is insufficient. Later live UI evidence must include
delivery failure/timeout as well as successful cancellation. The event's final
wire shape belongs to the implementation, not to the consumer emitter.

The consumer runtime owner must react to its root context cancellation locally:
stop lease admissions, cancel/join active work, close children innermost first,
and finish the root once, even if the controller never schedules cleanup. Its
local drain and controller-triggered finish share the consumer's single lifecycle
owner; they cannot both invoke the raw finish function. The backend's environment
release remains consumer-owned, with unavailable cleanup reported as unconfirmed.
This is a worker-local cancellation/drain obligation, not crash recovery. A dead
owner still cannot claim that it cleaned up resources.
A controller cancel/finish arriving after that owner completed local drain returns
the recorded outcome, including real cleanup failures, rather than calling the
raw lifecycle functions again or inventing an already-ended error. The consumer
owner provides this completed-outcome lookup; raw Gimbal finish remains single-use.

Test the combined console → backend → runtime/finish sequence and response race,
then a terminated backend and a delivery timeout while local work is active.
Demonstrate local drain and truthful backend/cleanup status without controller
cooperation. Also check delivery retry and direct backend cancellation. Testing
the two successful directions separately misses the original defect.

Whole-run cancellation is required. Tyler questions per-task cancellation and
may remove it; this surface omits scope/task kill. Turn stop and steering retain
their existing live-runtime meaning. Do not expose private host objects as an
alternate supported way to bypass the compiled-run control integration.

## Compiler checks and graph generation

```go
// Package compiler; pkg includes dependency syntax and Go type information.
func CheckSplitResponse(pkg *packages.Package, result types.Type, at token.Pos) error
func GenerateGraph(dir, entry, name, output string) error
```

`CheckSplitResponse` extracts the demonstrated admission checks from the current
emitter: matching Polytype schema/validation/codecs, admitted shapes, nested
methods and known validator/decoder hazards. The generated-file ownership marker
remains a trusted-source convention, not a security boundary. Arbitrary formatting
effects and Temporal replay policy stay in the consumer. A successful check is
not a guarantee for every Go/Polytype type or for arbitrary subsequent transport.

`GenerateGraph` factors the existing extraction, graph-literal serialization and
registration-source emission out of the stock CLI/Form assembly. It writes a Go
file in the authored package (`output` is relative to `dir`) containing the graph
and its `gimbal.RegisterGraph` init. The consumer invokes it at build/generation
time and imports the authored package in the serving binary, even when its backend
output otherwise references none of that package's types. It needs neither the
source tree nor Go analysis at runtime, and does not write its own graph serializer.
Preserve the example's one-workflow-per-authored-package arrangement initially.
Rewrite this generator's own previous output during ordinary regeneration;
diagnose foreign/handwritten output or another workflow's conflicting registration
instead of overwriting it. Keep an ownership marker and workflow identity in the
generated file; neither is a security boundary against deliberately falsified source.
Backend lowering reads typed Go separately. An extractor diagnostic is an honest
graph hole, not permission to guess an executable lowering.

Consumers can own the small `go/packages`/`go/types` callee-recognition code. Gimbal
supplies operation rules and the example; no universal AST framework or IR is
proposed. The example must also fix or reject inaccessible private named constants.

## Consumer call paths

These fragments illustrate placement, not a complete compilable consumer module.
`acquireTurn`, `TurnInput`, `Response`, `response` and `encodeFailure` are consumer
types/functions. Its compiler checks `Report` before emitting this worker method.

```go
func (a *Activities) GenerateReport(ctx context.Context, in TurnInput) (Response, error) {
    scoped, session, release, err := a.acquireTurn(ctx, in)
    if err != nil { return Response{}, err }
    defer release()
    raw, operationErr := session.GenerateResponse[Report](scoped, in.Context, reportPrompt)
    return response(raw, encodeFailure(operationErr)), nil
}

// Controller, after awaiting the backend's scheduled operation:
report, err := gimbal.ConsumeResponse[Report](out.Bytes, out.OperationError())
if err != nil { return err }
if report.Summary == "edit" {
    // Authored branch and subsequent scheduled operations.
}
```

For a scoped body, the worker opens the child using `OpenScope`; the emitted
controller copies the parent's snapshot reference into a separate child variable.
Writes advance only that variable. On every exit, the consumer joins work and
awaits the child's finish before resuming the parent with its original snapshot.

For a planner task, the controller awaits one `PlanNext` with the loop's captured
effective snapshot (the root reference in the current example), records the decision,
then calls a scheduled worker operation using `OpenTask`. It receives the task's
initialized snapshot and a consumer-owned scope handle. The body runs with that
snapshot; the controller awaits `TaskFeedback`, then task cleanup, before the
next decision. Gimbal constructs prompts, Check records and local feedback; the
consumer's generated loop decides which operation to schedule next. The next
`PlanNext` again receives the loop snapshot, not the completed task snapshot;
task feedback travels in the distinct `previous` argument.

The consumer can transport typed values or references rather than raw response
bytes, provided the supported value/error behavior survives. Reference resolution
belongs at a backend-appropriate boundary; Temporal replay cannot perform
unrecorded file/network reads. The current overflow codec is not a policy for
keeping every business input/result/error out of orchestration history. That
future target policy remains consumer-owned.

## What the next stage must demonstrate

Refactor privately against the current example first. Exercise actual entry,
operation, task-feedback, cleanup, cancellation and artifact call sites before
freezing names. Preserve the existing paired checks and add cases for changed
boundaries, including production graph registration and the real console control
command. A filesystem implementation plus a small second test object store can
test the storage seam without a cloud driver project.

Clean downstream imports, a complete runnable consumer example and essential
maintenance instructions are the following publication stage. This sketch does
not claim a working API, downstream build, browser result or new backend.
