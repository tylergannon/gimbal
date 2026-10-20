# Interactive conversations

Status: replacement contract accompanying the rewritten delivery plan.
The latest human correction removes recovery machinery from this work.
Earlier requirements for stack unwinding, replay/reconstruction and automatic
restart continuation are superseded. This document describes intended
behavior; it does not claim the current implementation satisfies it.

## Outcome and public API

A conversation is an ordinary workflow hosted beside other workflows in
shared Go capacity. Replace Interview and its Q/A types with:

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

The agent response uses the generated Polytype union inside an object:

```json
{"decision":{"type":"chat","message":"…"}}
{"decision":{"type":"end"}}
```

A chat result ends the agent turn and commits its reply and next wait
together. Interactive remains pending. End returns normally with EndedBy
agent. Human Finish returns normally with EndedBy human. On error, return
the committed transcript and error without inventing a successful ending.
Messages use user/assistant roles; controls are not prose messages.

Each occurrence starts human-first. A conversation uses the same session
and an ordinary loop around Interactive, returning on human Finish.
Agent end therefore leaves it available for another human message without
an autonomous model loop. No additional startup modes are required.

## Ordinary lifetimes

Waiting parks the current Go goroutine. Run returns only on actual workflow
completion/error/cancellation. Scope remains open, Group joins normally,
and runnable siblings continue. All-waiting is an unfinished state, not an
error or successful return. No suspension panic, stack unwinding or
activation/re-entry mechanism is involved.

Waiting does not perform cleanup or change SERVICE claims. Actual
completion and cancellation keep normal cleanup and joining semantics.
A run holds one live registration until it exits, including across waits.
Shared hosting preserves each project's identity, context, workdir and
selected placement. A run failure must not crash unrelated hosted runs.
Browser disconnection does not cancel workflow work.

Accepted inputs and reply/wait transitions use the existing durable journal.
Persisting those facts introduces no new promise about execution after
process loss. Build no recovery scans, replay, reconstruction, reconciliation,
restoration or automatic retry system. Existing executor behavior is not a
project to redesign.

## Input, delivery and controls

The interaction address is run/scope/constant name. Repeated calls at the
same site share ingress; each occurrence retains its own result identity.
One occurrence consumes an input. Two simultaneous consumers of one address
are rejected.

One interaction owner admits all inputs and controls. Commit before
acknowledging. Repeating the same ID/content returns the original acceptance;
conflicting reuse fails. Journal sequence supplies input order.
The owner must be available to accept; otherwise return an error, without
acknowledging storage or creating a second absent-owner writer.

The session/executor owns delivery versus turn completion. Deliver once:
steer when active, queue for the next Interactive-dispatched turn when the
turn has ended. A message being delivered cannot simultaneously become a
new-turn prompt. Return the delivery outcome to the interaction owner.
No private next turn is launched behind the workflow's back.
A native error is reported; unknown delivery is not a reason to resend.
There is no exactly-once external-effects guarantee or repair protocol.

The native session owns its history and operation counters. Interactive
owns its transcript. Send new input and relevant instructions/context;
do not retransmit the accumulated conversation as every prompt.
Returned transcripts contain confirmed delivered user messages and
committed assistant replies for that occurrence. Queued or failed input
remains visible with its actual status, without fabricated delivery.
Pending input survives agent end into the next occurrence. Actual owner
termination closes ingress and marks remaining input unprocessed.

Stop targets the current turn and requests interruption, leaving Interactive
open for more input. It does not finish the workflow or cancel siblings.
If the native turn completed successfully, preserve its reply or end even
when Stop was requested just before the result arrived.
Confirmed requested interruption is normal control handling; its expected
cancellation result does not become an unexpected Interactive failure.

Finish durably closes ingress before acknowledgement and prevents new turns.
It requests interruption of an active turn when safe. After confirmed
completion/cessation, return normally with EndedBy human. Earlier committed
replies remain visible; later results cannot reopen a finished interaction.
New messages are rejected and earlier undelivered messages become unprocessed.
Agent end completes an occurrence rather than closing the conversation's
ingress. An already completed occurrence is not rewritten to change EndedBy;
a pending Finish reaches the conversation's next occurrence.

