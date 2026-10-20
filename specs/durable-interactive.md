# Durable Interactive execution

Status: implementation contract incorporating human decisions and independent
Opus review adjudication. No implementation or live proof is claimed.

## Overview and goals

A workflow has a durable identity independent of its current execution. It can
remain logically unfinished at an explicit wait while releasing active
execution, then become runnable when input arrives. Conversation tabs are
ordinary workflows containing one interactive step, hosted alongside other
workflows in shared Go execution capacity.

Replace `gimbal.Interview` with `gimbal.Interactive`. The agent responds through
a Polytype tagged union with these exact alternatives:

```json
{"type":"chat","message":"…"}
```

```json
{"type":"end"}
```

`chat` finishes the agent turn, publishes its reply, and establishes a durable
wait as one durable transition. The workflow's call to `Interactive` remains
logically pending. `end` completes the interactive step and permits the
workflow to continue. Conversation workflows remain interactive until the
human archives them.

On completion, `Interactive` returns the ordered conversation transcript and
an error, preserving the useful return contract of `Interview`. The approved
public types are `InteractiveTranscript` and `InteractiveMessage`, with
ordered messages containing `role` and `message`. The transcript's `EndedBy`
distinguishes agent end from human finish.

Human archive requests successful completion. It signals the interaction to
finish through its durable inbox. Handling this finish signal makes
`Interactive` return normally, so subsequent workflow code runs and the
conversation workflow can complete successfully through its ordinary return
path. Archive does not merely mark an unfinished workflow successful.

The finish signal is proposed within this change because it can share the
durable interaction inbox. If implementing it requires a materially separate
mechanism, return that scope decision to the human, who permits deferring it.

If archive arrives during an active turn, request interruption of that turn.
This stops the turn, not the enclosing workflow or runnable sibling branches.
Once the turn has safely stopped, the finish signal makes `Interactive` return
normally. If interruption would leave unrecoverable or corrupt state, defer
it until safe. If cessation or the operation outcome is unconfirmed, preserve
the finish request and a recoverable unresolved state while reconciling;
uncertainty is not successful completion and does not justify blind replay.

Human messages enter a durable inbox addressed to the interaction regardless
of whether execution is active, suspended, or restarting. An incoming message
signals that the interaction can run again. Resume preserves run identity,
session binding, scoped context, and completed results. Execution may be
reconstructed by replay; the demonstrations here use the same code.
Serializing Go stacks is not required. Version changes across runs have no
policy or explicit enforcement in this change.

When a message arrives during an active agent turn, delivery steers that turn.
The message still enters the durable inbox before delivery; active execution
does not bypass durable acceptance.

Workflows from different projects may share an execution host within the same
execution environment. Each run retains its own identity, scoped context,
session bindings, completed outcomes, and placement. Sharing a host does not
merge project paths, run state, or independently selected task placements.

Suspension is distinct from completion and cancellation and does not perform
normal scope teardown. A waiting branch does not stop runnable siblings. A
run may become dormant when none of its branches can progress. Suspension
follows an explicit wait, never an inactivity threshold.

## Workflow authoring contract

Workflow functions use the Gimbal library for external effects and
nondeterministic observations. Direct file reads or writes, clocks, randomness,
and equivalent operations outside the library are forbidden in workflow
functions, even when an author considers them safe to repeat. Ordinary deterministic Go computation and control flow remain ordinary Go.
This applies to every accepted workflow, including maintained built-ins and
examples, and to their source-visible helpers. Raw goroutines, channels,
select and map iteration in authored workflow execution are rejected because
they introduce scheduling or iteration choices outside recorded library
operations. Deterministic slice/array iteration, map lookups and pure helper
calculations remain allowed. Opaque helpers cannot hide prohibited effects.
Authored defer and recover are rejected: suspension must not execute authored
completion work or permit authored recovery to continue beyond a pending wait.
Write cleanup on explicit completion/error paths using library operations.
Library internals retain their own suspension-aware defer/recovery handling.

Authored context deadlines/timeouts, sleeps/tickers, and direct cancellation
observations (including ctx.Err/Done) are also rejected. They introduce timing
or cancellation decisions outside recorded library outcomes. Use RunCommand
for explicit waits and observations. Cancellation still reaches library calls,
and their recorded outcomes can drive ordinary error handling. Existing
authored workflow/agent-turn deadlines without a library equivalent are removed
in this delivery; this includes validate-product's suite timeout option (formerly
defaulting to one hour). Normal caller-level Run deadlines/cancellation remain.
Research-eval's child-trial deadline remains expressible through recorded clock
observations and existing control commands. No new timeout API or authored
dormant-deadline semantics are introduced.

