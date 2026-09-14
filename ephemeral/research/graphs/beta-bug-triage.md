# Beta bug inclusion and parallel ownership

2026-09-14. Tyler challenged the release sequence's omission of small bug reports. The earlier sequence described the existing milestone rather than auditing all open issues; that was insufficient release planning.

The open backlog contained 27 issues, 16 outside Beta. Several omitted bugs were deliberately filed to keep a prior implementation branch focused ("presentation only" or "held until #173"). Those are branch-scope decisions, not good reasons to omit the defects from Beta. Include ordinary current-product bug fixes in Beta and route them to the existing owners, instead of treating the milestone as only major features.

## Added to Beta

| Issue | Work | Owner / timing |
| --- | --- | --- |
| #199 | Format float cost text | UI/#173; small presentation fix |
| #193 | Decode/render scope values as readable text/JSON | UI/#173; needed for useful command output inspection |
| #197 | Show steer/kill records with source, target, landed outcome and reason | Observation/UI #130/#173; uses records already present; coordinate reducers |
| #196 | Distinguish normal supervisor wind-down from actual errors | Runtime supervision #110/#120, then UI consumes the distinction |
| #195 | Preserve interrupted-turn usage when available; show unknown if unavailable | Adapter #135 plus usage #173; do not invent counts or silently report missing usage as zero |
| #194 | Clarify/preserve completed-work visibility in backlog/history | Loop/lifecycle owner; choose the smallest correct behavior before coding, not automatically a new task-state subsystem |
| #192 | Handle run directory visible before run.jsonl exists | Reader/lifecycle #126; focused regression, independent of UI design |
| #154 | Stop attestation reruns deleting previous logs | Independent small fix; do before relying on those proof reruns |
| #174 | Move the docs website out of the Go repository root | Independent repository cleanup; coordinate docs workflow and verify builds |
| #116 | Reconcile and fill lifecycle-log regression coverage | Runtime test owner; issue mentions obsolete events/API, so test the current equivalents or close already-covered portions on evidence rather than resurrecting old fields |

These are ten new milestone assignments, not ten completed fixes or guaranteed equal-effort jobs. #195 and #194 involve data availability/behavior decisions; scope those concretely at pickup. #199/#193/#154 are particularly straightforward candidates to finish early. Avoid parallel edits to the same observation reducers just because the reports are small.

## Outside Beta after this pass

- #105 and #150 are capability expansions, including explicitly deferred context storage/composition work, not small current-product bugs. Keep deferred unless separately requested.
- #164 explicitly tracks an upstream daemon limitation affecting a future archived-thread reuse feature; current Gimble refuses that path. It is not an easy local repair.
- #171 is a one-time review of old artifacts, not itself a requested code change. Read relevant notes when their active owner picks up the corresponding issue; do not turn it into a release architecture exercise.
- #108 describes a per-session app-server lifecycle replaced by the current shared-daemon implementation. A fresh fetch of main at 24347fbd1b8b9cf8357f0bb50243c44ccfa5fb75 confirms the shared connection. This is a stale-report audit candidate; do not dispatch an old architecture fix without verifying a current leak. It has not been closed by this pass.
- #152's potential per-message dollar attribution was already declined by #173, which explicitly removes message dollar cells and uses turn-level/priced totals. Keep that decision; do not invent per-message cost to close a perceived UI bug. Its bookkeeping should be reconciled with #173, not expanded into a feature.

## Process correction

The final Beta pass must compare **all open issues** with the milestone. Require a concrete reason for leaving a current-product bug out: outside scope, superseded, upstream blocked, or a knowingly accepted limitation. “Small,” “presentation only,” and “filed for later in another branch” are not release-deferral reasons by themselves. Use the existing parallel lanes for their small bugs; keep an independent cleanup lane for changes that do not contend for the same files.

This triage updates the sequence in beta-implementation-handoff.md. The original pre-triage inventory is retained in beta-triage-open-issues.json; the refreshed milestone snapshots and beta-issue-N.md files show the new assignments. No bug implementation, closure, or runtime proof was performed during this planning correction.
