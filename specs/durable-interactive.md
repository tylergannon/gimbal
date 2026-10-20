# Resumable Interactive workflows

Status: corrected contract, 2026-10-19. This replaces a6adf69d's erroneous
removal of restartability. It describes intended behavior, not verified code.
The [delivery plan](../ephemeral/research/durable-interactive/DELIVERY-PLAN.md)
is the implementation handoff. Historical maps/reviews add no requirements.

## Outcome and API

Workflows wait for human input and continue after process restart under the
same run identity, preserving accepted inputs, completed results, scoped
context and native session binding. Conversations are ordinary workflows
hosted beside other workflows in shared Go execution capacity.

Replace Interview and its Q/A types with:

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

Use a generated Polytype tagged union in the documented agent response:

```json
{"decision":{"type":"chat","message":"…"}}
{"decision":{"type":"end"}}
```

Chat commits the assistant reply and next wait atomically, ends the native
turn and leaves Interactive pending. End returns normally with EndedBy agent.
Human Finish returns normally with EndedBy human. On error return the committed
transcript and error, without a successful ending. Messages use user/assistant
roles; controls are not prose messages.

Each occurrence starts human-first, the chosen simplest initial behavior.
The conversation uses the same session in an ordinary loop around Interactive,
returning on human Finish. Agent end therefore does not archive the conversation
or start an autonomous model loop.

## Waiting and resuming

Waiting parks the current goroutine; it retains its stack and reachable objects.
Run remains pending until actual completion, error or cancellation. Scopes stay
open, Group siblings continue and Wait joins normally. Waiting does no teardown,
session Close or SERVICE claim changes. Actual completion/cancellation retains
ordinary cleanup behavior. One live registration lasts until Run exits.

After process restart, reopening the journal and re-entering the same workflow
body with the same run ID reconstructs execution. Stable scope/name/ordinal
slots identify recorded library operations. Completed calls return saved results
without redispatch; deterministic Go control flow and context operations rebuild
the scope state. Completed branches do not repeat their effects or cleanup.
The unfinished Interactive restores its transcript, input position and pending
wait. Session/executor owns native binding and operation-sequence restoration.

Hosted invocation data preserves inputs, workdir, role selections and placement
so the host can re-enter unfinished runs at startup. Direct Run callers supply
their body/bindings again. Re-entry is for process restart; ordinary input never
unwinds or reconstructs a workflow. Shared hosting preserves project/run isolation
and does not require one workflow process per conversation. A failed run does
not terminate unrelated hosted work; browser disconnection does not cancel it.

An operation interrupted by restart uses its recorded identity and the existing
executor's persisted request/result and native-session support. Reattach to/read
the existing operation. If its outcome is unavailable, return an actionable error
and retain its facts. Missing acknowledgement does not authorize a new operation.
No additional automatic repair protocol or guarantee for every failure window.

Demonstrate restart with unchanged code. Version changes across runs have no
policy or explicit enforcement; this scenario introduces neither support nor
non-support policy. SERVICE restoration and rechecks are deferred.

## Input and controls

The stable ingress is run/scope/constant name. Repeated calls share that ingress
but retain distinct occurrence/result identities. One occurrence consumes each
input; simultaneous consumers at the same address are rejected.

One interaction owner admits messages/controls in journal order, committing
before acknowledgement. Same ID/content reuses acceptance; conflicting reuse
fails. Acknowledged inputs survive restart. Restore the owner before accepting
requests; while unavailable return a retryable error without acknowledgement,
retaining the sender's text/ID for retry. A brief startup retry window is accepted
instead of introducing a second absent-owner writer.

Session/executor serializes message delivery against turn completion. An active
turn receives steering; a message arriving after completion is queued for the
next turn dispatched by Interactive. Assign delivery before native submission;
an in-flight message cannot also be dispatched as a new prompt. Unknown delivery
is reported, not automatically resent. No hidden turn starts outside the workflow.
Durable queue checks and notifications cannot lose an accepted input at sleep.

Native session history remains native. Send new input and relevant instructions/
context rather than repeating the transcript. Interactive owns the display and
returned transcript. Each occurrence returns confirmed delivered user messages
and committed replies; pending/unknown/unprocessed input retains its actual status.
Pending input carries across agent end into the next occurrence.

Stop targets the current turn, requests interruption and leaves the interaction
open. Confirmed requested interruption is normal control handling. If the native
turn already succeeded, preserve its reply/end even when Stop raced completion.

Finish durably closes ingress before acknowledgement and prevents another turn.
It requests interruption of active work using existing interruption behavior;
do not force interruption known to corrupt work. After confirmed completion/
cessation, Interactive returns EndedBy human so ordinary workflow code completes
successfully. Unknown cessation is an error, not successful completion. No new
general interruption-safety mechanism is required.

