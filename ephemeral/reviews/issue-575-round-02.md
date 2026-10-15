# Adversarial review: issue 575, round 02

Date: 2026-10-15 (local). Reviewer: Claude Fable 5.1. Read-only apart from this file.

## Review target

- Branch `codex/issue-575-workflow-observation`. The review began against the uncommitted working tree on `925fb2e1` (v0.13.31); partway through, that tree was committed unchanged as `1d29893d` ("feat: finish workspace workflow launch and live agent focus"). Everything below refers to `1d29893d`, which is the current HEAD with a clean working tree.
- Specification: `ephemeral/issues/575.md` ("Finish workflow launch, Runs and live agent focus inside the workspace").
- Repository instructions: `AGENTS.md`, `docs/definition-of-done.md`, `docs/web-app.md`, `ephemeral/research/svelte-idioms/dos-and-donts.md`.
- Prior round: `ephemeral/reviews/issue-575-round-01.md` (one requirement gap, three issues, five nitpicks).
- Caller constraints honoured: write only this artifact. No scope narrowing was requested; none was ignored.

## Evidence inspected

Full diff `925fb2e1..1d29893d` (50 files) plus the surrounding code it touches:

- Go: `internal/observation/{rows,store,store_test}.go` (session `workdir`/`effort`/`steers`, `cloneSession`), `web/runview/{list,list_test}.go` (`<$0.01`, pending questions only on running runs), `web/src/routes/{workflow,launch,steer,interview,control}.remote.go`, `web/src/lib/environments.remote.go`, `events.go` (`SessionCreated`, `Steer` lifecycle structs and their emitters in `session.go`/`loop.go`), generated `skgo_gen.go`/params/`workflow.remote.ts`/`types.ts`.
- Svelte/TS: `ConversationMessage.svelte` (new), `ConversationTranscript.svelte`, `SessionPage.svelte`, `SteerBox.svelte`, `SessionDetail.svelte`, `concepts/SessionScroll.svelte`, `RunView.svelte`, `Topbar.svelte`, `DetailPane.svelte`, `RunQuestions.svelte` (new), `RunsList.svelte`, `ProjectRuns.svelte`, `ProjectWorkspace.svelte`, `WorkflowLauncher.svelte`, `WorkflowGuide.svelte`, `WorkflowRun.svelte`, `RunPanel.svelte`, `observation/index.ts` (per-message revisions, implicit settlement invalidation), `sessionstate/index.ts` (which event fields the projection reads), `Pip.svelte`, the new `projects/[project]/workflows/[workflow]/+page.svelte`.
- Tests: `SessionPage.svelte.test.ts`, `RunsList.svelte.test.ts`, `DetailPane.svelte.test.ts`, `RunQuestions.svelte.test.ts`, `WorkflowLauncher.svelte.test.ts`, `WorkflowRun.svelte.test.ts`, `observation/index.test.ts`, `runs/[runID]/+page.svelte.test.ts`, `store_test.go`, `list_test.go`, `runs_page_test.go`, `launch_form_test.go`.
- Worklog `ephemeral/worklog/issue-575-workflow-observation.md`.

Commands run and what I saw:

| Command | Result |
| --- | --- |
| `go test -count=1 ./internal/observation ./web/runview ./web/` | all `ok` |
| `go vet ./web/... ./internal/observation/...` | clean |
| `cd web && vp run check` | 5447 files, 0 errors, 0 warnings |
| `cd web && vp run test` | 39 files, 169 tests passed |

### Round-1 findings, re-checked

| # | Round-1 finding | State at `1d29893d` |
| --- | --- | --- |
| 1 | Completion evidence absent | Partly addressed; see finding 1 below. |
| 2 | Every message deep-cloned on every event | Fixed. `SessionPage.svelte:31-35` builds a per-message revision map using `observation.messageRevision`; `ConversationTranscript.svelte:9` passes it per row; `observation/index.ts:323-335` invalidates the still-open assistant row on `session.step.started` and `session.execution.*` so settlement without an explicit message id still re-renders. Covered by the new `index.test.ts` case. |
| 3 | Steering fails closed under "Checking…" when the environment query errors | Fixed. `SessionPage.svelte:54` reads `environmentQuery.error` and shows "Could not check steering availability. Retry below." with a retry button at line 78. Covered by "steering availability failures explain the problem and offer retry". |
| 4 | Map detail pane says "Between turns" for an ended run | Fixed. `SessionDetail.svelte:208` passes an explicit reason for ended runs and ended turns; `SteerBox.svelte:11` falls back to "This turn has ended. Steering is closed." |
| 5a | `docs/web-app.md` stale | Not addressed (see nitpicks). |
| 5b | `picked` not reset on session change | Fixed by `{#key selectedSessionID}` around `SessionPage` in `RunView.svelte:480`. |
| 5c | `workflowCatalog` ships `source_json` per workflow unused by the launcher | Not addressed (see nitpicks). |
| 5d | Retry cause rendered as raw JSON | Fixed. `ConversationMessage.svelte:10-15` joins message/type/code/HTTP status. |
| 5e | `SessionRow` field order | Not addressed; cosmetic. |

