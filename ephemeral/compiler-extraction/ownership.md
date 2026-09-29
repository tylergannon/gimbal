# Compiler components and the consumer boundary

September 29, 2026. Stage 1 of the [consumer plan](compiler-consumer-plan.md).
This is a design proposal for review, not an approved public API or an
implementation report. Source inspected at Gimbal
`6aee0a14704389e88b4d630318aa3794f022483c`; the implementation checkout is unchanged.

## Recommendation

Publish access to Gimbal's existing semantic operations, together with narrowly
useful compiler checks and instructions. Keep the compiler's traversal, lowering,
emission, scheduler integration and infrastructure in consumer code. A shared
workflow IR, backend plugin registry and supported emitter collection are not
needed to obtain this boundary.

The important extraction is more than making `compiledscope` public. Its current
function variables avoid an import cycle, its context store assumes a filesystem
layout, and several shared behaviors still live in the specimen's activity code.
Those are useful experimental seams, but poor public contracts as they stand.

The existing direction is sound. The next implementation should make the local
and compiled paths share the remaining semantic mechanics, while leaving their
control flow explicit. The public surface can then follow the working call sites.

## Ownership map

“Extract” means reuse/refactor the implementation, not introduce a second engine.
“Obligation” means instructions plus behavioral examples: an arbitrary consumer's
emitter cannot be made correct just by calling a library.

| Responsibility and current location | Gimbal responsibility | Consumer responsibility | Treatment |
| --- | --- | --- | --- |
| Agent operation: `session.go`, supervision code, `compiled_generate.go` | Scoped prompting, validation/re-asks, supervision, harness/session behavior, attempt recording | Schedule the operation, locate its live session, transport its outcome | Extract a typed callable response operation over the existing engine |
| Scope/run lifecycle: `compiled_scope.go`, `scope.go`, `run.go` | Entry/exit meaning, enforce session reachability and legal scope lifetime, cleanup behavior, lifecycle observations | Arrange remote entry/exit, cancel and join outstanding work, handle unreachable workers | Expose explicit entry/finish and shared legality checks; document scheduling obligations |
| Worker resource table: `instrumented/activities.go`, `continuity_activities.go` | Check semantic legality against the runtime's scope/session state | Handles, worker affinity, leases, heartbeat, registries and recovery | Extract legality checks; keep physical lookup and drain coordination in consumer |
| Context: `compiled_context.go`, `compiledscope/context.go` | Define capture/inheritance/shadowing; implement immutable extension, value encoding, rendering, materialization into configured local roots and logical artifact resolution | Correct lexical snapshot threading, submission input capture, physical object storage, agent-readable/protected local directories, UI store access and retention for retained runs | Shared snapshot operations plus explicit compiler and storage obligations/checks |
| `Check`: `command.go`, `instrumented/planning_activities.go` | Canonical command evidence, record even on execution failure, key rules, context publication | Schedule command and return snapshot/outcome | Reunify the command/result path before publishing it |
| Planner dispatch: `loop.go`, `compiled_loop.go` | Prompt, validation, steering delivery, backlog/decision recording, task setup and completed local feedback | Repeat, schedule selected task body, transport decisions and feedback | Extract shared single-step mechanics; leave iteration in generated code |
| Result admission: `temporalgen/result.go`, parts of `valueMethods` | Check the supported split-response validator/decoder agreement | Decide where decoding occurs; enforce backend-specific replay/effect restrictions | Extract a bounded checker; retain target policy checks |
| Source lowering: `temporalgen/generate.go` | Describe language semantics and operation contracts | Recognize typed calls, support/reject syntax, preserve Go decisions, emit target code | Instructions/example first; no general shared lowerer yet |
| Visualization: `workflow/graph.go`, `internal/generate`, `RegisterGraph` | Extract and interpret workflow shape; define correspondence with observations | Include the authored graph and preserve workflow names, scope/session meaning and operation order | Supported graph-generation entry point and registration check; graph remains independent of compilation |
| Hosting and controls: `internal/host`, `web`, specimen `main.go` | Attach runtime observations and live controls to Gimbal's console | Route controls to the right worker/controller; transport, authentication and deployment | Expose a narrow hosted-run entry point; retain infrastructure routing in consumer |
| Backend cancellation: `compiled_scope.go`, `run.complete`, specimen `FinishCancelled` | Record cancellation and its origin distinctly from failure; retain cleanup outcomes | Cancel scheduled work, propagate the cancellation cause, await cleanup and report worker loss | Expose cancellation marking; specify control routing |
| Failure transport: `continuity_activities.go` | Specify operation outcomes and required error distinctions | Encode/decode them for the backend and distinguish infrastructure failures | Document/test the bounded contract; do not standardize one wire envelope now |
| Provisioning and payloads: specimen `main.go`, `payload.go`, `workflowStart` | No scheduler or deployment policy | Containers/pods, queues, retries, deadlines, business-data placement, rollout | Keep consumer-owned |

