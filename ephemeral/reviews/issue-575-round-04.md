# Adversarial review: issue 575, round 04

Date: 2026-10-15 (local)
Reviewer: Claude (read-only; this file is the only write)

## Review target

- Issue: `ephemeral/issues/575.md` (workflow observation: project-bound launcher, Runs list, agent focus as chat, steering, whole-run cancel, questions across tabs, reconnect, native window sizes, completion evidence).
- Branch `codex/issue-575-workflow-observation`, HEAD `bc5455c5` (includes merge of `main` at `0140f2ad`, v0.13.34). Working tree has one uncommitted change, another session's edit to `ephemeral/worklog/issue-575-workflow-observation.md`; it is notes, not product.
- Proof: `/tmp/gimbal-575-pr-body.md` (PR #577, open draft at `bc5455c5`).
- Authoritative sources, same as rounds 01-03: the issue, `AGENTS.md`, `docs/definition-of-done.md`, `docs/web-app.md`, `ephemeral/research/svelte-idioms/dos-and-donts.md`.
- Caller constraints honored: review file path, read-only elsewhere. No scope narrowing was requested; nothing ignored.

## Evidence inspected

Product delta `0140f2ad..bc5455c5` (the true branch delta; local `main` is stale) read in full: `internal/observation/{rows,store}.go`, `web/runview/list.go`, `web/src/routes/workflow.remote.{go,ts}` (plus the skgo link copy under `internal/skgo/links/`), `routes/types.ts`, alert-dialog/popover primitives, `ConversationMessage.svelte`, `ConversationTranscript.svelte`, `observation/index.ts`, `CancelGuard`, `DetailPane`, `MessageRow`, `RunQuestions`, `RunView`, `RunsList`, `Topbar`, `SessionDetail`, `SessionPage`, `StatusLine`, `SteerBox`, `UsageTable`, `WorkflowRun`, `ProjectRuns`, `ProjectWorkspace`, `WorkspaceShell`, `WorkflowGuide`, `WorkflowLauncher`, and the `workflows/[workflow]` route. Test delta read in full; every test named in the PR body exists.

Commands run at `bc5455c5`:

| Check | Result |
| --- | --- |
| `go test -count=1 ./internal/observation ./web/runview ./web/` | ok |
| `vp run test` (vitest) | 41 files, 185 tests passed (matches PR body) |
| `vp run check` (svelte-check) | 5453 files, 0 errors, 0 warnings |
| `gh run view 38812486987` | success at `bc5455c5` (job is named "Automatic release"; PR body calls it CI) |
| `gh pr view 577` | OPEN, draft, head `bc5455c5` |
| `git merge-tree` against `origin/main` | clean (checked at `bbe70a8d`; origin/main has since moved to `c6f7c9f4`, not re-checked) |
| 12 evidence URLs in the PR body | all HTTP 200 |
| Provenance: `git diff 32a73580..bc5455c5` on WorkflowLauncher/WorkflowGuide/WorkflowRun/SessionPage/ConversationMessage | empty, so reuse of 32a73580 evidence for those surfaces is sound |
| Provenance: `git diff 1d29893d..bc5455c5` on RunPanel/RunView | empty, so reuse of the reconnect evidence is sound |

Six evidence screenshots viewed: the opaque "Cancel the whole run?" dialog over the Runs row (no animation), the cancelled record with four "Claude API request failed and will be retried · overloaded · HTTP 529" rows under collapsed "Retry details", the live retry run with "code-review is working… · 0m 15s", the interview run with "Steer landed." and "Read at the next model call. Dropped if the turn ends first.", the launched run on the 800×532 minimum window with a running `sleep 20` tool and "Steer landed.", and the cancelled run showing cost "Not reported" with nonzero tokens plus the red "context canceled: gimbal: was killed by person: cancelled from the run workspace" line. The PR body names the final revision for every claim and states which claims are reused from which commit.

## Round 03 material findings, re-checked

| Round 03 finding | Status at bc5455c5 |
| --- | --- |
| No window-size evidence | Resolved. PR body reports 1000×600 default and 800×532 clamped minimum measured via authorized AppleScript sizing at 32a73580; the minimum-window screenshot shows the Focus layout intact at that size. |
| `$0.00` shown for an unstated cost | Resolved. `statedCostText` in `web/src/lib/observation/index.ts:448` prints "Not reported" for non-positive, "<$0.01" for small nonzero; Go `formatCost` in `web/runview/list.go` matches. Screenshot shows "Not reported" beside nonzero tokens. `TestSmallNonzeroCostNeverLooksFree` and the small-cost vitest cover it. |
| Provenance blurred | Resolved. Every evidence item names its source commit, and the unchanged-file diffs above confirm the reuse is legitimate. |

## Criteria walk (issue 575)

All acceptance criteria are implemented and either tested, evidenced, or both: launcher bound to the project with guide/structure tab retained (tabs stay mounted hidden in `WorkspaceShell`, so drafts survive the guide); Runs list with Live/Needs you/Ended/All, revealed search, summaries, elapsed, cost, sessions; incremental chat rendering with per-message revisions and implicit settlement; retry causes, message errors, shell, compaction; Return/Arrow steering keyed to the running session with landed/dropped feedback and route explanations; whole-run cancel only from row/header with confirmation and inactive-view teardown; questions on both tabs; reconnect; native window sizes; incremental-event and inactive-view regression tests present.

## Findings

No critical findings and no issues. The nitpicks below are the most relevant five.

1. **Nitpick, documentation drift.** `docs/web-app.md:89` (F1 "Filter by status and workflow; search by id") and `docs/web-app.md:93` (F5 "Home of the steer box") describe the pre-575 Runs list and session card. The branch moves filters to Live/Needs you/Ended/All with a revealed search and moves the steer box into the focus chat, but does not update the feature table. `AGENTS.md` names this file as the record of UI decisions, so it now disagrees with what shipped. Impact: the next reader of the design record is misled; no runtime effect.

2. **Nitpick, Svelte idiom.** `web/src/lib/conversation/ConversationTranscript.svelte:10` uses `{@const stopped = requests.find(…)}` inside the each block. `dos-and-donts.md` §29 lists `{@const}` as an anti-pattern to replace with a derived in script. Impact: style only; the value is recomputed per row per render, which is cheap here.

3. **Nitpick, dynamic import for `tick`.** `web/src/lib/run/RunsList.svelte:49` does `const { tick } = await import('svelte')` inside the search toggle handler, while `WorkflowGuide.svelte:2` imports `tick` statically. The dynamic form adds an async chunk boundary for a symbol already in the bundle. Impact: none observable; inconsistency.

4. **Nitpick, payload shape.** `web/src/routes/workflow.remote.go:42` sets `SourceJSON` on every `WorkflowDetails`, and `workflowCatalog` (line 53-69) returns one per workflow, so the launcher popover downloads every workflow's full source page although `WorkflowLauncher.svelte` never reads `source_json` (only `WorkflowGuide.svelte:12`, via the separate `workflowGuide` query, does). Impact: bytes over the wire proportional to catalog size; not a correctness problem.

5. **Nitpick, unauthorized handling.** `web/src/lib/workspace/WorkflowGuide.svelte:20` and `WorkflowLauncher.svelte:18` call `refresh()` on retry without `navigateOnUnauthorized`, while `SessionPage.svelte:78` and `RunsList.svelte:40` route the same failure through it. Impact: an expired session on the guide or launcher retry shows a generic error instead of the sign-in redirect.

Observed but not ranked: `SessionRow` field ordering in `internal/observation/rows.go` places the new fields ahead of `Run`/`ID` (cosmetic); non-assistant rows in `SessionPage` are re-copied on every global revision (bounded by row count, no visible cost); a run whose task environment is absent from inventory reads "This route can't be steered." (accurate, if terse).

## Outcome

only nitpicks remain
