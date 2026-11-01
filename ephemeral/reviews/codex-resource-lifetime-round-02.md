# Codex resource lifetime review — round 2

Target: `ba11f70d21069ee9bfbf82588a974f8142873d9e` on `codex/codex-resource-lifetime`, including the full change from `a3ee508b` and the repairs after round 1. Authoritative scope remains `ephemeral/worklog/codex-resource-lifetime.md`, `ephemeral/plans/codex-resource-lifetime-proof.md`, and repository instructions. Reassessed the transport terminal paths, session/operation accounting, admission and closure lock order, native turn recovery, worker final results, scope cleanup, supporting tests and contract updates.

Both round-1 findings are resolved. Session route admission now shares `s.mu` with the closing state, including ordinary RunTurn, stop confirmation, redial restoration and recovery readers. Terminal transports also reject parent/child route admission. This prevents an ended active session from restoring a route while another session keeps the connection alive, without closing the unrelated session's client. The new two-session regression establishes that ownership boundary directly.

Cancelled/timed-out final results now preserve their saved release-error details together with context error identity. The outer worker adapter returns the collected ordinary final result/error instead of replacing it with bare cancellation. Added worker-subprocess and outer-adapter regressions cover confirmed interruption followed by failed unsubscribe, while existing uncertainty and successful-final-result race cases remain covered. These repairs add no retained cleanup retry worker or cancellation-confirmation journal.

Independently ran the focused transport, duplicate-reply, session release, concurrent admission, active closure, two-session routing and worker lifetime/result tests under `GIMBAL_LIVE=0` with the race detector. Both packages passed: Codex 1.561s, agentexecutor 2.694s; output is local at `/tmp/gimbal-resource-review-round02.log`. This includes the new routing regression, the cancellation-plus-release-error path, and the existing normal/cancel/error/fork failure worker owner tests. No race-detector warning was emitted. No competing whole-repository test invocation was started during this round; the caller's serial full-suite run was still in progress when this review completed.

Findings: none.

The proof boundary remains limited to local test doubles, subprocess ownership and focused checks. No real Codex app-server, Gimbal daemon, provider/model session, GUI, existing-service lifecycle action or credentials change was performed. Actual native descendant/FD stabilization, historical crash causation, aggregate cross-session capacity and accepted-startup identity reconciliation remain unproved or deferred as explicitly documented; this review establishes neither product acceptance nor native cleanup completion. The bounded future real-workflow plan preserves unrelated clients and requires explained resource counts before those claims can be made.

Outcome: no findings.