Use existing RunCommand operations for file reads/writes, stat, clock reads,
randomness and other external work when Check or Service is not the relevant
operation. RunCommand returns complete captured stdout and stderr, including
on replay; observation excerpts remain bounded. This replaces its old 64 KiB
caller-return truncation and permits complete file reads through the existing
API. No new file, clock or custom-activity API is introduced. Mutable inputs
are observed through recorded operations, never reread during replay.

Replay reconstructs that control flow using recorded library-operation
outcomes. Accepting arbitrary user I/O under an author-supplied replay-safety
promise is not the contract. A possible future `Activity` concept for custom
work is outside this change; no Activity API is introduced here.

## Proposed cells and seams

These are responsibilities and change boundaries, not new public packages or
wrapper APIs. The file sketch locates existing code and likely replacements;
the slice spec can settle file organization without changing these contracts.

| Cell | Responsibility and boundary | Reason to change | Current lines | Proposed maximum |
| --- | --- | --- | ---: | ---: |
| J: durable records and inbox | Durable identity, committed batches, inbox acceptance and durable transition storage; no provider calls or scheduling policy | Storage consistency and recovery format | 953 | 1,400 |
| E: execution and replay | Reconstruct library operations, scopes, session bindings and concurrent branches; distinguish suspension from terminal outcomes | Workflow execution semantics | 7,388 | 9,000 |
| I: interaction protocol | Interpret chat/end, transcript, human message/finish input and turn delivery; use J for durable transitions and E for execution | Human/agent interaction contract | 102 | 900 |
| H: shared hosting | Own execution capacity and placement, accept controls independent of an activation, schedule runnable work and reconstruct runs | Host ownership, placement and dispatch | 7,132 | 8,500 |
| P: presentation | Project durable lifecycle and interaction state into run/chat views and route user actions through durable acceptance | Product presentation and observation | 13,882 | 14,800 |
| A: workflow admission | Accept only library-owned effects, migrate maintained workflows, expose Interactive in extraction/diagrams and replace Interview support | Accepted workflow language and static representation | 11,609 | 13,200 |

J stores authoritative facts; I and E decide their meaning. H owns when and
where an activation runs, independently of E's workflow semantics. P observes
committed state and cannot create execution truth from a display event. A
checks authored source independently of runtime execution. These differing
reasons to change motivate each seam. I is separate from E because the
human/agent protocol can change without changing generic replay or Group.

Budgets count all physical lines, including comments and blanks, in
handwritten production Go/TypeScript/Svelte source. Exclude tests, testdata,
fixtures, mocks, stories and generated source. For Go and TypeScript, a
generated header is an actual `^// Code generated .* DO NOT EDIT\.$` comment
(before the Go package clause, or in the TypeScript header),
never a string emitted by a generator; `_gen.go` and `.gen.*` suffixes
also identify generated output. The skgo-generated source copies under
`internal/skgo/links/**` are also generated (owned with A's generated outputs,
not handwritten production); regenerate them rather than hand-editing them.
Generated output must match its source.
Expanded baselines count existing affected code, not just newly written code.
The measured ownership sets are:

- J: `runstore/**/*.go` and `internal/runstate/store.go` (shared store
  resolution and fenced restart-inbox acceptance).
- E: root `run.go`, `scope.go`, `group.go`, `iterate.go`, `session.go`,
  `command.go`, `run_recording.go`, `event_persistence.go`, `artifact.go`,
  `schema.go`, `events.go`, `harness.go`, `env.go`, `loop.go`, `service.go`,
  `remote_execution.go`, `supervise.go`, `supervise_jev.go`, `doc.go`; Go source
  in `internal/{runstate,agentexecutor,executor,commandexecutor,effects}/`,
  excluding `internal/runstate/store.go`, which belongs to J.
  New root execution/replay source belongs to E.
- I: `interview.go` until removed, `interactive.go`, and new handwritten
  interaction protocol source including `internal/interaction/**` if useful.
- H: Go source in `internal/{host,runner,invocation,conversation,live}/`,
  `backend/backend.go`, `registry.go`, `cmd/gimbal/main.go`,
  `cmd/gimbal/daemon.go`, Go source in `internal/taskdaemon/`,
  `backend/messaging.go` and `cmd/gimbal/messaging.go` (329 existing lines
  assigned for the already required remote messaging/steering continuation).
- P: Go source in `internal/observation/` and `web/runview/`;
  Go/TypeScript/Svelte in `web/src/lib/{conversation,run,observation}/`;
  `web/src/lib/workflow/types.ts`; workspace `ProjectWorkspace.svelte`,
  `ProjectHome.svelte`, `ProjectRuns.svelte`, `projectHome.ts`,
  `conversations.ts`; route `conversation.remote.*`, `interview.remote.*`,
  `control.remote.*`, `projects/**/+page.server.ts`, and handwritten Go/Svelte
  under `projects/[project]/conversations/**`, plus replacements.