If delivery/interruption cannot be established, report the error and retain
the recorded facts. Do not claim success, automatically resend, or build a
reconciliation loop. No universal tool-safety detector or rollback system.

## Existing boundaries and authoring

These are ownership boundaries in existing code, not new frameworks or
separate delivery gates. Retain the previous source ceilings; no budget
expansion or new counting infrastructure is part of this change.

| Cell | Owns / contract | Existing maximum handwritten production lines |
| --- | --- | ---: |
| J | runstore and runstate storage: committed records, IDs, atomic batches; no native calls | 1,400 |
| E | root execution/session/command and existing executors: ordinary lifetimes and native delivery; no interaction-state callbacks | 9,000 |
| I | Interactive: input admission, protocol, transcript and control decisions; uses J and E | 900 |
| H | host/runner/invocation/conversation/taskdaemon and existing backend transports: placement, registration, routing; no second workflow lifecycle | 8,500 |
| P | observation, run views and conversation UI/remotes: display committed facts and submit controls; no execution decisions | 14,800 |
| A | gimballint/generate/workflow/built-ins/examples: accepted source, schemas and diagrams | 13,200 |

Counts include comments/blanks and exclude tests, fixtures and generated code.
The existing detailed source map is historical orientation, not extra scope.

Workflow effects and nondeterministic observations use library operations.
Keep the completed admission/migration work; do not enlarge it to support
this design. The earlier authored defer/recover and timing restrictions are
not a reason to build suspension machinery or conduct another migration.
RunCommand returns complete captured stdout/stderr with bounded display
excerpts. No Activity, file or clock API is introduced. Regenerate generated
source and remove replaced Interview paths without compatibility shims.

Preserve existing conversation metadata, file context, worktree/environment
selection, association, rename, idle model/effort controls and explicit
cleanup. Message, Stop and Finish work in both conversation and ordinary
run views. Local execution comes first, then existing remote transport;
both use the same ownership and delivery semantics.

## Required evidence

| Claim | Kind / owner | Demonstrate |
| --- | --- | --- |
| C1 | Cell-local / J | Input is committed before acknowledgement; retry identity/conflict and atomic reply/wait work |
| C2 | Cell-local / E | Run remains pending, cancellation joins, completed runs deregister; no cleanup at wait |
| C3 | Cell-local / I | Chat/end/Finish return and transcript behavior; no post-Finish input or reopening |
| C4 | Cell-local / E | Steering racing turn completion has one delivery; Stop preserves a successful result |
| C5 | Cell-local / H | One registered owner and shared placement/routing; unrelated runs remain controllable |
| C6 | Cell-local / P | Actual unfinished/error/terminal state and controls shown correctly |
| C7 | Cell-local / A | Interactive schemas/inspection and existing library-only admission still work |
| L1 | Integration / J,E,I,H,P | Real local conversation: send, steer, Stop/continue, agent end/send, active archive and actual successful workflow return |
| L2 | Integration / E,I,H | A waiting branch permits a useful sibling and another project to progress, then joins normally |
| L3 | Integration / J,E,I,H,P | The same conversation controls work through the real separate daemon/messaging route |

Use package tests for repeatable regressions and the actual app/native agent
for integration. Use an inexpensive unversioned model alias, report which
ran, and describe observed behavior in chat/PR. No committed proof programs
or run output. A successful test gate alone does not prove the app works.

## Exclusions

No new recovery machinery or restart guarantees. No stack serialization,
alternate workflow framework, actor engine, new storage backend, scheduler,
or absent-owner writer. No universal interruption safety or exactly-once
external-effect guarantee.

SERVICE restoration and rechecks remain deferred; waiting leaves all
services and claims alone. Version changes and native-process resource
handling have no policy and no enforcement tests. No retained builds,
version gates, Activity API, Python dependency or unrelated UI redesign.

The [delivery plan](../ephemeral/research/durable-interactive/DELIVERY-PLAN.md)
is the coding handoff. Older review dispositions describe the abandoned
implementation; they add no requirements to this replacement.
