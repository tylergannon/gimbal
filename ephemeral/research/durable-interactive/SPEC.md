# Durable Interactive execution

Status: completed authoring draft, reviewed against the nlspec standard.
The public behavioral choices below reflect the human's brief and answers.
Cells, budgets and derived contracts are ready for final review before this
document governs implementation. No implementation or live proof has been
performed.

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
reconstructed by replay under the same code; serializing Go stacks is not
required.

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
functions, even when an author considers them safe to repeat. Ordinary Go
control flow and deterministic local computation remain ordinary Go.

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
| J: durable records and inbox | Durable identity, committed batches, inbox acceptance and durable transition storage; no provider calls or scheduling policy | Storage consistency and recovery format | 913 | 1,200 |
| E: execution and replay | Reconstruct library operations, scopes, session bindings and concurrent branches; distinguish suspension from terminal outcomes | Workflow execution semantics | 3,469 | 4,500 |
| I: interaction protocol | Interpret chat/end, transcript, human message/finish input and turn delivery; use J for durable transitions and E for execution | Human/agent interaction contract | 102 | 800 |
| H: shared hosting | Own execution capacity and placement, accept controls independent of an activation, schedule runnable work and reconstruct runs | Host ownership, placement and dispatch | 4,288 | 4,500 |
| P: presentation | Project durable lifecycle and interaction state into run/chat views and route user actions through durable acceptance | Product presentation and observation | 4,532 | 5,000 |
| A: workflow admission | Accept only library-owned effects, expose Interactive in extraction/diagrams and replace Interview authoring support | Accepted workflow language and static representation | 5,881 | 6,500 |

J stores authoritative facts; I and E decide their meaning. H owns when and
where an activation runs, independently of E's workflow semantics. P observes
committed state and cannot create execution truth from a display event. A
checks authored source independently of runtime execution. These differing
reasons to change motivate each seam. I is separate from E because the
human/agent protocol can change without changing generic replay or Group.

```text
gimbal/
  runstore/                         J
  run.go, scope.go, group.go         E
  iterate.go, session.go, command.go E
  run_recording.go                  E
  event_persistence.go, artifact.go  E
  schema.go                         E (shared lifecycle declarations)
  interview.go -> interactive.go    I
  internal/
    interaction/                    I (only if useful; not a required package)
    host/, runner/, invocation/     H
    conversation/                   H (replace separate execution ownership)
    observation/                    P
    gimballint/, generate/           A
  workflow/                         A
  web/src/
    lib/conversation/               P
    lib/workspace/                  P (chat tab integration only)
    routes/                         P (conversation/interaction actions only)
```

Budgets count all physical lines, including comments and blanks, in
handwritten production source. Exclude tests, testdata and generated source
(generated suffix or a Code generated header); generated output must still
match its source. The measured ownership sets are:

- J: `runstore/**/*.go`.
- E: the ten named root files in the sketch. New root source for execution
  or replay belongs to E and counts toward its maximum.
- I: `interview.go` until removed, `interactive.go`, and any new handwritten
  interaction protocol source, including `internal/interaction/**` if created.
- H: Go source in `internal/host/`, `internal/runner/`,
  `internal/invocation/` and `internal/conversation/`.
- P: Go source in `internal/observation/`; Go/TypeScript/Svelte source in
  `web/src/lib/conversation/`; `ProjectWorkspace.svelte`,
  `workspace/conversations.ts`, and the `conversation.remote.*` and
  `interview.remote.*` route files plus their replacements.
- A: Go source in `internal/gimballint/`, `internal/generate/` and `workflow/`.

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

For initial entry, choose the simplest correct behavior. Existing immediate
agent turns are available as a starting point, but this spec does not require
either entry mode. Supporting both agent-first and human-first entry is future
direction; it is not a requirement to add mode machinery in this change.

## Internal contracts

The following records and interfaces name semantic boundaries, not exported
Go APIs, required packages, or a new workflow DSL. Existing library operations
remain visible at their workflow call sites.

### Durable identity and transitions: J