- A: Go source in `internal/{gimballint,generate,devgenerate,builtin,workflows}/`,
  `workflow/`, `cmd/examples/`, and `cmd/gimbal/audit_index.go` (the existing
  existing validated completion/freshness reads exposed as minimal CLI modes
  for recorded workflow use).

Additional or relocated production source implementing a cell's responsibility
counts toward that cell; moving files cannot evade a budget. A slice names
the exact additional ownership paths so the count is mechanical. Existing
adapters and SERVICE implementations retain their own contracts; necessary
changes outside these sets must be assigned and budgeted before implementation.
On a budget breach, stop and report a spec defect rather than inventing a new
cell or expanding the allowance. No test-count target or proof program is
introduced.

## Public shape

The replacement keeps the existing call arguments. The human approved the
transcript and message types, ordered role/message representation, and EndedBy:

```go
func Interactive(ctx context.Context, name string, session *Session, purpose string) (InteractiveTranscript, error)

type InteractiveTranscript struct {
    Messages []InteractiveMessage `json:"messages"`
    EndedBy  string               `json:"ended_by"`
}

type InteractiveMessage struct {
    Role    string `json:"role"`
    Message string `json:"message"`
}
```

Roles identify human and assistant messages as `user` and `assistant`.
The transcript remains a generated Output usable with SetJSON, like the
replaced transcript. It is the ordered interaction conversation; provider tool
events remain in their existing observation surface. The agent's chat/end decision
is a generated Polytype tagged union and needs no new exported decision type.
Replacement removes Interview and its Q/A-only types rather than retaining a
compatibility alias. On error, return the committed transcript accumulated so
far with the error. A finish signal and an agent end both return normally;
EndedBy is `"agent"` for an agent end and `"human"` for a human finish signal.
It is runtime-produced termination provenance, not another agent-response
field. On an error that prevents normal completion, EndedBy is empty; it must
not invent a successful ending. The completed result, including EndedBy, is
durable and returned unchanged on replay.

The conversation workflow uses one authored Interactive call site in an
ordinary loop. It returns successfully when EndedBy is `"human"` and otherwise
continues interaction on the same run and session:

```text
LOOP:
    transcript, error = Interactive(context, name, session, purpose)
    IF error exists:
        RETURN error
    IF transcript.EndedBy == "human":
        RETURN success
```

After agent end, re-entry waits durably for further human input rather than
starting another autonomous turn. Already accepted undelivered input or a
pending finish signal can satisfy that wait immediately. Keep their identities
and dispositions across the boundary; do not redeliver confirmed input or
require an additional message. The conversation's durable ingress remains
usable while its workflow moves between these call occurrences. A human-first
default is sufficient to provide this behavior without adding startup modes.
The transcript of each call covers that call's interaction; the saved
conversation view projects the ordered records across all occurrences.

Every Interactive occurrence waits for human input before its first agent
turn in this delivery. Pending accepted input can satisfy that wait immediately.
The maintained interview example therefore starts with a human opening message.
This is the simplest delegated initial choice; supporting agent-first entry as
well is future direction, with no entry-mode machinery required now.

## Internal contracts

The following records and interfaces name semantic boundaries, not exported
Go APIs, required packages, or a new workflow DSL. Existing library operations
remain visible at their workflow call sites.

### Durable identity and transitions: J

```text
RECORD InteractionAddress:
    run          : RunIdentity
    scope        : LogicalScopeIdentity
    name         : AuthoredConstantName

RECORD InboxInput:
    address      : InteractionAddress
    identity     : InputIdentity
    sequence     : AcceptanceOrder
    kind         : MESSAGE | FINISH | STOP
    message      : String when kind is MESSAGE
    disposition  : pending | delivered | unresolved | unprocessed
    occurrence   : OperationIdentity when claimed for delivery

RECORD RecordedOutcome:
    operation    : OperationIdentity
    result       : RecordedOutputOrError
    binding      : RecordedNativeBinding when applicable

INTERFACE DurableRecords:
    accept(input) -> DurableAcknowledgement | Conflict | Closed | Error
    commit(transition) -> DurableAcknowledgement | Error
    read(run) -> CommittedState
```

An interaction address survives activation loss, reconstruction and repeated
calls at the same authored name in the same logical scope. It deliberately
omits the call ordinal. Each call occurrence still has its own recorded
OperationIdentity/ordinal; replay cannot allocate replacement identities.
Repeated occurrences share this durable ingress and its session binding.
Different interactions in a scope use different constant names. Two concurrent
consumers of the same address are rejected rather than racing for its inputs.

Acceptance does not require an active occurrence. Delivery associates an input
with the occurrence/native turn consuming it in durable order. Confirmed
consumed input never moves to another occurrence. Pending input remains at the
stable ingress across agent end and between calls; unresolved delivery must be
reconciled before it can be consumed again. Agent end completes an occurrence,
not the ingress. When the owning scope or run actually terminates, close the
ingress and mark remaining undelivered input visibly unprocessed, so a one-shot
workflow cannot leave an indefinitely pending inbox after completion.