Reject messages after Finish; earlier undelivered messages become unprocessed.
Earlier replies stay visible and a late result cannot reopen ingress. Agent end
completes an occurrence, not the stable ingress. An already completed occurrence
keeps its EndedBy; a pending Finish reaches the next occurrence. Actual owner
termination closes ingress and leaves remaining inputs visibly unprocessed.

## Existing ownership boundaries

These responsibilities use existing packages, not new frameworks or separately
delivered slices. Retain the prior maximum handwritten production line budgets;
do not create new counting infrastructure or expand budgets for completeness.

| Cell | Owns and boundary | Maximum lines |
| --- | --- | ---: |
| J | runstore/runstate: durable ordering and atomic batches; no native execution | 1,400 |
| E | root runtime/session/commands and executors: lifetime, saved results, native identity/delivery; no interaction-state callbacks | 9,000 |
| I | Interactive: admission, protocol, transcript and control decisions using J/E | 900 |
| H | host/runner/invocation/conversation/taskdaemon/transports: saved invocation, startup re-entry and routing; one Run lifetime | 8,500 |
| P | observation/run views/conversation UI: committed views and control submission; no execution decisions | 14,800 |
| A | admission/generation/workflows/examples: library-only effects, schemas and diagrams | 13,200 |

Counts include blanks/comments and exclude tests, fixtures and generated code.
Observation follows committed execution facts; a projection error cannot turn
authoritative native success into execution failure.

Workflow effects and nondeterministic observations use Gimbal library operations.
Preserve completed admission/migration work and approved complete RunCommand
stdout/stderr returns with bounded display excerpts. Earlier defer/recover/timing
restrictions do not justify rebuilding suspension or another corpus migration.
Regenerate generated source; remove replaced Interview paths without shims.

Preserve existing file context, metadata, worktree/environment selection,
association, rename, idle model/effort controls and explicit cleanup. Conversation
and ordinary run views use the same Send/Stop/Finish path and show actual wait,
working, error and completion states. Local then remote uses the same contracts.

## Definition of done

| Claim | Kind / owner | Demonstrate |
| --- | --- | --- |
| C1 | Cell-local / J | Commit-before-ack, retry identity/conflict, atomic reply/wait and acknowledged input after reopening |
| C2 | Cell-local / E | Saved results/context/native binding restored; completed operations do not redispatch |
| C3 | Cell-local / E | Run pending at wait; ordinary joining/cancellation/cleanup and final deregistration |
| C4 | Cell-local / I | Chat/end/Finish and transcript semantics; ingress stays closed after Finish |
| C5 | Cell-local / E | One delivery when steering races completion; Stop preserves successful reply/end |
| C6 | Cell-local / H | Saved invocation re-enters once at startup; terminal runs stay terminal; routing preserves ownership |
| C7 | Cell-local / P | Display actual states and controls without deciding execution outcome |
| C8 | Cell-local / A | Interactive schema/inspection and existing library-only admission work |
| L1 | Integration / J,E,I,H,P | In the real local app, chat/wait, abrupt host restart, then continuation on the same run/native thread with earlier command result/context preserved and no duplicate reply |
| L2 | Integration / E,I,H,P | Send/steer, Stop/continue, agent end/send again, active archive and actual successful workflow return |
| L3 | Integration / E,I,H | A waiting branch permits a useful sibling and another project to progress, then joins normally |
| L4 | Integration / J,E,I,H,P | Same conversation controls and resume through the real separate daemon/messaging route after daemon and workflow-host restart |

Use package-adjacent tests for regressions and real app/native execution for
integration. Existing executor interrupted-operation recovery is exercised with
its focused checks; no exhaustive crash matrix. Use inexpensive unversioned
aliases (luna locally), and report observed behavior/model/source in chat or PR.
Green checks alone do not establish live behavior. Commit no proof programs,
screenshots, logs or run-output documents.

## Exclusions and deferred boundaries

No unwind-on-wait, dormant activation loops, stack serialization, actor/reducer
framework, new storage backend, scheduler, second inbox writer, universal repair
system or exactly-once external-effects guarantee. Persistence, saved-result
replay and startup re-entry are required, not excluded as “recovery machinery.”

SERVICE lifecycle/restoration/rechecks remain deferred; waiting leaves services
and claims alone. Version handling and native-process resource handling have
no policy or policy tests. No retained builds, version gates, Activity/file/clock
API, Python dependency or unrelated UI redesign. These boundaries do not block
ordinary resumability and must not be converted into implementation requirements.
