# Distributed integration continuation

This is the next implementation assignment after source-stage review and the
conversation C1 live checkpoint. Tyler already authorized the whole goal.
Work in `/Users/tyler/.codex/worktrees/1430/gimbal`; the authoritative requirements
and acceptance denominator are `ephemeral/plans/distributed-execution-slice.md`.
Preserve the conversation prerequisite and the existing named River path.

Implement the parent plan's maintained opt-in integration test beside registered
invocation/task dispatch. It registers an ordinary Go workflow and exercises the
actual River plugin and separately started participants on two real computers.
Do not commit a standalone proof program, relabel one as an example, or add a
new exported workflow helper. A testdata workflow plugin used by the maintained
test is appropriate if required by the real application catalog. Generate its
declarations through the owning generator; do not fabricate graph metadata.

The registered workflow stays on A, selects `workflow-local`, and runs a command
there that returns hostname and a fresh nonce. Use ordinary Set/SetJSON to pass
that value, an inherited/shadowed value, and a large complete-context value whose
buried marker is unavailable from its preview. In a named scope select
`task-remote`, start a real native `luna` session in the explicitly supplied B
directory, and have it read the complete value and an explicit TaskFile byte input,
return a deterministic derived result and its hostname, then demonstrate a
follow-up in that same session. Validate actual values and admitted identities.
Both A's command and B's session use the same selected bus. Native directories
are owner-interpreted inputs; no automatic checkout/sync or peer endpoints.

The test must support an already-running isolated application instance, private
route-file path and existing A/B workers so the actual run is visible in the
normal page. Its failure must identify missing external setup, never silently
substitute in-process handlers or another transport for the two-computer claim.
Root owns deployment and browser interaction; do not reset shared services.

Provide a controlled native restart specimen using the existing retained task
state and real completed native result whose publication is interrupted. The
same worker identity and state directory recover it once, without rerunning the
native work. Keep all failpoints/test controls scoped to the test/owned fixture;
do not add an operator-facing fault injection framework. Exercise independent
observer disconnect/reconnect and retrieve final output bytes through River.
If a real test requires a narrow internal test hook, explain its owning seam.

One possible D7 approach requires no product failpoint: in the owned disposable
database, install a test-only BEFORE INSERT trigger on gimbal_message_events,
restricted to this test's queue/run/request and Kind=completed from the bytea
JSON envelope. Let it wait on a test-held advisory lock. At that point the real
daemon has durably saved completion before PublishEvent. Verify that state and
the waiting publication, stop only the owned B worker process, remove the trigger
and release the lock, and restart that worker under its same identity/state.
Observe original event/native identities and the command's once-only marker.
The test must clean up its trigger/function/locks on all exits and never affect
other queues or databases. This is a suggestion, not a new authority contract;
inspect the actual persistence/publication order before using it.

Root prepared an owned Compose database, separate live database from source
tests, private A/B state directories, and private route files. Ask root for the
current concrete paths in the Gimbal assignment before operating participants.
Credentials are files: never print contents. Do not touch other databases,
workers, Desktop, installed Gimbal or the shared Codex app-server daemon.

Run appropriate tests/builds. Report exact changes, actual observed behavior and
remaining claims; no source-only result satisfies D6–D10. Do not commit, push,
merge, publish releases, change requirements, or claim browser/connection review
that has not occurred. Root checkpoints meaningful progress and independently
validates the final composition with Sonnet.