Acceptance is durable before acknowledgement. A retry with the same input
identity and content returns the previous acceptance; conflicting reuse is an
error. Assign an unambiguous durable acceptance order, rather than relying on
wall-clock timestamps. The inbox is accessible independently of a live Go
call, waiter, provider turn or browser connection.

Committing a chat reply and its next wait is indivisible. Recovery exposes
both or neither, with one logical reply identity. A committed reply is not
published again as new output during replay. Streaming provider diagnostics
may still be visible before this commit; they are not the authoritative chat
publication or evidence that the durable transition succeeded.

Native turn results may be recorded before the chat transition. A crash in
that gap reconstructs from the recorded result rather than running the agent
again. A result that was never recorded remains subject to the adapter's
existing uncertainty/reconciliation contract, not a claim of exactly-once
external effects.

The existing store's exclusive writer and atomic batch mechanisms may satisfy
these contracts. Inbox admission and workflow commits must share an ownership
arrangement that permits acceptance while an activation is running or absent.
No specific database, DBOS dependency, distributed lease system or new storage
backend is required. Existing accepted-intent evidence remains authoritative if
native result retention later removes a request/result: absence then cannot
prove the operation never ran. A replayed intent without an outcome or retained
native evidence remains unresolved rather than being submitted as fresh work.
Completed journal outcomes already bypass native dispatch.

### Agent and human protocol: I

```text
INTERFACE Interaction:
    accept(address, input) -> DurableAcknowledgement | Error
    advance(recordedState, observedTurnOutcome) -> DurableTransition
    result(address) -> OrderedTranscriptAndOutcome
```

These are internal descriptions of the public Interactive call and the host's
message/control ingress. An accepted message received during an active turn
is delivered through steering. If the adapter confirms that it did not land,
retain it for a subsequent turn while the interaction remains unfinished.
If delivery is uncertain, preserve that uncertainty and reconcile before
redelivery. Neither a missing acknowledgement
nor a restarted activation establishes that the message was not delivered.

Pending inputs are supplied in durable acceptance order. Whether the next
native turn receives several pending inputs together is internal, provided
accepted content and ordering are preserved. A committed chat always creates
its logical wait, including when pending input makes that wait immediately
runnable again. No second human message is needed to repair a lost wake-up.

Each returned transcript contains only user messages confirmed delivered to
that occurrence's native turns, plus assistant replies committed by that
occurrence. Pending, unresolved or unprocessed inputs are excluded; their
honest disposition remains visible in the conversation view. Input accepted
during occurrence N but delivered in N+1 belongs only to N+1's transcript.
Within a turn, user messages retain acceptance order; replies follow the turn's
delivered messages, and turns retain durable execution order. Transport retries,
replay and duplicate observations cannot duplicate transcript entries. Control
signals are never fabricated as prose. The conversation view may additionally
show accepted but unconsumed input with its actual disposition.

A finish signal is durable control, not an instruction that the model must
choose to obey. It prevents further ordinary turn dispatch for that
interaction and requests turn-scoped interruption if necessary. New messages
after the durable finish boundary are rejected as closed; earlier accepted
but undelivered inputs remain recorded with their terminal disposition.
Repeated finish requests are idempotent. Finish closes ingress to new ordinary
messages and remains pending for the current or next occurrence until handled.
Earlier accepted but undelivered messages become visibly unprocessed at human
completion; they are never acknowledged as delivered. Agent end only completes
the current occurrence. Pending input remains eligible for the next occurrence
without changing identity/order; if no later call occurs, owning-scope/run
termination closes and resolves its disposition as described by J.

An already committed chat remains visible if finish follows it. Once finish
has won the durable ordering, a late native result cannot create another wait
or restart the interaction. A normal agent end and human finish that race
produce one logical completion for that call, with the winning EndedBy
recorded. Do not rewrite a completed result to change its ending. A pending
human finish still reaches the next call if the conversation workflow
continues after an agent end.

STOP is the existing operator interrupt control, distinct from FINISH. It
requests turn-scoped interruption and, after confirmed cessation, records a
wait and keeps Interactive logically pending. It does not fabricate a partial
assistant chat reply, return an Interactive error merely because Stop was
requested, set EndedBy, close ingress, end the workflow or cancel siblings.
Confirmed delivered input remains in that occurrence's transcript; unresolved
cessation/delivery stays honestly unresolved. The same control applies in
ordinary run views and conversation tabs. Duplicate STOP requests are
idempotent; a settled/no-active-turn stop does not create a new agent turn.

