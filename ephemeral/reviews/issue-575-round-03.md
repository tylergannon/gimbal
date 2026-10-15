# Adversarial review: issue 575, round 03

Date: 2026-10-15 (local). Reviewer: Claude Fable 5.1. Read-only apart from this file.

## Review target

- Completion evidence for issue 575: the PR description in `/tmp/gimbal-575-pr-body.md`, which is byte-identical (bar a trailing newline) to the body of draft PR #577 (`gh pr view 577`: OPEN, draft, head `b0d28f57`).
- Product source at `1d29893d` on `codex/issue-575-workflow-observation`; `af6633f6` and `b0d28f57` touch only `ephemeral/reviews/` and `ephemeral/worklog/`, confirmed with `git diff --stat 1d29893d HEAD`.
- Specification: `ephemeral/issues/575.md`. Repository instructions: `AGENTS.md`, `docs/definition-of-done.md`, `docs/web-app.md`.
- Prior rounds: `ephemeral/reviews/issue-575-round-{01,02}.md`. Round 2 reviewed the same product revision and left one material finding (incomplete native evidence) plus nitpicks.
- Caller constraints honoured: write only this artifact. The caller's note that later commits are "notes only" was verified rather than assumed. No scope narrowing was requested; none was ignored.

## Evidence inspected

- The nine evidence links in the PR body. All returned HTTP 200 and were viewed: final live focus, observation disconnected, observation reconnected, native-tab-dismissed cancel, settled cancelled record, active retry cause, ended search outcomes, question retained, historical-turn steer landed.
- Worklog `ephemeral/worklog/issue-575-workflow-observation.md`, which records which run was made on which source revision.
- Code paths the evidence depends on: `web/runview/list.go` (`formatCost`, `pendingInterviews`), `internal/observation/rows.go` (`Usage`), `web/src/lib/run/detail/{UsageTable,StatusLine,SteerBox}.svelte`, `web/src/lib/run/MessageRow.svelte`.

Commands run and what I saw:

| Command | Result |
| --- | --- |
| `cd web && vp run test` | 39 files, 169 tests passed (matches the PR body) |
| `go test -count=1 ./internal/observation ./web/runview ./web/` | all `ok` |
| `curl` each evidence URL | 9 × HTTP 200, 217–483 KB each |

### Criteria walked against the evidence

| Criterion | Native evidence at `1d29893d` | Gap |
| --- | --- | --- |
| 1 Launch, retained tab, same inventory, guide/structure | Launch of `01M50FH9…` and later runs; guide/structure tabs visible in screenshots | Draft retention across guide/structure was shown only on `01M50F1R…`, which the worklog places before the final source (finding 3) |
| 2 Runs filters, search, summaries, outcomes, elapsed, sessions, cost | Live/Ended/Needs you filters, exact-id search, Failed row with error summary, Cancelled and Completed peers, elapsed, session count | "All" filter not reported; focus cost shows `$0.00` for an unstated charge (finding 2) |
| 3 Live focus without reopening | FIRST/SECOND UPDATE, read-tool completion, retries arriving while mounted | none |
| 4 Retry causes, message errors, records retained | `overloaded · HTTP 529` with attempt/max/delay while Running + Live; completed turn retains prompt/context, output, usage | message-level error not shown natively (tested only; acceptable) |
| 5 Arrow/Return steer, landed/dropped, composer explanations, no stop in focus | Arrow and Return landed, including with Turn 1 selected; focus header shows only Close | dropped outcome and the three unavailable-composer explanations rest on tests only (nitpick) |
| 6 Row/header cancellation, inactive-view teardown | Dialog dismissed by Control-Tab, not revived; `…4Y4R3N` settled Cancelled, peer Completed | none |
| 7 Question across tabs, reconnect, window sizes | Question and draft survived tab change and was answered; proxy kill showed Running + Disconnected then Live on the same run | default and minimum native window sizes not demonstrated (finding 1) |

## Findings

### 1. Issue — criterion 7 is not met: no evidence at default or minimum native window size

