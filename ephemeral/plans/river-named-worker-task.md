# Implement the named-worker River path

Authorized next coding outcome in `/Users/tyler/.codex/worktrees/1430/gimbal`.
Read the current working tree first; preserve the preceding declaration/preflight
and clock-skew/completion-ID changes. Parent contract:
`ephemeral/plans/distributed-execution-slice.md`. Do the implementation, not just
an inventory; report only a specific underivable blocker if one prevents it.

Outcome: the same River connection carries new work addressed to a configured
logical environment and continued session operations addressed to their retained
native worker. Two manually started workers use different stable identities on
one Postgres intermediary. No orchestrator, environmentMessaging, task endpoint,
per-peer plugin connection, SSH or native direct-execution fallback supplies work.

Implement:

1. Strict plugin-owned payload/schema/defaults and the pure namespace callback used
   by preflight. The common route envelope does not decode River-specific fields.
   Configure a few logical names mapped to stable worker identities, plus private
   DSN-file reference and local worker identity at worker startup. Validate before
   connection/credential read. Reject conflicting legacy placement/provisioning.
2. Add logical destination/retained-owner identity to the existing request/event
   contract. Effects submit the captured environment. Persist session-owner binding
   before ambiguous delivery; turns/controls/cleanup reach the original worker
   after name rebinding. Preserve authoritative native session/operation identities.
3. Workers register with exclusive database authority; same-ID concurrent instances
   cannot admit together. Loss/uncertainty of authority pauses consumption and
   admission without spending a River attempt or cancelling queued work. Ownership
   commits use the lock-owning connection or equally sound atomic fencing. Reacquire
   within the parent reconnect budget, otherwise exit explicitly. Add the existing
   executor-state directory lease to message-worker. No heartbeat/lease framework.
4. Keep acceptance distinct from native completion. Reuse taskdaemon/executors.
   On startup, reconcile and republish retained completions with their original
   identity even when River does not redeliver an admission. No blind native replay.
   Missing native evidence stays unresolved; already admitted native work is not
   cancelled merely because observation/registration reconnects.
5. Full identity-based replay/live delivery and bounded reconnect for waits/watchers;
   preserve independent readers and source sequence. Explicit no-owner at submit
   rejects before publication; uncertainty is not reported as definite non-admission.
6. Carry explicit input/output and complete-context bytes via the selected boundary.
   No direct ReadBytes/WriteBytes HTTP shortcut. Record actual logical/worker identity
   in accepted/completed facts for the observation projection, not guessed labels.

Use focused tests with actual isolated Postgres for database contracts; package
tests belong beside code. No proof program, run dump or screenshot is committed.
An owned Postgres 17.11 is healthy at the service described by the private file
`/tmp/gimbal-distributed-owner-b/live/postgres.dsn`; do not print its contents.
You may use only that disposable database, with isolated test namespaces/queues,
for this assignment. Do not touch other Docker databases or shared services.

Run affected tests/race/build checks. Do not weaken existing tests to hide a
regression. Do not start remote native workers, commit, merge or publish. Actual
conversation and A/B live proof remain coordinated by this chat and independent
Sonnet validation. Report changes, verification, remaining claims and cell-budget
usage. Stop/report a real budget breach or contract defect before expanding scope.