Use the adapter's supported interruption and reconciliation contract. Do not
introduce a universal detector for arbitrary tool-side corruption or a rollback
system. Known unsafe interruption is deferred; unconfirmed cessation stays
recoverable and cannot be reported as successful completion. This rule does
not cancel runnable siblings or reinterpret an uncertain native action as
safe to repeat.

### Execution, scopes and reconstruction: E

```text
INTERFACE Execution:
    activate(savedInvocation, committedRun) -> Progress
    progress(run) -> Active | Runnable | Dormant | Terminal | NeedsReconciliation

RECORD SavedInvocation:
    runIdentity       : RunIdentity
    workflow          : RegisteredWorkflowIdentity
    inputs            : ImmutableInvocationInputs
    projectAndWorkdir : SavedPlacement
    rolesAndRoutes    : SavedBindings
```

Activation evaluates the same workflow code using recorded library outcomes
for completed operations. Preserve scope parentage, stored values, session
binding including native identity, and completed results. Reconstruct through
ordinary deterministic control flow; do not serialize Go stacks. Saved graph
metadata alone does not reconstruct executable code.

Native continuity uses the existing workflow executor path: recorded node
identity determines executor session/turn IDs; session checkpoints and per-turn
results permit reattachment to accepted work, including completed and active
turns. Reuse that path in internal/agentexecutor and internal/executor, with
its existing SaveSession/LoadSession behavior in Claude, Codex, Pi and Agy.
Do not route ordinary workflows through the old conversation-manager-only
ResumeSession mechanism or add a public HarnessAdapter method. A harness
without checkpoint/reattach capability retains saved identity/input and shows
unresolved recovery when continuation cannot be established; never substitute
a fresh native session. This is a correctness capability, not a resource policy.

Run keeps its existing signature and blocks until actual logical completion or
settled cancellation/error. Dormancy does not return success, a suspension
error or a new exported sentinel. The authored body/branch stacks unwind;
library coordination may await input without retaining those authored stacks.
Calling Run with the same existing run ID reconstructs that run from its
recorded work using the supplied body/bindings. Hosted admission persists the
registered workflow and inputs for startup reconstruction. Direct callers
supply them again on re-entry. A direct Run without installed ingress remains
pending at an Interactive wait until ingress is provided or its caller cancels;
it does not invent human input or silently finish. Existing hosted/example
entry points supply the control ingress.

The caller's original absolute run deadline is recorded at admission and keeps
running during dormancy; reconstruction does not reset it. Caller cancellation
remains an explicit request to stop the logical run. Each accepted native or
command operation reuses its recorded deadline/request on reattachment, so a
new activation cannot create conflicting accepted inputs by recomputing time.
Expired deadlines and cancellation use normal cancellation/reconciliation,
without blind native redispatch. These library-owned caller semantics do not
permit authored context timing observations. Update Run's godoc accordingly.

Concurrent branches retain authored identities independent of scheduling.
Record any library-controlled nondeterministic choice needed to reproduce
control flow, such as a failure that cancelled siblings. Waiting is neither
an ordinary error nor a successful return from the logical branch. It cannot
trigger Group's error cancellation, finish a scope, or allow code after an
unfinished join to run.

A waiting branch permits runnable siblings to continue. When no branch can
progress and the unfinished branches have durable waits, the run can release
its activation and remain dormant. Resumption reconstructs enough execution
to continue, including pending joins. Dormancy releases the activation's Go execution by unwinding it; parked
workflow stacks are not the suspension mechanism. Library scopes and Group
recognize suspension before generic panic/error handling, preserve logical
scope state, and avoid terminal cleanup or sibling cancellation. Admission
rejects authored defer/recover so user code cannot intercept this unwind or
run completion effects during it. The pending call is a property of recorded
execution; reconstruction uses ordinary Go code and recorded outcomes.

Suspension does not run normal scope teardown or emit scope/run completion or
cancellation. Actual completion and explicit cancellation retain their normal
lifetime behavior. Replay must not repeat completed operations or already
performed terminal cleanup. This is not a SERVICE restoration contract; the
explicit SERVICE exclusion below takes precedence at that boundary.

Version changes across runs have explicitly no policy and no explicit
policy enforcement. Do not add executable/plugin digest gates, version-mismatch
blocking, saved-build retention, per-version execution hosts, upgrade migration
machinery, or tests purporting to establish a cross-version guarantee or
non-guarantee. This is not a deferred policy-design task. The reconstruction
claims use unchanged code and do not prescribe behavior at a version boundary.
Existing record identity/conflict validation remains ordinary storage integrity,
not an added version policy. Unconfirmed native work remains actionable and
recoverable rather than being blindly redispatched.

### Hosting and ingress: H

```text
INTERFACE ExecutionHost:
    admit(invocation) -> RunIdentity
    submit(address, input) -> DurableAcknowledgement | Error
    schedule(runIdentity)
    observe(runIdentity) -> CommittedState
    cancel(runOrScope, cause) -> Acknowledgement | Error
    reconcile(runIdentity) -> CurrentRecoveryState
```

