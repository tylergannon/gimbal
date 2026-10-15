# Implemented local desktop worker: current evidence

[Proposal](../../proposal.md) · [Placement decision](../../topics/execution-placement.md) · [Earlier source research](assessment.md)

**The fresh-session native approval question is resolved for the tested, persistently approved Chrome path.** PR #440 merged the local experimental worker at `1c3eb84c94c5cce42a7f78018ca51e285e789dc1`. An ordinary desktop-launched MCP adapter accepted external requests while its owning agent was idle, created two distinct desktop tasks, and both used native macOS Chrome capture/input without an observed per-run approval. This is the implemented slice; the rest of the distributed backend remains proposed.

## What the observations establish

| Observation | Evidence and boundary |
| --- | --- |
| Two fresh native tasks | Threads `01a12342-1904-7e70-b349-ad958f85b2e0` and `01a12343-d9cf-7f23-a5d3-a1e545a6415c`, using `gpt-5.6-luna`, bound actual `com.google.Chrome` via native `cua.getApp` and submitted different fixture values. They were new sessions, not two turns sharing a temporary permission. |
| Actual native input | The localhost fixture recorded `FRESH_NATIVE_439` and `FRESH_NATIVE_439_SECOND` after native input and clicking; native screenshots showed the resulting values. Browser-tab automation was not substituted for the native check. |
| No observed per-run approval | Chrome already had legitimate persistent app approval. Neither interval contained a human approval response or observed approval wait in the scoped runtime evidence. The native metadata also marks `codex/computerUseChrome: true`; this establishes the tested Chrome path, not a promise that other apps/actions can never ask. |
| Active message and cancellation | Task `01a12346-d751-7bf0-b703-1f702821ecdc` wrote the requested marker while its original foreground process remained active. Archive-and-stop then produced an interrupted turn, a missing process, and a stopped heartbeat. It does not prove turn-only interrupt or arbitrary detached-child cleanup. |
| Admission and ownership behavior | The worker checks declared app approvals before creation, persists request identity before calling the desktop, refuses blind replay of uncertain creation, and restricts reads/messages/cancel to recorded task IDs. A lost creation reply can leave actual work started with no recorded native ID: the local request identity is not passed to the provider as an idempotency key. These contracts do not provide full Gimbal run ownership. |
| Unexpected approval handling | On `read`, a reported approval wait triggers an archive-and-stop attempt and an explicit failure with cancellation confirmation. This has source/unit evidence; deliberately causing a live consent prompt was unnecessary. Callers must continue reading. |

The native actions occurred October 9 in the user's timezone (the raw records use October 10 UTC). Sources: [first native fixture result](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/build/first-native-record.json), [second result](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/build/fresh-native-record.json), [first session source locations](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/codex/approval-review/fresh-native-independent-evidence.json), [second session source locations](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/codex/approval-review/second-native-independent-evidence.json), [active cancellation evidence](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/codex/approval-review/active-cancel-independent-evidence.json), and [independent approval-contract review](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/codex/approval-contract-review.md). Raw material and fixtures stay outside Git.

## What exists in the package

The [README](../../../../../plugins/codex-desktop/README.md) defines the local start/read/message/cancel surface, explicit one-time setup, private bridge dependency, and polling obligation. [Worker logic](../../../../../plugins/codex-desktop/lib.mjs) maintains one owner and a request-to-thread ledger. The [MCP server](../../../../../plugins/codex-desktop/server.mjs) receives genuine binding metadata and exposes a private local socket. The package's twenty tests passed during implementation and independent review; they establish their scoped protocol contracts, not native permissions by themselves.

The installed executable was exercised via ordinary MCP registration in a trusted project. This is distinct from testing marketplace package discovery, installation, and reload. The owner must remain loaded; no agent turn is needed merely to receive external work. The ledger is durable, but is not automatic startup or recovery. Current owner equality rejects binding a replacement owner; a held-lock launch bootstrap and the proposed [fenced handover](../../topics/execution-placement.md#recover-the-desktop-owner-without-losing-its-tasks) are future backend work.

## Remaining architecture obligations

Before depending on a non-Chrome app, demonstrate a fresh task using that actual app with saved permission; Chrome has a distinct native surface in the observed metadata. Worker protocol changes require `npm --prefix plugins/codex-desktop test` separately because current `just test` does not include the package.

Before first work, the backend must allocate a native task without a turn, durably bind its identity, and then dispatch under stop intent, or use recoverable provider-idempotent creation. This required native-admission route is not exposed by the tested ordinary MCP call and remains unresolved. The backend/caller must retain reservations for the current worker's unknown starts (the worker does not enforce desktop reservations); it cannot claim cancellation until the native identity and cessation are resolved.

The backend must associate run/task/attempt with worker owner, request ID, and native thread; reserve the actual desktop; transport remote requests; and retain control responsibility through worker unavailability. A lost owner connection does not prove its created tasks stopped. Run cancellation addresses owned tasks, never the shared desktop or its owner.

The worker's `read` returns a bounded recent task view, not a complete resumable event history. It cannot become the authoritative Gimbal journal by relabeling its output. The backend must record the required lifecycle/agent observations through an integration that satisfies the normal runtime contract. Likewise `message` does not promise a landed steer, and archive-and-stop is not a turn-only interrupt. A full `HarnessAdapter` needs separate implementation and validation of the semantics the workflow uses.

Keep the private bridge isolated in this adapter and recheck its small live acceptance scenario after updates. The demonstrated local route is sufficient to proceed with backend design; it is not a public OpenAI API, remote Linux/macOS workflow proof, Claude integration, or a general unattended capability guarantee.
