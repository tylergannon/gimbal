# Adversarial review: issue 575, round 01

Date: 2026-10-15 (local). Reviewer: Claude Fable 5.1. Read-only apart from this file.

## Review target

- Branch `codex/issue-575-workflow-observation`, uncommitted working tree on top of `925fb2e1` (v0.13.31).
- Specification: `ephemeral/issues/575.md` ("Finish workflow launch, Runs and live agent focus inside the workspace").
- Repository instructions: `AGENTS.md`, `docs/definition-of-done.md`, `docs/web-app.md`.
- Caller constraints honoured: write only this artifact. No scope narrowing was requested; none was ignored.

## Evidence inspected

Working tree diff (32 modified, 13 untracked files) and surrounding code:

- Go: `internal/observation/{rows,store,store_test}.go`, `web/runview/{list,list_test}.go`, `web/src/routes/{workflow,launch,control,steer}.remote.go`, `internal/skgo/links/onzggl3sn52xizlt/workflow.remote.go`, generated `skgo_gen.go`/params, `events.go` (SessionCreated, Steer), `internal/environments/service.go`, `web/src/lib/environments.remote.go`.
- Svelte/TS: `RunsList.svelte`, `SessionPage.svelte`, `SteerBox.svelte`, `ConversationTranscript.svelte`, new `ConversationMessage.svelte`, `RunView.svelte`, `Topbar.svelte`, `DetailPane.svelte`, `SessionDetail.svelte`, `concepts/SessionScroll.svelte`, `WorkflowLauncher.svelte`, `WorkflowGuide.svelte`, `WorkflowRun.svelte`, `ProjectRuns.svelte`, `ProjectWorkspace.svelte`, `observation/index.ts` (revision bookkeeping, `messageRevision`), `sessionstate/index.ts` (retry/error folding), `Activity.svelte`, `SessionTimeline.svelte`, old versions of `SessionPage`/`SteerBox`/`RunsList` at HEAD.
- Tests: `SessionPage.svelte.test.ts`, `RunsList.svelte.test.ts`, `DetailPane.svelte.test.ts`, `WorkflowLauncher.svelte.test.ts`, `WorkflowRun.svelte.test.ts`, `runs/[runID]/+page.svelte.test.ts`, `store_test.go`, `list_test.go`.
- Worklog `ephemeral/worklog/issue-575-workflow-observation.md`.

Commands run and what I saw:

| Command | Result |
| --- | --- |
| `go test -count=1 ./internal/observation ./web/runview ./internal/skgo/... ./web/` | all `ok` |
| `go build ./...` | only the pre-existing `plugins/*` "function main is undeclared" wasm-target errors; unrelated to this change |
| `cd web && vp run check` | 5445 files, 0 errors, 0 warnings |
| `cd web && vp run test` | 38 files, 164 tests passed (the suite ran whole; the file filter was ignored by the runner) |

The incremental-event regression test the issue demanded exists (`SessionPage.svelte.test.ts`, "focus renders new messages, text deltas, tool progress/results and retry causes") and applies real `session.*` events to a mounted `RunObservation`. Inactive-view coverage exists for Runs (`RunsList.svelte.test.ts`) and the launcher (`WorkflowLauncher.svelte.test.ts`).

## Findings

### 1. Issue — completion evidence is not yet present

The issue states: "Completion evidence must name the final Gimbal revision and demonstrate the live transitions above in the native app." The worklog records that native UI access was only re-established after an unlock failure and that "native acceptance remains required". Nothing in the tree or the worklog reports a real inexpensive-model launch from the project launcher, a live transcript advancing in agent focus, a steer landing, a row cancellation, or a disconnect/reconnect, and no Gimbal revision is named. Under `AGENTS.md` ("Proof is what you saw") and the issue's own acceptance text, the slice is not done until that report exists. This is a requirement gap, not a code defect; the automated coverage above is necessary but the issue says it is not sufficient ("A populated screenshot or a successful steer alone does not establish live transcript correctness").

### 2. Issue — agent focus deep-clones every message on every observation event

`web/src/lib/conversation/ConversationMessage.svelte:8`:

```svelte
const row = $derived.by(() => { revision; return structuredClone($state.snapshot(message)); });
```