```text
RECORD InteractionAddress:
    run          : RunIdentity
    interaction  : OperationIdentity

RECORD InboxInput:
    address      : InteractionAddress
    identity     : InputIdentity
    sequence     : AcceptanceOrder
    kind         : MESSAGE | FINISH
    message      : String when kind is MESSAGE
    disposition  : pending | delivered | superseded | unresolved

RECORD RecordedOutcome:
    operation    : OperationIdentity
    result       : RecordedOutputOrError
    binding      : RecordedNativeBinding when applicable

INTERFACE DurableRecords:
    accept(input) -> DurableAcknowledgement | Conflict | Closed | Error
    commit(transition) -> DurableAcknowledgement | Error
    read(run) -> CommittedState
```

An interaction address survives activation loss and reconstruction. Identity
uses the existing run/node naming discipline, including parentage and authored
occurrence; replay cannot allocate replacement identities for recorded work.

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
backend is required.

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

The returned transcript contains durably accepted human messages and committed
assistant chat replies in their recorded order. Transport retries, replay,
and duplicate provider observations do not add duplicate transcript messages.
Control signals are not fabricated as human prose or assistant replies.

A finish signal is durable control, not an instruction that the model must
choose to obey. It prevents further ordinary turn dispatch for that
interaction and requests turn-scoped interruption if necessary. New messages
after the durable finish boundary are rejected as closed; earlier accepted
but undelivered inputs remain recorded with their terminal disposition.
Repeated finish requests are idempotent. Agent end also records a terminal
boundary. Inputs not delivered before a terminal boundary remain durably
recorded and visibly unprocessed, rather than being acknowledged as delivered.
In a continuing conversation workflow, pending input follows the durable
ingress into the next call occurrence without changing its identity or order.
Human finish closes that ingress and prevents additional ordinary turns.

An already committed chat remains visible if finish follows it. Once finish
has won the durable ordering, a late native result cannot create another wait
or restart the interaction. A normal agent end and human finish that race
produce one logical completion for that call, with the winning EndedBy
recorded. Do not rewrite a completed result to change its ending. A pending
human finish still reaches the next call if the conversation workflow
continues after an agent end.

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
    workflowAndCode   : SameCodeIdentity
    inputs            : ImmutableInvocationInputs
    projectAndWorkdir : SavedPlacement
    rolesAndRoutes    : SavedBindings
```

Activation evaluates the same workflow code using recorded library outcomes
for completed operations. Preserve scope parentage, stored values, session
binding including native identity, and completed results. Reconstruct through
ordinary deterministic control flow; do not serialize Go stacks. Saved graph
metadata alone does not reconstruct executable code.

Concurrent branches retain authored identities independent of scheduling.
Record any library-controlled nondeterministic choice needed to reproduce
control flow, such as a failure that cancelled siblings. Waiting is neither
an ordinary error nor a successful return from the logical branch. It cannot
trigger Group's error cancellation, finish a scope, or allow code after an
unfinished join to run.

A waiting branch permits runnable siblings to continue. When no branch can
progress and the unfinished branches have durable waits, the run can release
its activation and remain dormant. Resumption reconstructs enough execution
to continue, including pending joins. The pending call is a logical property
of recorded execution, not a requirement to retain its current Go stack.

Suspension does not run normal scope teardown or emit scope/run completion or
cancellation. Actual completion and explicit cancellation retain their normal
lifetime behavior. Replay must not repeat completed operations or already
performed terminal cleanup. This is not a SERVICE restoration contract; the
explicit SERVICE exclusion below takes precedence at that boundary.

Only same-code reconstruction is in scope. Persist enough code identity to
avoid silently resuming an incompatible workflow. If the matching workflow
code or recorded native operation cannot be recovered, expose an actionable
recoverable state instead of inventing a fresh run or blindly redispatching.
Hot upgrades, code migration and changed-workflow replay are not promised.

### Hosting and ingress: H

```text
INTERFACE ExecutionHost:
    admit(invocation) -> RunIdentity
    submit(address, input) -> DurableAcknowledgement | Error
    schedule(runIdentity)
    observe(runIdentity) -> CommittedState
