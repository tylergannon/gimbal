# Resumable workflows with ordinary Go waits

Status: replacement coding handoff, 2026-10-19. Planning only; implementation
has not been validated. The contract is [the spec](../../../specs/durable-interactive.md).

## Deliver this

A workflow waits for human input without finishing. After the process restarts,
the same run continues with its saved inputs, completed results, scoped context
and native session. Conversation tabs are ordinary workflows using Interactive.
Send steers an active turn; Stop leaves the interaction open; archive finishes
the workflow successfully. Deliver locally first, then through the remote path.

**Keep the Go goroutine parked during a wait. Reconstruct only after process
restart.** Retaining idle stacks is an accepted tradeoff. Run stays pending,
scopes stay open, and Group siblings run normally.

Persistence and replay are the resumability being built. They stay. There is
no additional workflow recovery subsystem to build around them.

## Lessons from both failed attempts

This handoff incorporates every turn of chats
01a156a2-13b2-7511-ad6f-171ab6e30038 and
01a157a6-95bb-7ba0-a30c-63edc411ea4a. Their lessons:

- Unwinding on every wait forced special cases into Run, Group, scopes,
  cleanup and hosts. Delete that mechanism; blocking already yields execution.
- Run returning success while unfinished was a defect, even though tests
  expected it. Test the public lifetime contract before a large migration.
- A message could be steered and also dispatched as a new prompt. Give delivery
  versus turn completion one owner instead of adding competing fallback paths.
- Interaction state copied session counters, callbacks mutated across layers,
  and commits mixed execution with presentation. Each concern needs one owner.
- Review findings exposed real bugs but also fueled scope growth. A reviewer's
  proposed enhancement does not become a requirement.
- The second attempt removed replay and restartability in response to rejecting
  extra recovery machinery. That changed the task. Simplify the mechanism while
  preserving the requested outcome.

Earlier suggestions to drop steering or remote resumability were not accepted.
The human explicitly requested local followed by remote. Keep both.

## Start from the existing work, without inheriting its design

Baseline main is ed50f6bd. The failed implementation is committed through
cc976137 on codex/durable-interactive-spec, with unfinished repairs in its
original worktree. a6adf69d changed planning documents and incorrectly removed
restartability. This revision replaces that plan and contract.

Preserve those worktrees. Reuse useful types, schemas, atomic journal batches,
recorded operation results, native bindings, ordinary conversation workflow,
UI wiring and completed admission/migration work. These are candidates, not
proof of correctness. Compare ordinary Run/Scope/Group lifetimes with main.

Delete suspension panics, dormant-run tables, per-message reconstruction,
cleanup suppression and host wait/poll/re-entry loops. **Keep and simplify
replay.go's saved-result lookup and the replay hooks needed after restart.**
Remove the interaction's duplicate session counters and backward callbacks.
Do not repeat the workflow corpus migration or invent another linter project.

## One ownership model

| Owner | Responsibility |
| --- | --- |
| Run and ordinary scopes/Group | Logical lifetime, joining and final cleanup |
| Interactive loop | Input/control decisions and its transcript |
| Session/executor | Operation identity, native history, turn completion and delivery |
| Existing journal | Durable inputs/results and atomic reply-plus-wait |
| Host | Saved invocation, startup re-entry and routing to one live run |
| Observation | Display committed facts and submit controls |

Use a small Go loop with channels. Native actions report results to its owner;
callbacks do not take interaction locks or commit interaction state. No actor,
reducer/effect framework, scheduler or new public workflow wrapper.

## Build in this order

