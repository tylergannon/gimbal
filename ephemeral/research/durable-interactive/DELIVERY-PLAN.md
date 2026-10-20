# Working Interactive conversations

Status: replacement plan, 2026-10-19. This is a coding handoff, not a claim that
the implementation works. The human's latest instruction is decisive:
**“NO … RECOVERY MACHINERY … DO NOT SOLVE PROBLEMS THAT WE DO NOT HAVE.”**

## Deliver this

Replace Interview with Interactive. A conversation is an ordinary workflow
running in shared Go capacity. Interactive waits for input, runs an agent
turn, publishes chat, and waits again. Agent end returns from the call.
Human archive returns normally with EndedBy human so the workflow completes.

**Waiting blocks the goroutine.** Run remains pending, scopes remain open,
and Group siblings continue. Retaining idle Go stacks is fine.

Use the existing journal to persist accepted inputs and commit reply plus
wait together. That storage does not require an execution recovery system.
Do not add replay, startup reconstruction, recovery scans, activation loops,
reconciliation, restore routines, or restart guarantees. Leave the executor
behavior that predates this feature alone except for the delivery change below.

## What went wrong, and what replaces it

I read the source chat from the original request through its final
simplification discussion: 01a156a2-13b2-7511-ad6f-171ab6e30038.

The prior implementation made a wait unwind the workflow and then rebuild
it. Run and two hosts each owned part of the lifecycle. Tests even expected
Run to return success while unfinished. Another path could deliver one
message both as a steer and as a new turn. Finish did not close input; Stop
could discard a successful reply; waited runs could lose deregistration.

Replace those mechanisms, rather than adding more checks around them:

| Concern | Simple owner |
| --- | --- |
| Workflow lifetime | Run, ordinary scopes and Group |
| Interaction decisions and transcript | Interactive's loop |
| Native history, operation IDs and message delivery | Session/executor |
| Durable records | Existing journal |
| Display | Existing observation layer |

No actor framework, general reducer/effect framework or new public wrapper.

## Use the useful existing work

The failed branch is codex/durable-interactive-spec at cc976137. Its original
worktree also contains unfinished repairs. Preserve that work while reading
it; it is material to reuse, not an accepted implementation.

Keep useful chat/end types, generated schemas, journal batching, ordinary
conversation workflow, UI routes and completed workflow migrations. Keep
the approved complete RunCommand return values and library-only effect rule.
Do not redo the corpus migration or broaden the linter.

Compare Run/Scope/Group with ed50f6bd for their ordinary lifetime behavior.
Remove suspension panics, suspension-specific Group handling, cleanup bypasses,
dormant tables and host polling/re-entry loops. Remove the absent-owner inbox
writer and the new replay/reconstruction paths added by this feature,
including replay.go and their hooks in Run, sessions, commands and hosts.
Delete interaction copies of session counters and callbacks that
mutate interaction state from inside Session. Do not rebuild the full
transcript into every prompt; use the native session's history.

## Implement sequentially

1. **Make one local conversation work.** Interactive commits waiting state
   and blocks on input or cancellation in the same call. Native work reports
   results back to that loop so Send, Stop and Finish remain responsive.
   No native callback takes interaction locks or commits interaction state.
   Run owns one registration until actual completion and then releases it.
   A wait does not return, throw, cancel or run cleanup.

2. **Serialize message delivery once.** The interaction owner commits an
   input before acknowledging it. Use its stable ID for duplicate transport
   requests. All input comes through this one owner. If it is unavailable,
   return an error and retain the sender's text/ID for retry; do not create
   an alternate storage owner.

   The session owner orders message delivery against turn completion.
   Active turn: steer. Turn already finished: retain for the next turn and
   notify Interactive. This implements the human's “deliver anyways if it
   arrives after the turn ends” behavior through a small private control
   change if needed. The session reports delivered or queued; Interactive
   never independently resends a message that is still being delivered.
   Queued input starts its next turn through ordinary Interactive execution.

   A native error is an error. Report it; do not guess that delivery failed
   and resend, inspect old executors to repair it, or build a recovery state
   machine. Use the same owner's queue and buffered notification so draining
   committed input and going to sleep cannot lose a signal. No host poller.

3. **Complete controls and display.** Commit chat plus waiting in one journal
   batch. Feed new messages and purpose/scoped context to the existing native
   session. Agent end returns EndedBy agent; the conversation's ordinary loop
   calls Interactive again and waits for the next human message.

   Stop interrupts the active turn and leaves the interaction open. Preserve
   a successful native result when Stop arrived too late to stop it.
   Finish closes ingress before acknowledgement, prevents another turn,
   requests safe interruption, then returns EndedBy human normally.
   Reject subsequent new messages. Mark earlier undelivered messages
   unprocessed. Do not let a late result reopen the interaction.
   If interruption fails or its outcome is unknown, report that failure;
   do not label the workflow successful or launch repair machinery.
   Confirmed requested interruption is normal Stop/Finish handling, not an
   unexpected native failure that terminates Interactive.

   Preserve the existing file context, placement/worktree, association,
   rename, idle model/effort controls and explicit cleanup. Ordinary run-view
   controls use the same input path. Keep the frontend changes small and
   show the actual waiting, working, failed and completed states.

4. **Use the real local app, then the existing remote path.** With luna and
   disposable project/catalog data, send and receive, steer, Stop and
   continue, agent-end and send again, then archive an active turn. Observe
   actual successful workflow completion. Run a waiting branch alongside a
   useful sibling and another project. Only then route the same behavior
   through the existing separate daemon/messaging path and exercise it.
   Remote transport adds no recovery or second lifecycle implementation.
   Use the documented decision envelope across harnesses; schema syntax is
   a small implementation detail.

## Check the actual defects early, then finish

Keep package-adjacent regressions for these observed failures:

- Run stays pending at a wait; a sibling progresses; cancellation joins.
- A delayed steer acknowledgement racing turn completion delivers once.
- Finish rejects later input and a late result cannot reopen it.
- Stop racing a successful turn preserves its reply/end.
- An actually finished run deregisters after having waited.

These tests support the live app observations above. No restart/crash matrix,
new verification framework or committed proof programs.

One implementer is sufficient. No more subdivision or repeated setup.
Run focused checks while editing and one set of required affected build,
Go, frontend and analysis checks at the coherent implementation. Reuse valid
unchanged results. The earlier requested Opus review should check concrete
behavior and defects, not introduce new requirements or another spec loop.
Report what worked, the model used and any real remaining defect, then stop.

SERVICE lifecycle and claims remain untouched by waiting. Restoration is
outside the work. Version handling and native-process resource handling get
no policy, machinery or policy tests. No Activity API, Python detours,
broad UI redesign or unrelated feature work.