Paths abbreviated with `instrumented/` above are beneath
`internal/experiments/instrumented/` in Gimbal.

## Three concrete execution paths

### Generate

The current compiler recognizes the actual Gimbal callee using Go type
information, admits the result type, and emits a distinct activity for the source
call. That activity obtains a scope lease, binds the supplied immutable context,
and resolves the session handle. `compiledscope.Generate[T]` reaches
`Session.generateResponse`, which performs the shared operation. The activity
returns accepted bytes plus the target's operation-failure envelope; controller
code calls `Consume[T]` before the authored branch, field access or mutation.

Keep the lease, handle lookup, activity and envelope in the consumer. Replace
the private callback with a typed Gimbal call. Move the common result-admission
checks out of the Temporal emitter so another compiler using this split does not
have to discover the validator/decoder gaps again.

The local `Session.Generate[T]` continues to decode during acceptance because it
supports a broader output contract, including decode re-asks and partial values
on failure. Publishing a byte-returning operation must not silently narrow that
local API. The split facility's supported contract is narrower and explicit.
Typed transport is also permitted; the consumer must account for extra codec
invocations and preserve the supported result/error behavior.

### Scoped operation

In Continuity, the parent session is created in the root scope. A child scope
shadows the `layer` context value, uses the parent session, and creates its own
fork. The generated child carries a child snapshot without changing the parent's
snapshot. Closing the child closes its fork; the parent session remains usable.
That snapshot threading is performed by the emitted controller, not inferred by
Gimbal. Binding a valid but wrong snapshot currently succeeds. The compiler must
copy the parent reference on entry, advance only the current lexical scope's
reference after a write, and resume the unchanged parent reference on exit.
Operations receive the reference captured at their authored scheduling boundary.
Gimbal owns immutable extension and rendering; the consumer implements this
explicit lowering rule. Check stale-shadow and child-to-parent leakage cases.
Keep capture-time values authoritative; binding validates manifest/blob integrity,
not current live-ancestor membership. This is a deliberate limit on runtime
detection, rather than a claim that no provenance check could ever be written.
The [approved snapshot contract](https://github.com/tylergannon/gimbal-view/blob/fd9d5e5/ephemeral/static-reassessment/context-snapshot-contract.md)
requires resolution independent of live ancestor objects, makes provenance
optional, and explicitly assigns lexical reference threading to generated code.
`TestCompiledSnapshotRenderingIndependentOfLiveAncestors` also erases ancestor
metadata and binds a retained child's complete snapshot after that child closes.
That is a storage-resolution test, not permission for an authored parent operation
to consume the child's injected context. A mandatory live-ancestor membership
check would break the demonstrated resolution independence. A separately persisted
ownership/capture record could support stronger checks,
but is additional authority/lifetime machinery, not a free reuse of observation
metadata. It would still need to know which source scheduling point selected the
reference to distinguish all valid old captures from stale ones.

For this extraction, retain the approved design: the library guarantees the
contents of a valid snapshot; the compiler guarantees selecting the right one.
A wrong but valid snapshot can therefore still leak a task's data into a parent
prompt. Demonstrate that the emitter restores the parent reference and does not
thread completed-task context into the next planner's current input. Optional
diagnostics may use provenance, but removing observation metadata must not change
execution. Stronger runtime capture-authority checking would need its own agreed
contract, rather than silently changing this one.

Gimbal already owns that session creation/fork/close behavior. The specimen owns
the handle table and checks that a session's owner is the current scope or an
ancestor. Its activity lease prevents closing a scope while an operation still
uses it. Exit refuses an unclosed child, stops new leases, cancels existing work,
joins it, and only then calls Gimbal's finish operation.

Move semantic legality checks into Gimbal's operation/lifecycle entry points:
session use must be reachable from the active scope, a parent cannot finish with
an open child, and operations cannot enter a scope the runtime has marked ended.
The runtime already has the scope tree and owned sessions needed for
these checks. The consumer keeps physical handle lookup and stops admissions to
its lease table before cancelling/joining remote work. This closing/admission
state belongs to its lease table; it is not an existing Gimbal scope state.
It must then await finish.
That coordination does not require a shared distributed resource manager, but it
also does not justify copying the runtime's legality rules into every target.
A live Go context,
session pointer and finish function stay in their owning process; none is a
transportable resource or a worker-recovery mechanism.

The finish function combines the body and relevant cleanup outcome. Session-close
failures are aggregated at the run boundary in the existing runtime; callers must
not accidentally change that by making all cleanup errors task failures. The
consumer wire representation needs to retain the distinctions its supported
source can observe. The current envelope proves cancellation/deadline identity
and execution messages, not arbitrary Go error identity or every `errors.As` use.

### Planner task

The generated Planning workflow explicitly repeats: plan once, record the
decision, select the task, enter its scope, publish its task value, run the body,
collect its local feedback, close the task, then plan again. The planner session
persists; each worker session is owned by its task scope.

`compiled_loop.go` already shares prompt construction, validated planner dispatch
and steering queues with runtime code, but duplicates parts of local loop setup
and decision recording. The specimen additionally defines a second plan wire
shape and filters feedback by a list of keys collected by the emitter.

Reunify decision validation/recording and task-local feedback as Gimbal operations.
The consumer still carries the revised backlog and feedback between decisions,
and its emitted loop visibly schedules the body. Gimbal need not execute that
loop inside an activity. Keep deciding and recording separate for now: they are
already separate scheduled operations, and combining them changes failure and
history boundaries without being necessary for this extraction.

Local `PromiseLoop` currently has special behavior for an operator killing one
task: record its outcome and feed the reason to the next planner unless the loop
itself was cancelled. The generated target's correspondence is unproved. Tyler's
subsequent clarification questions this feature's semantic value and favors
considering its removal. Do not make task-kill continuation a requirement of this
extraction or design public compiler APIs around it. Preserve whole-run
cancellation; assess removing per-task controls as a separate product decision.
No existing control is removed by this analysis. The proposed public hosted
compiled-run surface omits scope/task kill; it is not a supported back door
through the internal host object.

## What needs refactoring rather than a rename

**Command evidence.** Local `Check` checks key availability before execution and
builds its record from the runtime's `CommandStarted`/`CommandEnded` values.
The specimen calls `RunCommand`, recreates the record using input arguments, and
publishes afterward. This duplicates behavior: for example, the local record uses
the resolved absolute working directory, while the specimen records the supplied
directory. The paths also place duplicate-key rejection differently. These are
source-established differences, not newly executed runtime reproductions.
Both paths should use the same check preparation/result/publication logic, with
compiled execution returning the resulting snapshot even when the command fails.

**Feedback.** Local `scope.localText` selects values owned by the completed task.
The specimen loads a full snapshot, filters keys supplied by the compiler, then
renders through a private hook. That implementation currently handles the tested
conditional writes, shadowing and implicit Check results, but makes each compiler
responsible for part of Gimbal's feedback semantics. Prefer a shared operation
that uses the task's recorded local writes. Do not infer local writes by subtracting
parent/child snapshots: a local shadow may have the same value as its parent.

**Context storage.** The current store combines immutable manifests, hashing,
filesystem publication and local materialized files. `configureCompiledStore`
also creates a symlink under the run directory for UI artifact access. Separate
the shared snapshot/value semantics from object reads/writes, and keep a built-in
filesystem implementation. The consumer supplies the materialization root and
makes it accessible at the advertised absolute path inside the principal's and
supervisors' execution environments. Gimbal writes/verifies complete local files
there, renders those local paths, and repairs materialized copies from immutable
objects before dispatch. The consumer provides read-only agent access where
possible, or separates mutable materializations from immutable backing objects.
A path readable by the Go process alone is insufficient if the agent is sandboxed
elsewhere. This requires a per-environment visibility arrangement, not identical
paths throughout the organization.

The consumer also configures the web host's access to the same logical objects
and its own local cache/materialization directory. Gimbal owns resolving retained
artifact references through that configured access; the old symlink cannot be
the universal resolver. Host access must be configured when serving retained runs,
including after a restart, rather than existing only on a live activity context.
These are required inputs to the API sketch, with exact signatures settled in
the following API review. No cloud-provider driver library is proposed.

The consumer owns storage implementation and retention policy subject to a floor:
all snapshots/blobs and generated prompt/index artifacts named by retained runs,
turns, supervisors or the UI remain available after scope cleanup. Run retirement
may retire those references together; a bucket TTL must not silently expire them
while the run is retained. Local caches may be recreated from that retained store.
This obligation is distinct from retaining live sessions or promising worker
crash recovery.

**Hosting.** `gimbal.Project` explicitly does not attach a run to a web instance.
The specimen uses `host.Project.OpenCompiledRun` to install observation and live
control hooks and participate in instance shutdown. `web.Instance.Owner` exposes
an internal host type; some methods can be called through inferred values, but
consumers should not need to name private host types or depend on their internals.
A supported hosted-run method should hide those mechanics.

## API proposal

The separate [API sketch](compiler-api-sketch.md) turns these ownership boundaries
into proposed signatures and consumer call paths. Names remain provisional;
implementation and public API approval follow the design check-in. Ownership
consensus with Claude Fable 5.1 reached **only nitpicks remain** in
[round 04](../ephemeral/reviews/compiler-ownership-round-04.md). The API has its own
subsequent review; ownership consensus is not API or runtime validation.
That separate API review subsequently reached **only nitpicks remain** in
[round 03](../ephemeral/reviews/compiler-api-round-03.md). Both proposals are ready
for the design check-in; neither is implemented or approved for publication.

## Observation and control boundary

Authored graph extraction already exists. `workflow.Graph` explicitly omits ordinary
program calculations; its nodes are associated with runtime paths through names
and ordinals. Run pages find the graph by the registered workflow name. Preserve
that association and the runtime's operation observations. A Temporal activity
name such as `PlanningGenerate1` is not a new universal UI node identity.

The graph and backend compiler independently read the authored Go. The compiler
does not consume the visualization graph. A consumer build must register the
authored graph in the binary serving the relevant run; source filename comments
in emitted code alone do not connect a run to the UI.

Ownership review exposed a concrete exception in the reference example:
`go list -deps ./internal/experiments/instrumented` does not include the authored
`planning` package, while `go list -test -deps` does. Its graph registration runs
only in that package's generated `init`. The production binary therefore lacks
the Planning graph even though the test binary includes it. Both reviewer and
coordinator reproduced this dependency result at the inspected revision. This
supersedes the earlier review's statement that no disconnection had been found;
it is a graph-registration defect, not a browser observation. Include/import the
authored graph regardless of whether generated code needs one of its Go types.
Check registration from a production build, without test-only imports, before
starting a compiled workflow advertised as visualizable.

Gimbal owns what steering, cancellation and cleanup records mean. The consumer
owns how an operator request reaches a session, loop or backend run. Examples and
checks should distinguish a queued message from a landed message, a stop request
from completed cleanup, and an unavailable worker from a successfully closed run.
Fresh browser observation of the latest generated path remains outstanding;
the package-level registration defect above is already established.

There is also a source-established cancellation-routing defect. The specimen mux
provides literal `/control/cancel` and `/control/steer` endpoints, but the console
uses skgo remote commands. Its `cancelRun` command directly calls the worker
runtime's `CancelScope`; that route passes through the specimen's proxy and does
not call Temporal `CancelWorkflow`. The custom cancel endpoint's tests therefore
do not prove the actual console-to-backend path. The runtime is cancelled, but the
backend does not receive that cancellation request through this route. Exact
terminal states and visual behavior have not been observed. Steering and turn
stopping can appropriately stay in the live runtime; whole-run cancellation needs
the run-level backend hook in the API sketch. Test the actual console command path, not just the
specimen's auxiliary endpoint.

| Control | Consumer routing | Gimbal meaning |
| --- | --- | --- |
| Cancel whole run | Stop backend scheduling and active work; deliver cause to runtime root; await finish | Cancellation with origin, followed by truthful cleanup/terminal outcome |
| Kill scope or task | No new compiler integration required in this proposal; candidate for product removal | Existing local behavior is not a promised compiled-target capability; removal remains a product decision |
| Kill current turn | Reach the live runtime turn; preserve the containing session/scope | Turn outcome, not automatic whole-run cancellation |
| Steer session | Reach its live runtime/provider adapter | Truthful landed/rejected response and observation |
| Steer loop | Reach the loop's pending message queue | Queued messages land at the next decision or are recorded as dropped on closure |
| Answer interview | Reach the runtime's waiting question if that operation is supported | Answer the identified live question; current compiler does not support Interview |

This table defines the integration obligations; it does not claim every control
has been demonstrated through the generated target. A consumer must state its
supported controls and must not silently present an ineffective control as working.
Tyler explicitly requires whole-run cancellation and questions whether per-task
cancellation adds enough value to justify its semantics and development cost.
The proposed hook is therefore run-only. Existing turn-stop behavior is a distinct
control; this clarification does not by itself decide its removal.

## Proposed next implementation boundary

Use the current Temporal/Docker example to test the separation before publishing
names. The next stage should achieve these results:

- Typed runtime entry points replace private global callbacks; local and compiled
  operations still use one agent engine and one lifecycle implementation. Shared
  legality checks reject sibling-session use and closing a parent with an open
  child; cancelled backend runs reach the runtime's cancellation path.
- Check evidence, planner recording and task feedback use common mechanics.
  Context snapshots retain write-time capture, complete values and local rendering;
  physical storage stops dictating a shared host/worker pathname.
  The consumer configures agent-readable materialization roots, host object access
  and cache roots, and retention that lasts as long as the referenced run history.
- The emitter consumes a common bounded result check and retains its own replay,
  syntax and emission policies. The private named-constant defect is fixed or
  source-diagnosed, rather than emitting code known not to compile.
- A narrow hosted-run integration and graph-generation route are identified for
  public use, with no need for consumers to name internal host types. Correct the
  specimen's missing Planning graph and check registration in the production build.
  Route the actual console cancellation command through the consumer controller
  hook as well as the runtime, preserving acknowledgement versus completed cleanup.

Retain the existing paired/source-variation checks for results, context capture,
shadowing, authored mutation, feedback and cleanup. Add focused cases for actual
refactoring risks: Check's canonical record and preconditions, task-local feedback
selection, stale/incorrect lexical snapshot threading, resource legality and the
selected control behavior. Prove the storage seam with the
filesystem implementation and a second small in-test object store; a cloud backend
is not needed. A clean consumer-module build and live UI acceptance remain the
later publication/consumer milestones.

Stage 1 used source inspection, production/test dependency listing and the
already-recorded checks at the named revision. No new runtime, provider, browser, deployment or conformance test was
run for this document; none of the proposed API is implemented. No runtime defect
described as a source difference above is being presented as a fresh reproduction.

**Design check-in:** accept or revise these ownership boundaries and the direct
typed-operation approach before extraction. The companion API sketch makes the context/task entry call shape
concrete for its separate review; implementation must test those call sites. This does not require selecting Kubernetes hosting, broadening fan-out or
numeric results, or defining a general backend framework.

## Source anchors

All links below name the inspected revision, not a moving branch.

- [Shared Generate and local typed acceptance](https://github.com/tylergannon/gimbal/blob/6aee0a14704389e88b4d630318aa3794f022483c/session.go#L90), [private split response seam](https://github.com/tylergannon/gimbal/blob/6aee0a14704389e88b4d630318aa3794f022483c/internal/compiledscope/generate.go).
- [Scope begin/finish and ownership](https://github.com/tylergannon/gimbal/blob/6aee0a14704389e88b4d630318aa3794f022483c/scope.go#L170), [consumer leases and cleanup](https://github.com/tylergannon/gimbal/blob/6aee0a14704389e88b4d630318aa3794f022483c/internal/experiments/instrumented/activities.go#L104).
- [Local loop semantics](https://github.com/tylergannon/gimbal/blob/6aee0a14704389e88b4d630318aa3794f022483c/loop.go#L88), [compiled planner mechanics](https://github.com/tylergannon/gimbal/blob/6aee0a14704389e88b4d630318aa3794f022483c/compiled_loop.go), [specimen checks and feedback](https://github.com/tylergannon/gimbal/blob/6aee0a14704389e88b4d630318aa3794f022483c/internal/experiments/instrumented/planning_activities.go).
- [Local Check implementation](https://github.com/tylergannon/gimbal/blob/6aee0a14704389e88b4d630318aa3794f022483c/command.go#L132), [snapshot storage](https://github.com/tylergannon/gimbal/blob/6aee0a14704389e88b4d630318aa3794f022483c/internal/compiledscope/context.go), [runtime context binding](https://github.com/tylergannon/gimbal/blob/6aee0a14704389e88b4d630318aa3794f022483c/compiled_context.go).
- [Result admission](https://github.com/tylergannon/gimbal/blob/6aee0a14704389e88b4d630318aa3794f022483c/internal/temporalgen/result.go), [operation and loop emission](https://github.com/tylergannon/gimbal/blob/6aee0a14704389e88b4d630318aa3794f022483c/internal/temporalgen/generate.go#L556).
- [Hosted run entry](https://github.com/tylergannon/gimbal/blob/6aee0a14704389e88b4d630318aa3794f022483c/internal/host/host.go#L247), [public web instance](https://github.com/tylergannon/gimbal/blob/6aee0a14704389e88b4d630318aa3794f022483c/web/runtime.go#L24), [graph meaning](https://github.com/tylergannon/gimbal/blob/6aee0a14704389e88b4d630318aa3794f022483c/workflow/graph.go).
- [Previously collected behavioral evidence](compiler-operation-proof.md) and [review verification](../ephemeral/worklog/20260929-compiler-portability-review.md).