The host executes runnable workflows from different projects using shared Go
execution capacity in their selected environment. Logical run lifetime is
independent of any activation. Replace separate conversation execution
ownership with ordinary workflow admission, operations, controls and records.
A shared-host activation boundary contains an ordinary authored panic as that
run's failed outcome; another project's runs remain controllable and runnable.
Run/Group retain panic reporting, while the host catches the escaped panic at
the per-run boundary. Host loss/restart alone is not human cancellation and must not invent logical
completion or cancellation facts. Host restart releases its Go activations; accepted native
work remains subject to the existing executor checkpoint/reattach contract.
No code/plugin update policy is established here. SERVICE lifecycle and
restoration across host restart remain deferred, without a new missing-service
check, restart action or restoration guarantee.

Chat naming and workspace metadata may remain product data. This applies to
local and non-local workflow placement in the selected execution environment.
Remote ingress uses existing ControlWorkflow transport with interaction/input
identity in its control payload, routed to that environment's durable owner;
no new root API or backend interface method is required. Implement local
durability first, then remote durability in the same delivery. The remote
workflow/task daemon persists its run/session indexes and accepted identity
bindings, reconstructs its durable input owner on startup, and routes input
without a live-activation prerequisite. Steering through the existing messaging
transport is included. Reuse executor checkpoint/reattach for accepted native
work; a daemon restart cannot turn missing in-memory maps into a fresh run or
native session. SERVICE restoration/rechecks remain excluded.

Ingress must not require an in-memory waiter or a run classified as actively
executing. During restart, requests are either durably accepted or receive a
retryable failure; a transport outage is not a successful acceptance. Startup
and wake-up handling discover pending durable input and runnable work, so
notification loss cannot strand accepted work.

At most one authoritative activation may dispatch new work for a run.
Retain writer exclusion or equivalent ownership; do not start a competing
activation because a socket disappeared. A stale activation cannot commit
new replies or consume input after authority changes. These requirements do
not make arbitrary provider effects exactly once.

### Presentation and workflow admission: P and A

Dormant and unresolved runs retain the existing explicit cancellation control.
Cancel the logical run/scope, not merely its current activation; activate only
the library cleanup/reconciliation needed to settle that cancellation, without
replaying completed effect bodies. Actual cancellation uses existing terminal
cleanup behavior. Suspension alone never runs that cleanup. If native cessation
or cleanup is unconfirmed, retain its owned recoverable state and report it;
there is no invented successful ending. Reconciliation can be retried by the
host/control path by re-reading accepted executor work, without new native
turn dispatch. Expose retry-recovery and cancellation with the actual cause and
native/run identity. Uncertainty may remain unresolved when the external state
cannot be confirmed; no forced abandon outcome or eventual-settlement promise
is introduced. SERVICE restoration/rechecks remain excluded.

Preserve these existing conversation product behaviors while replacing their
execution owner; this does not authorize new high-level workflow APIs:

| Existing feature | Expression in this delivery |
| --- | --- |
| Stop turn | Durable STOP above, returning to wait after confirmed cessation |
| Model/effort change | Preserve existing settled-idle local Codex change; record configuration for subsequent turns on the existing thread through private workflow/executor controls; completed outcomes retain their original binding |
| File discussion | Preserve CacheFile before admission and bind its cached absolute path/context durably; an opening message uses the normal human-first inbox |
| Worktree, environment selection and run association | Keep product metadata/placement preparation and association; workflow execution uses that saved placement, including the remote stage |
| Rename and setup queries | Keep as product data/queries |
| Explicit cleanup | Retain explicit cleanup ownership and truthful outcome; suspension never invokes it |

P projects committed identity, replies, input disposition and execution state.
An ordinary run view supplies message, Stop and Finish controls for Interactive;
Finish is not limited to the conversation-tab Archive action. Waiting and dormancy remain visibly unfinished; successful completion,
cancellation and unresolved recovery are distinguishable. Human messages and
archive use durable ingress. Browser disconnects do not cancel workflow work;
observation gaps do not change execution truth. Archive completes through the
workflow's return path; merely closing a visible tab is not proof of archive.

