# Conversation-first prerequisite: implementer handoff

Tyler authorized implementation after selecting conversation migration first,
as an unmerged prerequisite to the single distributed-execution task. Work in
`/Users/tyler/.codex/worktrees/1430/gimbal`, branch `codex/distributed-execution-slice`.
Parent contract: `ephemeral/plans/distributed-execution-slice.md`, including C1.
Production base is bc9e158d, incorporating #665. Preserve its ownership/lifetime fixes.

Implement the minimum logical-addressing/retained-owner River substrate and the
actual conversation consumer. Conversation UI selection becomes a declared string
workflow input, resolved against plugin-owned namespace before effects. Extend
the existing extraction/catalog/preparation path for constants and direct input
fields; reject arbitrary computed names. Expose the proposed `gimbal.WithEnvironment`.
New command/session requests name logical environments; continued operations use
their admitted owner. Do not route task operations via host endpoints or SSH.

Current path to replace:

    conversation UI -> Manager.New / createRemote -> environment physical Target.Route
    -> host WorkflowStart -> invocation.Request.TaskRoute.Environment
    -> runner's run-wide effects.WithEnvironment -> environmentMessaging peer routes

Required path:

    UI selection -> manager -> registered conversation input/Prepare namespace check
    -> Conversation's declared environment -> selected River request/retained owner
    -> native taskdaemon -> same-boundary authoritative results and observations

Primary files: `internal/workflows/conversation/conversation.go`,
`internal/conversation/{manager,remote,requests}.go`,
`internal/host/{host,environments,routes}.go`, `internal/environments/*.go` where
preparation/selection currently chooses peer routes, `internal/invocation`,
`internal/runner/runner.go`, `registry.go`, `workflow`, `internal/generate`,
`backend/messaging.go`, `messaging_registration.go`, `plugins/messaging-river`,
`internal/effects`, `internal/taskdaemon`, and placement observation/UI owners.
The prerequisite adds these conversation/host/environment owning files to the
parent map; do not expand into an unrelated UI redesign or workspace manager.
For these added files, report measured baseline and added production lines before
large changes; an increase above 700 production lines in conversation/host/
environment integration (excluding the shared transport work already budgeted)
is a stop-and-report tripwire. Existing directories are valid live inputs; no
automatic source synchronization or managed layout.

Work items:

1. Inspect current source and produce a concise plan/file map; then implement.
2. Add logical input declarations/pre-effect namespace validation and conversation
   input propagation. Preserve intentional local behavior; invalid explicit remote
   selection must never run locally. No compatibility shim for a replaced route.
3. Implement addressed River requests, native-owner binding and participant bootstrap
   sufficient for this consumer, respecting the parent's registration/replay/recovery
   contracts. Preserve existing native executors; do not rewrite harnesses.
4. Migrate conversation preparation and follow-up/control to that path. Surface
   actual admitted placement in existing observation/UI models.
5. Add focused package/integration tests for these behaviors. Run affected checks,
   generators and builds. No proof-only programs or committed run output.
6. Report exact changes, checks, remaining work and any live behavior actually seen.
   A green test is not C1. Live two-machine/Docker/native work will be coordinated
   by this chat after inspecting the candidate; do not touch shared services now.

Contracts are inviolable: report underivable seam decisions or budget breaches;
do not silently amend the parent spec or weaken checks. Build only this scope.
Do not merge, publish releases, alter shared/qualification services, reset Desktop,
or create another coordinating chat/worktree. Do not commit while this parent is
writing task notes; report the finished source diff so the parent checkpoints it.
Keep this subtask's source compatible with the remaining distributed task; do not
claim the entire architecture gate or all final live claims are complete.
