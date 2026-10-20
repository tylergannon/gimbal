# Durable Interactive execution

Status: authoring draft. Goals and decisions below reflect the human's brief
and answers. The cell decomposition and budgets are proposed for review;
contracts and claims are not yet spanning. This draft does not govern
implementation.

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
an error, preserving the useful return contract of `Interview`. Its public
type names and exact transcript representation remain to be specified.

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

## Proposed public shape

The replacement keeps the existing call arguments. The two transcript type
names below are proposed public names, pending approval of the contract:

```go
func Interactive(ctx context.Context, name string, session *Session, purpose string) (InteractiveTranscript, error)

type InteractiveTranscript struct {
    Messages []InteractiveMessage `json:"messages"`
}

type InteractiveMessage struct {
    Role    string `json:"role"`
    Message string `json:"message"`
}
```

Roles identify human and assistant messages as `user` and `assistant`.
The transcript is the ordered interaction conversation; provider tool events
remain in their existing observation surface. The agent's chat/end decision
is a generated Polytype tagged union and needs no new exported decision type.
Replacement removes Interview and its Q/A-only types rather than retaining a
compatibility alias. Initial turn behavior and the precise delivery/terminal
ordering are still open before this public shape becomes binding.

## Explicit exclusion: SERVICE lifecycle

Suspension must not close, release, or otherwise alter any service or SERVICE
claim. Existing service behavior and claims are left alone. Rechecking or
restarting SERVICE claims on resume and service-restoration guarantees are
deferred. This spec must not acquire those requirements indirectly through
cleanup, replay, recovery, or validation claims.

## Authoring work remaining

The [source map](SOURCE-MAP.md) records current code facts and cached prior art.
Next: align the proposed cells and budgets, then harden contracts external
seams first and define routed behavioral claims and exclusions. Run
spec-review before this draft governs work.

Remaining contract decisions include the public transcript representation,
initial entry into Interactive, the resources released by dormancy, and
steering/termination races. Proposed answers must preserve the settled goals;
these are not implementation permission to choose silently.