`revision` is the run-wide counter that `RunObservation.apply` bumps on every frame (`web/src/lib/observation/index.ts:328`) and `RunView` forwards on every event (`RunView.svelte:317-346`). `SessionPage.svelte:24` passes that global value through `ConversationTranscript` to every `ConversationMessage`, so one text delta re-runs `structuredClone` on every message in the turn, including completed tool calls carrying their full input and output payloads. The observation already keeps a per-message revision for exactly this purpose, `messageRevision(turn, id)` at `index.ts:367`, and the existing renderers use it (`run/detail/Activity.svelte:98`, `run/SessionTimeline.svelte:32`). The new path ignores it.

Impact: cost per streamed token is O(messages × payload size) in the chosen turn. A long agent turn with large `Read`/`Bash` results will stutter on every delta, which is the view the issue says must stay live and readable. Fix shape: have `SessionPage` compute `revision + observation.messageRevision(chosen.id, message.id)` per message (as the two existing callers do) and clone only when that per-message value changes.

### 3. Issue — steering in agent focus fails closed, with a misleading reason, when the environment query errors

`web/src/lib/run/detail/SessionPage.svelte:25-31`:

```ts
const environmentQuery = $derived(project ? environmentSnapshot(project) : undefined);
const inventory = $derived(environmentQuery?.current);
...
const canSteer = $derived(!project || target?.capabilities.steer === true);
const steerReason = $derived(!live ? '…' : !running ? '…' : project && !inventory ? 'Checking steering availability…' : !canSteer ? "This route can’t be steered." : '');
```

`environmentQuery.error` is never read. If `environmentSnapshot` rejects (it returns 503 whenever `host.OwnerFrom(ctx)` is nil, `web/src/lib/environments.remote.go:17-20`, and propagates any config or `inspectLocal` failure from `internal/environments/service.go`), `inventory` stays `undefined` forever, `canSteer` is false, and the composer is disabled under the text "Checking steering availability…" with no retry and no honest explanation. Before this change the focus composer did not depend on the environments service at all; the steer remote itself (`steer.remote.go`) already reports when a session cannot be reached. The issue asks that "unsupported routes and ended/between-turn states need an explanation rather than a misleading active composer"; a permanently "checking" composer on a local, steerable run is the mirror failure.

Reproduction: render `SessionPage` with `project` set and mock `environmentSnapshot` to return `{ current: undefined, error: new Error("boom") }` (the shape the real query produces on failure). The "Send steer" button stays disabled and the help text remains "Checking steering availability…" indefinitely. Secondary cost: every agent-focus open now calls `environmentSnapshot`, which runs `inspectLocal` over the project directory (`service.go:279`) just to learn a boolean.

### 4. Issue — the detail-pane composer explains an ended run as "Between turns"

`SteerBox.svelte:11` derives its explanation as `reason || (ended ? 'Between turns. Steering is available while this agent is working.' : …)`. Only `SessionPage` passes a `reason`. `SessionDetail.svelte:204-209` (the Map detail pane) and `concepts/SessionScroll.svelte:226` pass `ended={ended || !live}` with no reason, so a completed, failed or cancelled run, and any finished turn in a live run, shows a disabled composer captioned "Between turns. Steering is available while this agent is working." The replaced `SteerBox` said "This turn has ended. Steering is closed." The issue requires ended runs to get an explanation, and this one is false.

Reproduction: open a recorded run (`run.status !== 'running'`), Map, select an agent node so `DetailPane` renders `SessionDetail`; read the composer caption.

### 5. Nitpick — small leftovers

- `docs/web-app.md` still describes the Runs list as "Filter by status and workflow; search by id" (F1) and the old Focus card/stop flow; `AGENTS.md` names that file as the record of UI decisions, and this change replaces both views.
- `SessionPage.svelte:22` never resets `picked` when `sessionID` changes (the old page did). Pick turn 2 on agent A, focus agent B, return to A: the recorded-turn note reappears instead of the live turn.
- `workflow.remote.go:48` serialises `SourceJSON` for every workflow in `workflowCatalog`, but `WorkflowLauncher` never reads it; the guide tab fetches the same page again via `workflowGuide`. The catalog payload is a full source page per workflow for nothing.
- `ConversationMessage.svelte:27` renders a retry cause as `JSON.stringify(row.retry.error)`, so the reader sees `{"message": "…"}` rather than the message. Accessible, as required, but raw.
- `internal/observation/rows.go:102-105` puts the three new `SessionRow` fields ahead of `Run`/`ID`; cosmetic, but it reads as if they were the primary keys.

## Outcome

material findings remain