```

The host executes runnable workflows from different projects using shared Go
execution capacity in their selected environment. Logical run lifetime is
independent of any activation. Replace separate conversation execution
ownership with ordinary workflow admission, operations, controls and records.
Chat naming and workspace metadata may remain product data.

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

P projects committed identity, replies, input disposition and execution state.
Waiting and dormancy remain visibly unfinished; successful completion,
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
| C2 | Cell-local: J | Accepted input survives restart; identical retry preserves identity; conflicting retry fails; equal timestamps cannot make durable order ambiguous |
| C3 | Cell-local: J | Record recovery preserves valid committed batches and enforces writer authority; interrupted acknowledgement does not require duplicate admission |
| C4 | Cell-local: E | Reconstruction uses saved run/scope/session identities, context and completed outputs without redispatching completed commands or turns; incompatible code or unconfirmed native work remains actionable and recoverable |
| C5 | Cell-local: E | A waiting branch leaves runnable siblings progressing; all-waiting work becomes dormant; neither path emits terminal teardown or alters SERVICE claims because of suspension |
| C6 | Cell-local: I | Polytype validates the specified chat/end alternatives; ordered transcripts and durable EndedBy distinguish agent/human completion, with no invented ending on error; conversation re-entry waits or uses pending input instead of spinning |
| C7 | Cell-local: I | Active input steers; a confirmed missed steer remains pending; uncertain delivery is not blindly repeated |
| C8 | Cell-local: I | Finish is durable and idempotent, interrupts the active turn when safe, completes normally, and prevents late results from reopening the interaction |
| C9 | Cell-local: H | Runnable runs from different projects share host capacity with independent invocation data and authoritative execution ownership |
| C10 | Cell-local: H | Pending durable input is discovered after restart or lost notification without requiring a second human message or a competing activation |
| C11 | Cell-local: P | Committed replies and waiting/runnable/terminal/unresolved state are rendered honestly; message/archive actions use durable addresses and acknowledgements |
| C12 | Cell-local: A | Prohibited nondeterminism and effects in authored bodies/helpers are rejected; Interactive replaces Interview in accepted API and workflow inspection |
| L1 | Integration: J, E, I, H, P and native adapter | In the real app, converse, observe chat/wait, restart the host, send again, and observe the same run, native session continuity, scoped context, prior results and nonduplicated transcript |
| L2 | Integration: J, E, I, H and native adapter | In an ordinary workflow, a useful runnable branch progresses while another waits, alongside another project's work on the shared host; resume the waiting branch and observe its join and subsequent workflow code execute with independent project/run state |
| L3 | Integration: J, E, I, H, P and native adapter | Accept messages during active execution, waiting and restart; exercise a commit/wake-up interruption and verify accepted input is retained and safely delivered or explicitly unresolved |
| L4 | Integration: J, E, I, H, P and native adapter | An agent end leaves the conversation available on its existing run without autonomous repeated turns; archive during an active turn then causes safe interruption, EndedBy human, normal Interactive return and actual successful workflow completion; unconfirmed cessation is shown honestly |

Cell-local tests may use contract-faithful stand-ins. Live claims use the real
application and native adapter, with the cheapest unversioned model alias for
the harness used (`luna`, `haiku` or `flash`); report which one actually ran.
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
- No stack serialization, changed-code replay, automatic code migration,
  alternate durable-workflow framework, or new database backend.
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
  Keep the easiest correct starting behavior. Conversation re-entry must
  respect the required input wait; no mode framework is necessary.
- If the preferred archive finish signal requires a materially separate
  mechanism beyond durable interaction ingress, return its scope/cost to the
  human, who permits deferral. Do not silently claim successful workflow
  completion by relabeling unfinished execution.

The [source map](SOURCE-MAP.md) records current implementation facts and cached
prior art. It informs construction and is not a second authority for intent.
The [author's review](REVIEW.md) records the six-pass review and the human's
resolution of the termination-provenance finding. An independent pass is
available before implementation. After approval, cross-seam implementation
uses slice specs derived from these contracts; implementers escalate underivable
decisions or budget breaches rather than amending this spec themselves.