### Requirements walked

- Launcher, guide and structure tabs: `WorkflowLauncher` uses the same `gimbal.DefaultRegistry` and `launchWorkflow` form remote as the catalog page; `ProjectWorkspace` owns tab identity for `/workflows/[name]` and the launcher is hidden/inert when its Runs view is inactive. Draft survival across guide navigation is tested.
- Runs: four filters, revealed search, factual summary from Go, elapsed, cost (`<$0.01` for small nonzero), session count, distinct Completed/Failed/Cancelled text, row menu + `AlertDialog` cancellation with whole-run wording, `cancelRun({run})` for the chosen run only, inactive-view teardown tested.
- Agent focus: chat rendering with collapsed tools, facts aside (run, role, scope, provider, host, worktree, model, effort, turn picker, usage, prompt/context, structured output, steers). No Stop/Cancel in focus (`Topbar.svelte:212,236` plus `RunView` closing `cancelOpen` on focus), tested in the run page test.
- Live rendering: incremental-event test applies real `session.*` frames to a mounted `RunObservation` and asserts deltas, tool progress/result and a retry cause appear; tail-follow vs. reader-in-place tested.
- Diagnostics: message `error`, `retry`, synthetic "Harness retry", shell and compaction records all have render paths and a test.
- Steering: `SteerBox` is a real `form` remote; session identity is the focused session even while viewing an older turn (tested); between-turns, ended-run, unsupported-route and unavailable-check states each have copy.
- Questions: `RunQuestions` renders per pending question above both Map and Focus with explicit run/question identity; pending is zero for non-running runs in both Go and TS.
- Reconnection: `RunPanel`/`RunView` connection handling is untouched by this change.

## Findings

### 1. Issue — completion evidence still does not meet the issue's bar

The issue says: "Completion evidence must name the final Gimbal revision and demonstrate the live transitions above in the native app." The worklog now records one native run (`01M50F1RVRXRTKZ714QRFW6ZNC.review`, Claude Code / `claude-haiku-4-5`) showing launcher → retained run tab, guide/structure survival, live prose and tool states in focus, a Return steer landing, and recorded cost. That covers criteria 1, 2 (partly) and 5 (partly). It does not cover, and the worklog itself says "Subsequent live proof will use the final source after review fixes":

- a controlled retry/backoff case showing its cause while the turn is still active (criterion 4);
- row/header cancellation with the authoritative record settling the outcome, and native tab navigation removing the launcher, row menu and cancellation dialog (criterion 6);
- a waiting question answered from the run across tab changes, and observation disconnect/reconnect reporting the gap without inventing failure (criterion 7);
- default and minimum native window sizes (criterion 7);
- the final revision named (`1d29893d`, or whatever is merged).

Under `AGENTS.md` ("Proof is what you saw") this is a report to be written in the chat or PR description, not code; the automated coverage is in place and the issue says it is necessary but not sufficient. This is the only remaining material gap.

### 2. Nitpick — `{@const}` in new code

`web/src/lib/conversation/ConversationTranscript.svelte:10` uses `{@const stopped = …}`. `dos-and-donts.md` §29 lists `{@const}` as legacy; the replacement is a declaration tag (`{const stopped = $derived(…)}`). Low, but it is new code written after the rule.

### 3. Nitpick — `docs/web-app.md` still describes the old views

Row F1 (line 89) still reads "Filter by status and workflow; search by id", F5/F9 still describe the Session card as the home of the steer box and the Focus card flow. `AGENTS.md` names this file as the record of UI decisions, and this change replaces the Runs list and agent focus. Carried over from round 1.

### 4. Nitpick — catalog payload carries every workflow's source page

`web/src/routes/workflow.remote.go:42` sets `SourceJSON` in `describeWorkflow`, which `workflowCatalog` calls for every registered workflow. `WorkflowLauncher` and `WorkflowRun` never read `source_json`; the guide tab fetches it again via `workflowGuide`. Opening the launcher downloads N full source pages for nothing. Carried over from round 1.

### 5. Nitpick — small leftovers

- `RunsList.svelte:49` does `const { tick } = await import('svelte')` inside the search toggle; `tick` is already statically importable and is imported statically elsewhere in the same tree.
- `ConversationMessage.svelte:35` renders a "Harness retry" cause through `cause({ type, status })`; when neither field is set the reader sees `{}` after the dot.
- `internal/observation/rows.go:100-102` places `Workdir`/`Effort`/`Steers` ahead of `Run`/`ID` in `SessionRow`; cosmetic.
- `SessionPage.svelte:49` queries `environmentSnapshot(project)` on every focus open to learn one boolean; the query runs `service.Snapshot` server-side. Acceptable now that errors are surfaced, but a steer-capability lookup keyed on the run's environment would be cheaper.

## Outcome

material findings remain