1. **Prove a small local workflow resumes.** Run a recorded command, set scoped
   context, create a session and enter Interactive. Waiting blocks in the call.
   Run must still be pending, with no scope teardown. Commit accepted input
   before acknowledging it and commit chat plus the next wait in one batch.

   On restart, reopen that run's journal and re-enter the supplied Go body under
   the same run ID. Stable scope/name/ordinal slots identify operations. Return
   saved results for completed operations, reconstruct context through the same
   deterministic workflow code, restore session bindings, and continue the
   unfinished interaction. Session owns restoration of its operation sequence;
   Interactive does not keep a second copy. Completed work must not redispatch
   or repeat cleanup merely because the body is being traversed again.

   The host saves invocation inputs, bindings and routes and re-enters unfinished
   invocations once at startup. Direct Run callers supply the body and bindings
   again with the same ID. This startup work is necessary resumability; it is
   not an activation loop on every message. Preserve terminal invocations as
   terminal. Existing journal writer exclusion prevents competing owners.

   Use the existing executor's persisted request/result and native-session
   support for an operation interrupted by restart. Reattach/read that operation
   under its recorded identity. If its outcome cannot be established, return
   an actionable error with the recorded facts intact. Do not start replacement
   work or add a second reconciliation protocol.

   **Before expanding the change**, use the real local app with luna: obtain a
   chat reply and wait, abruptly stop the host, restart the same binary, send
   again, and observe the same run and native thread. Check that the earlier
   command did not execute again, context remains available and the first reply
   was not duplicated. Retelling the whole transcript is not session continuity.

2. **Make delivery and controls correct in that working path.** One interaction
   owner admits inputs. Commit their stable IDs; identical retries reuse the
   acceptance and conflicting reuse fails. One session owner serializes delivery
   against turn completion: steer while active, otherwise queue for the next
   Interactive-dispatched turn. This is the requested “deliver anyway” behavior.
   Assign the message before starting delivery; it cannot also be a new prompt.
   Keep the small private delivery change at that boundary, not in every harness.
   Unknown delivery is an error, not permission to resend.

   Subscribe/establish notification before checking the durable queue so input
   cannot be lost between draining it and sleeping. Notifications are wakeups;
   committed input is the authority. Restore the owner before admitting input
   during startup. If unavailable, return a retryable error and keep the sender's
   text/ID for retry. This accepts a brief startup retry window and avoids a
   second absent-owner inbox writer. Already acknowledged input survives restart.

   Stop interrupts the current turn and leaves Interactive open. Preserve a
   successful reply/end when interruption arrived too late. Finish closes ingress
   durably before acknowledgement, prevents further turns and requests active
   interruption using existing behavior. Confirmed cessation allows normal return
   with EndedBy human. Unknown cessation reports an error; do not invent a safety
   detector or repair loop. Agent end returns EndedBy agent, and the conversation's
   ordinary loop calls Interactive again, waiting for human input.

   Send new input plus purpose/scoped context to the native session. Keep the
   transcript for display and returned results, not as a replacement for native
   history. Observation follows the committed transition; display failure cannot
   overturn a successful execution result.

3. **Finish the app path, then remote resumability.** Preserve existing file
   context, worktree/placement, association, rename, idle model/effort controls
   and explicit cleanup. Use the documented decision envelope across harnesses;
   schema syntax is a routine implementation choice. Regenerate generated files.

   In the local app, exercise steering, Stop/continue, agent end/send again and
   active archive-to-success. Run a waiting Group branch beside a useful sibling
   and another project. Then use the same run/input ownership through the existing
   separate daemon/messaging route. Reuse the saved daemon session/task indexes
   where needed; local and remote hosting share startup re-entry behavior rather
   than independently implementing workflow lifecycles. Restart the daemon and
   workflow host and observe continuation with the same run/native session.

## Verify the known failures, then stop

Keep focused package-adjacent regressions for saved results/context/session
identity after reopening; Run pending at wait; sibling progress and normal join;
one delivery when steering races completion; Finish closing ingress; Stop
preserving a completed result; and final deregistration after a run has waited.
Exercise acknowledged-but-unconsumed input across restart. These are concrete
requirements and previously observed bugs, not an exhaustive crash matrix.

One implementer is enough; retain the earlier Sol 6.1 choice when execution is
authorized. Use focused checks during editing, then one coherent affected
build/test/analysis pass and the live observations above. The requested Opus
review checks the completed behavior and material defects. Do not turn it into
another series of speculative spec rounds. Reuse unchanged valid checks.

Report what actually worked, source revision, model and remaining defects in
chat/PR. Commit no proof programs, screenshots, logs or run-output documents.
This planning task does not execute the implementation.

SERVICE claims/lifecycle/restoration remain outside this work. Version changes
and native-process resource handling have no policy and no enforcement tests.
No retained builds, version gates, Activity/file/clock API, Python dependency,
exactly-once external-effects promise, universal repair system or UI redesign.
