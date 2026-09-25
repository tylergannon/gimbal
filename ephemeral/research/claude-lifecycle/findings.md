# Claude waiting and structured completion: investigation synthesis

This is a decision brief, not an implementation specification.

Updated after the [Generate-lifetime probe](generate-lifetime.md): prefer one
process per logical Generate, retained through waiting and continuation, then
closed after terminal completion. A new process successfully resumed the same
conversation with different schemas and with no schema. The session-wide
process option discussed below is no longer the smallest proposed direction. Sol and Terra
investigated independently using live Claude Code 2.1.270 with
claude-haiku-4-5-20251001. Their notes distinguish observations from proposals:
[Sol](sol-semantics.md), [Terra](terra-lifecycle.md). The companion
[literature report](literature.md) collects versioned documentation, source,
and historical reports. The assignments are in [briefs/](briefs/common.md).

The [server-hosting follow-up](server-hosting.md) distinguishes Claude’s
multi-session servers from their per-session worker processes.

## What actually happens after a waiting result

A native successful result closes one generation. It does not inherently close
stdin, stdout, the SDK stream, the native process, or the conversation. With
one persistent reader and open input, the observed sequence is:

1. Claude launches background work. The stream identifies native tasks and
   publishes snapshots of active background work.
2. Claude emits a successful result saying it is waiting. This can include a
   valid structured object and terminal_reason completed. These describe the
   generation, not completion of the assigned work.
3. The stream remains connected while no model request is active. Background
   work can continue. Receiving a result does not require a disconnect.
4. Background completion or failure produces task lifecycle events and a
   task_notification. A model continuation can consume more than one
   notification; do not assume one notification always yields one result.
5. Claude can use more tools and emit another result, with task-notification
   origin in the observed automatic-wakeup cases. The structured value is a
   new complete object, not a patch or accumulation of previous values.
6. Later host prompts can be processed by the same process. They and native
   continuations share the event stream and must be attributed correctly.

A failed shell task is not automatically a failed native model result. Sol's
probe reported the task's failure, then returned a successful model result.
The host needs the assignment's meaning to decide whether that failure is
fatal, recoverable, expected, or irrelevant.

Gimbal currently ends the process at step 2. Its next call resumes conversation
history in a new process. The observed new process emits a stopped/orphan-task
notification and an empty successful notification-originated result before the
actual prompt answer. Selecting the first result therefore both loses the
continuation opportunity and misattributes the next call's result.

## Can existing fields deterministically identify waiting?

No sufficient native predicate was established. The concrete counterexamples
are stronger than absence of documentation:

| Proposed predicate | Why it is insufficient |
| --- | --- |
| result / subtype success | A waiting answer has both. |
| terminal_reason completed | Waiting and final answers both have it. |
| schema-valid structured_output | The waiting object validates too. An orphan-notification result can conversely lack it despite a configured schema. |
| no running background tasks | A deliberately persistent service can remain after legitimate completion. Other dependencies might not be native tasks at all. |
| task_notification completed | A particular task completed; the model may still need to inspect it, other tasks may remain, or recovery may be necessary. |
| origin task-notification | Classifies the source of a continuation; does not decide whether its answer finishes the assignment. |
| increasing result_index | Orders results within the observed process; does not establish task identity or completion and resets on process restart in the observed case. |
| silence / idle state | No model activity can mean waiting, completed, blocked, or broken. |

Sol deliberately elicited a waiting result with several background jobs, then
also ran a natural assignment without asking for an early answer. In the
natural case Claude completed the required finite job and returned its final
answer while a service remained running. Thus the same broad native state
(successful structured result plus running background work) can call for
different host behavior. The difference was the meaning of the assignment and
the model's requested structured declaration.

The SDK also names special terminal reasons background_requested (the turn
itself moves to background) and tool_deferred. These can identify their specific
yield paths, but they do not cover the observed conversational waiting result,
which said completed. They are not a general waiting predicate.

An explicit waiting/completed/failed declaration makes classification
mechanical. It does not make the model infallible. Just as a normal final
answer can be wrong, the model can declare completion early or fail to declare
a dependency. Where the workflow knows an external completion check, that
check can strengthen the decision. Do not present parsing as proof of truth.

## The smallest direction worth testing

Preserve Claude's background capabilities within the logical Generate call.
Keep its native client/process and one stream reader alive through intermediate
results, then close after the associated terminal outcome. Resume the same
conversation in a fresh process for the next call. The pinned Go SDK supports
multiple generations through Stream.Messages; a native result need not end the
logical call. Session-wide ownership remains an alternative only if live work
must span calls.

For an assigned task, require an explicit structured declaration associated
with that assignment. A waiting declaration keeps the logical Generate pending;
it is recorded as an intermediate answer. Notifications and their model
continuations stay attached to that pending assignment. A terminal declaration
must contain its complete final value or task failure, not merely the latest
background-task output. The host validates the final value before returning.
Known awaited dependencies and intentionally persistent services need distinct
meaning; native task counts cannot invent that distinction.