A exposes Interactive in workflow extraction, generated schemas, graph
inspection and the maintained example that replaces Interview. Accepted
workflow bodies and their source-visible helpers cannot perform external
effects or nondeterministic observations outside the library. A helper is not
an escape hatch. Diagnostics reject prohibited authored code; the contract
does not require a general Go sandbox or invent an Activity mechanism.
Admission follows source-visible helpers across repository packages and uses
an explicit narrow allowlist for known deterministic opaque functions. Unknown
opaque calls are rejected, not trusted by package membership. Formatting/error
construction, lexical path operations, scalar conversions, JSON encoding and
pure collection/string calculations are eligible; ambient I/O, process cwd
reads (including filepath.Abs), callbacks into unknown effectful code and direct
context cancellation/deadline operations are not. Maintain the precise admitted
function table beside the analyzer and admit only its known deterministic forms.
Deterministic map lookup is allowed; raw map range is rejected. The known
slices.Sorted(maps.Keys(m)) composition and standard JSON map-key ordering are
admitted deterministic forms. This is a static authoring boundary, not a
whole-Go sandbox. Revise public godoc for Steer, Killed, Group and package
lifetime guidance to stop teaching newly forbidden authored constructs;
steering/control still comes through library-owned ingress/concurrency.
The gate is generation: gimbalgen runs admission over each selected entry and
its reachable helpers and fails before emitting/refreshing registration/graph
output on a violation. Built-in and project workflow generation use this same
gate. Registration/hosting consume generated definitions under the existing
graph contract; they do not perform source analysis of arbitrary compiled Go
plugins. No general sandbox or cryptographic provenance scheme is introduced.
Keep diagnostics through lint/vet too; C12 demonstrates generation refusal,
not merely an advisory diagnostic. Maintained hosted examples persist their
registered invocation/accepted ID rather than minting an unrelated run on
restart through an unregistered body wrapper.

Migrate all maintained built-ins/examples to these rules using the existing
library operations, including complete RunCommand outputs, and admit them
through the same checks as project workflows.

The public surface matches the approved replacement. Generated files are
regenerated from source. Existing Interview call sites and execution paths are
replaced, not retained through aliases or a parallel conversation runtime.

## Durable chat transition

Only the ordering that spans I, J, E and H is prescribed:

```text
WHEN an authoritative turn has a recorded valid CHAT result:
    COMMIT AS ONE TRANSITION:
        publish the logical assistant reply once
        establish the interaction's next durable wait
    IF durable pending input exists:
        make the interaction runnable
    ELSE:
        permit its branch to suspend

WHEN input is durably accepted:
    retain its stable identity and acceptance order
    IF it is FINISH:
        request normal interaction completion through safe turn interruption
    ELSE IF a turn is active:
        steer, retaining delivery disposition
    ELSE:
        make the interaction runnable

WHEN reconstructing completed library work:
    return its recorded outcome under the same operation identity
    do not dispatch it again
```

The scheduler must derive runnable work from durable state as well as transient
notifications. If finish was ordered first, the chat publication rule does
not authorize a late turn result to reopen the interaction.

## Definition of done

These are required future demonstrations, not reports of work already done.
Tests live beside the code they check. An agent observes the real application
and reports what happened in chat or the PR; no proof programs, run dumps,
screenshots or execution logs are committed as evidence.

| Claim | Classification and routing | Required observation |
| --- | --- | --- |
| C1 | Cell-local: J | Crash/failure around commit exposes reply and wait together or neither; replay never creates a second logical reply |
| C2 | Cell-local: J | Accepted input survives restart; identical retry preserves identity; conflicting retry fails; equal timestamps cannot make durable order ambiguous; stable ingress preserves pending input between occurrences and closes unconsumed input at owner termination |
| C3 | Cell-local: J | Record recovery preserves valid committed batches and enforces writer authority; interrupted acknowledgement does not require duplicate admission |
| C4 | Cell-local: E | Reconstruction uses saved run/scope/session identities, context and completed outputs without redispatching completed commands or turns; unconfirmed native work remains actionable and recoverable; RunCommand returns complete stdout/stderr above 64 KiB on original execution and replay while display excerpts stay bounded |
| C5 | Cell-local: E | A waiting branch leaves runnable siblings progressing; all-waiting work becomes dormant; Go execution unwinds without authored completion effects or generic panic handling; neither path emits terminal teardown or alters SERVICE claims because of suspension |
| C6 | Cell-local: I | Polytype validates the specified chat/end alternatives; per-occurrence transcripts include only confirmed delivered input and committed replies in the specified order; durable EndedBy distinguishes agent/human completion, with no invented ending on error; conversation re-entry waits or uses pending input instead of spinning |
| C7 | Cell-local: I | Active input steers; a confirmed missed steer remains pending; uncertain delivery is not blindly repeated; operator Stop safely returns to wait without completing the interaction |
| C8 | Cell-local: I | Finish is durable and idempotent, interrupts the active turn when safe, completes normally, and prevents late results from reopening the interaction |
| C9 | Cell-local: H | Runnable runs from different projects share host capacity with independent invocation data and authoritative execution ownership; a panic in one run leaves other runs and host controls working |
| C10 | Cell-local: H | Pending durable input is discovered after restart or lost notification without requiring a second human message or a competing activation; dormant cancellation and retry-reconciliation preserve ownership and honest unresolved outcomes |
| C11 | Cell-local: P | Committed replies and waiting/runnable/terminal/unresolved state are rendered honestly; message/archive/Stop/Finish actions use durable addresses and acknowledgements; listed existing conversation product behaviors remain available |
| C12 | Cell-local: A | Prohibited nondeterminism, effects, authored defer/recover and unrecorded timing/cancellation observations in bodies/helpers are rejected; generation fails on such source; Interactive replaces Interview in accepted API and workflow inspection; maintained built-ins/examples pass the same admission checks after migration |
| L1 | Integration: J, E, I, H, P and native adapter | In the real app, converse, observe chat/wait, restart the host, send again, and observe the same run, native session continuity, scoped context, prior results and nonduplicated transcript |
| L2 | Integration: J, E, I, H and native adapter | In an ordinary workflow, a useful runnable branch progresses while another waits, alongside another project's work on the shared host; resume the waiting branch and observe its join and subsequent workflow code execute with independent project/run state |
| L3 | Integration: J, E, I, H, P and native adapter | Accept messages during active execution, waiting and restart; exercise a commit/wake-up interruption and verify accepted input is retained and safely delivered or explicitly unresolved |
| L4 | Integration: J, E, I, H, P and native adapter | An agent end leaves the conversation available on its existing run without autonomous repeated turns; archive during an active turn then causes safe interruption, EndedBy human, normal Interactive return and actual successful workflow completion; unconfirmed cessation is shown honestly |
| L5 | Integration: J, E, I, H, P, remote workflow/task daemon and native adapter | After local proof, run through a real separate daemon/transport, reach durable wait, restart that daemon, submit input and observe the same run/native session continue without losing accepted input; exercise active steering through the remote/messaging route and normal finish |