The issue requires "The changed views remain usable at default and minimum native window sizes." The PR body states the sizing check is unknown and keeps the PR draft for that reason, and the worklog explicitly says the file named `minimum-focus.png` is not minimum-size proof. The report is honest, but the slice is not complete until that check is observed. All screenshots are at one large geometry (2434×1272 or 3840×2100 logical pixels). Nothing in the tree substitutes for it: no test exercises narrow layouts for `RunsList`, `SessionPage` or `WorkflowLauncher`. This is the same material gap round 2 left open, now narrowed to one criterion.

### 2. Issue — agent focus shows an unstated charge as `$0.00`

Criterion 2: "A nonzero charge never appears as free." The submitted "final live focus" screenshot shows Session total `cost $0.00` beside `cache read 157,747`, `output 437`, `cache write 5,402` for `claude-haiku-4-5`, i.e. a real, nonzero charge displayed as free. The cause is in code, not the screenshot:

- `internal/observation/rows.go:20` documents `Usage.StatedCost` as "0 when it stated none". Claude Code states cost only on the turn's result event, so mid-turn the tokens are known and the cost is not.
- `web/src/lib/run/detail/UsageTable.svelte:12` renders `stated_cost` as `<$0.01` when `0 < cost < 0.01`, otherwise `toFixed(2)`, so an unstated cost prints `$0.00`. `StatusLine.svelte:33` and `MessageRow.svelte:29` do the same.
- The Runs list already does it right: `web/runview/list.go:132-144` returns `—` when `observation.TotalCost` reports no stated cost, and `<$0.01` for small nonzero. The focus pane was changed in this slice (the `<$0.01` branch) and still prints zero for the unknown case.

Reproduction: open Focus on a running Claude Code turn after its first assistant message and before its result; the Session total reads `$0.00` with nonzero token rows. The same happens for any harness that never states cost. Fix shape: render `—` (or "not stated") when `stated_cost === 0` and any token count is nonzero, mirroring `formatCost`.

### 3. Issue — evidence provenance is blurred across source revisions

The PR body opens with "Product source tested: `1d29893d`" and then lists `01M50F1RVRXRTKZ714QRFW6ZNC.review` as the run that proved the launcher's goal/model draft survived the guide and structure tab. The worklog records that run before the round-1 fixes landed and says "Subsequent live proof will use the final source after review fixes." The launcher's draft retention (the `ContentStatic` rework) is exactly the area that changed. On final source the launcher was only shown being dismissed by Control-Tab, and launches succeeded, but draft survival across guide/structure was not re-observed. The issue says completion evidence "must name the final Gimbal revision and demonstrate the live transitions above"; a bullet proved on an earlier tree should say so or be repeated. Low effort to close: re-open the launcher on the final build, type a goal, visit guide and structure, return.

### 4. Nitpick — states covered by tests only

Not transitions, so not required natively, but worth one sentence each in the report so a reader knows they were not observed: the "dropped" steer outcome, the three unavailable-composer explanations (unsupported route, between turns, ended run), a message-level `error` record, and the "All" filter.

### 5. Nitpick — round-2 nitpicks remain at `1d29893d`

`docs/web-app.md` F1/F5/F9 still describe the old Runs filter and Focus flow; `workflow.remote.go:42` ships `source_json` per workflow that the launcher never reads; `ConversationTranscript.svelte:10` uses `{@const}` against `dos-and-donts.md` §29; dynamic `import('svelte')` for `tick` in `RunsList.svelte:49`. None blocks acceptance.

### Note on concurrent, uncommitted edits

While this review was being written, uncommitted edits appeared in the worktree from another session: `web/src/lib/run/detail/UsageTable.svelte` now renders `Not reported` when `stated_cost <= 0`, with a `test.each` case added to `SessionPage.svelte.test.ts`. That addresses finding 2 for the focus pane but not `StatusLine.svelte:33` or `MessageRow.svelte:29`, which still print `$0.00`. Those edits are outside the reviewed revision `1d29893d`, were not run here, and this review's findings stand against the commit the PR body names.

## Outcome

material findings remain