This is a proposed completion policy, not a finished or proven fix. A minimal
implementation may keep the same RunTurn callback alive across intermediate
results. Events after a genuinely terminal return, including service events,
need a session-level route or another explicitly defined retention/attribution
policy: the current callback is per invocation and is stamped with that turn.
One reader must consume the shared SDK channel; multiple per-call readers can
race for messages. Ordinary prompts and automatic continuations require
explicit routing. Origin helps, but does not alone supply a host assignment ID
or prove a prompt/result mapping under every queue race.

Explicit TaskOutput(block=true) remains a demonstrated smaller workaround when
a known finite job must complete within one call. It relies on Claude following
the waiting instruction and does not repair wrong-result attribution. Current [tools documentation](https://code.claude.com/docs/en/tools-reference)
deprecates TaskOutput in favor of reading task output files; the successful
blocking probe establishes a workaround on the tested version, not a durable
completion contract.

## Compatibility question: per-call output schemas

Waiting and automatic resumption do not require changing the schema. One fixed
schema can express waiting, completed, and failed for the entire assignment.
The compatibility question arises only when successive Generate[T] calls in
one Gimbal session request different result types: today each call starts a
new process configured for that type. A persistent process removes that
implicit schema boundary.

Terra tested the native control protocol, not just the Go convenience API:
initialize with schema A, then reinitialize with incompatible schema B, then
reinitialize with literal jsonSchema null. All control requests acknowledged
success. All subsequent outputs still followed A. Reinitializing with B during
a background wait also left the eventual notification result constrained to A.

Therefore no working schema-change/clear path has been verified on CLI 2.1.270.
This does not prove that no other or future protocol path exists. It does mean
we cannot promise arbitrary per-Generate native schemas simply by retaining the
existing process.

Fresh-process resume now provides a demonstrated schema boundary between
completed calls. The following alternatives matter only if a process must
stay alive across differently typed calls:

- A fixed native schema for a session whose tasks genuinely share a result type.
- One stable native envelope containing assignment identity, waiting/completed/
  failed state, and a payload; Gimbal validates the payload against the caller's
  requested type. This retains native envelope validation but changes where the
  caller's exact type is enforced. Waiting/failure variants must not fabricate
  fields of a completed result.
- Change processes only at a safe schema boundary after pending work is settled.
  This retains per-call native enforcement at that boundary, but cannot preserve
  live native background work across it by assuming resume is equivalent.

None of these alternatives was implemented. The stable-envelope idea is a
candidate to test, not a claim that a new generic API or framework is required.

## Listening and minimizing data loss

Keep the reader and native process alive through intermediate results. In the
pinned SDK, Stream.Close stops that stream's sending/listening while Client.Close
tears down its transport. Closing stdin signals input exhaustion; it is not a
way to tell Claude to wait. The transport's shutdown grace concerns the top-level
process and is not a guarantee about every descendant.

If delivery to an application/UI disconnects while Gimbal still owns the native
process, keep consuming into the existing local store and replay what was
actually retained to the downstream consumer. Exactly which additional native
fields or events need retaining is an implementation decision; this does not
adopt a general raw-payload recording requirement.

If the native stdio connection or process is lost, --resume starts from stored
conversation context. It does not replay the old event stream or resurrect a
killed computation. Task outputs and transcripts can help reconstruct facts,
but missing evidence stays missing. Reconcile known task identities and effects;
do not silently mark outstanding work successful or automatically repeat
possibly completed non-idempotent work. Any continuation should explicitly
re-check the unresolved task before producing the eventual terminal result.

The [environment reference](https://code.claude.com/docs/en/env-vars) also
documents a separate supervised background-session handoff mechanism. This was
not tested and must not be conflated with ordinary SDK close/resume; it remains
an alternative hosting path to investigate if process replacement is required.

On cancellation or unrecoverable transport loss, resolve the caller as cancelled
or failed rather than substituting an intermediate waiting response. Guaranteeing
progress forever is impossible if a task hangs or the provider fails; an explicit
cancellation/deadline/failure policy is required. An inactivity timeout can bound
waiting but cannot prove successful completion.

## What is settled and what remains

Settled by the observed cases: persistent listening is possible; native result
success is not assignment completion; waiting and final structured values are
separate; first-result selection can consume an unrelated empty notification;
persistent services invalidate a universal wait-for-all rule; the tested schema
reinitialization and clearing attempts leave the original schema in force.

Not settled: a fully exercised completion-envelope policy; all notification/
prompt/steer races; dynamic schema control beyond the tested path; a lossless
reconnect/replay guarantee; background-subagent equivalence to the shell probes;
or compatibility across other Claude versions and models. The next useful
experiment is a bounded prototype of the chosen completion and schema policy,
including task failure, service survival, cancellation, and late notifications.

The literature corpus is local at /private/tmp/gimbal-317-literature/corpus/.
It contains synthesized source extracts and clips, not a complete mirror of the
originals. Use the cited primary URLs and actual probe captures for exact text
or wire-byte claims.
Historical issue reports were independently audited against their original
bodies/comments; closed/fixed reports and causal hypotheses must not be presented
as current provider guarantees. Probe programs and raw captures remain outside
the repository in the temporary paths cited by the individual notes.