Cell-local tests may use contract-faithful stand-ins. Live claims use the real
application and native adapter, with the cheapest unversioned alias of the exercised harness (`luna`,
`haiku` or `flash`); report which one actually ran. Native continuity uses the
existing workflow executor checkpoint/reattach path, not a new adapter lifecycle.
The proof must show behavior, not merely a green gate. It does not assert any
policy about native agent resource retention or release.

Mechanical review reports public-surface differences, cross-cell calls added
or removed, cell ownership and line counts, and claim coverage for every seam.
It must not use style objections or unrequested guarantees as acceptance gates.

## Exclusions and intentionally absent requirements

- Native agent resource handling has explicitly no policy in this change.
  Choose the easiest correct default. Do not establish detach, retain,
  eviction, process-count or resource-release rules, and do not add tests or
  proof claims purporting to enforce such rules. Correct session continuity
  and workflow resumption are required; how native resources support them is
  internal. This is not a deferred policy that an implementer must go design.
- No Activity API, arbitrary user-I/O escape hatch or author-promised
  replay safety for prohibited operations.
- No stack serialization, alternate durable-workflow framework or new
  database backend. Version changes across runs carry no policy or enforcement
  as stated above; no version-handling work or tests are required.
- No arbitrary exactly-once agent/tool effects, universal interruption-safety
  detector, transactional rollback of provider tools, or blind redispatch of
  unresolved native work.
- No timer/inactivity suspension, service-restoration guarantees or new service
  lifecycle rules. Shared Go hosting does not set native agent process policy.
- No compatibility shims for Interview, permanent first-turn mode machinery,
  unrelated UI redesign, scheduling quotas or performance/SLA framework.

## Explicit exclusion: SERVICE lifecycle

Suspension must not close, release, or otherwise alter any service or SERVICE
claim. Existing service behavior and claims are left alone. Rechecking or
restarting SERVICE claims on resume and service-restoration guarantees are
deferred. This spec must not acquire those requirements indirectly through
cleanup, replay, recovery, or validation claims.

## Deferred decisions and handoff

- SERVICE claim rechecks/restarts and restoration across resume are explicitly
  deferred. If a slice requires those guarantees, stop and escalate instead
  of adding them as an implicit dependency of this change.
- Supporting both agent-first and human-first entry is future direction.
  This delivery uses human-first entry for every occurrence; no mode
  framework is necessary.
- If the preferred archive finish signal requires a materially separate
  mechanism beyond durable interaction ingress, return its scope/cost to the
  human, who permits deferral. Do not silently claim successful workflow
  completion by relabeling unfinished execution.

The [source map](../ephemeral/research/durable-interactive/SOURCE-MAP.md) records current implementation facts and cached
prior art. It informs construction and is not a second authority for intent.
The [author's review](../ephemeral/research/durable-interactive/REVIEW.md) records the six-pass review and the human's
resolution of the termination-provenance finding. An independent pass is
available before implementation. After approval, cross-seam implementation
uses slice specs derived from these contracts; implementers escalate underivable
decisions or budget breaches rather than amending this spec themselves.
