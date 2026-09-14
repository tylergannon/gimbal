# Token usage, time, and cost by scope: one tree of the run from the log, live and finished

URL: https://github.com/tylergannon/gimble/issues/173
State: open
Updated: 2026-09-13T17:35:16Z

Replaces #157, #155, #151, #147, and #13 (all closed into this). #169 is edit zero. #172 supplies the price table. PR #158 delivers edit 1 (#151). #152 stays open: no harness reports a per-call cost, and the proxy below is not one.

This is the merge of the two proposals on branch `claude/token-usage-by-scope-50a907` (`docs/sprints/drafts/COVERALL-CLAUDE.md` and `COVERALL-ASTRA.md`), as named in the #157 status comment: Claude's turn record on the existing `Invocation`; Astra's record decoding, live axis, root span, result field, and proof program. Where they differed beyond that, the choice is stated inline. The findings that led here are in `ephemeral/research/token-usage-coverall/FINDINGS.md` on branch `claude/token-usage-coverall-12e8d5`.

## Intent

Every turn's usage is charged to the scope it ran in. Every scope shows the sum of what ran inside it, per model: new input, cache read, cache write, output, reasoning, wall time, and a priced cost. The run page draws one tree of scopes, turns, and messages on the run's wall clock, from the same fold live and after the run. The adapters compute nothing new; the store records what the run log already says.

## What is true today (main at `ad6f2f6`)

- A turn's records carry the scope it ran in, not the scope that created its session: `session.go` uses the calling ctx's scope key for `turn_started`, native events, and `turn_ended`; `run.go` puts it in `Placement.Scope`. `session_created` is recorded under the creating scope. Seen in `ephemeral/attest/issue125/run.jsonl`: `planner.1` created at `""`, its turn placed in `without-plan.1`.
- `turn_ended` already carries the turn's final usage per model (`[]ModelUsage`), result, error, interrupted, and duration in ns. The runtime builds it as the harness's report when the adapter returned one (Claude), otherwise the turn's step usage under the session's configured model (Codex, Antigravity return none). An empty report is a report: `session.go` replaces the fallback whenever `result.Usage != nil`.
- Every lifecycle record reaches the store as JSON with its `time` (`event_persistence.go`, `Store.Lifecycle`'s `Record`). Native events carry `created` in Unix ms.
- The store keeps one `invocation` per turn keyed by turn id, created on the turn's first native event, published as `Invocation{Scope, Session, Turn, Snapshot, Provenance}`. The browser mirrors it in `invocations`.
- `RunInfo.Usage[session]` is each session's running total from `session.usage.updated`, read by the session card. It is step tokens plus the report's stated cost only, so its tokens need not equal the sum of turn reports.
- `RunInfo.ScopeUsage` sums those session totals by the session's *creating* scope. Served by the `scopeUsage` live query (`web/src/routes/runs/[runID]/usage.remote.go`), consumed only by the run header line, tested by `internal/observation/usage_test.go` and `web/observation_live_test.go`, both feeding only `session.usage.updated`. This is the wrong key: in the sprint workflow the planner and validator are created at the root and generate inside laps, so laps show nothing.
- `ScopeInfo` (since #146) has name, status, error, task, values, decisions; no times. The page lists scopes, then invocations, flat.
- The ported reducers never create a session's `info` record, so their `session.usage.updated` case is a no-op. Nothing reads reducer info.
- Codex `normalizeUsage` subtracts cache read from input and not cache write; PR #158 fixes it.
- Every step event carries `cost: 0` on all three harnesses. Claude states cost once per turn.
- Saved run for arithmetic: `ephemeral/attest/issue-149/logs/runs/20260912-205306.issue-149/` (three sessions in three scopes, two turns each; Claude reports under `claude-haiku-4-5-20251001` while the session was created as `haiku`).

## Decisions absorbed

- **#155.** Closed by decision. The observation store is the contract: session totals in `RunInfo.Usage`, turn totals on the invocation, scope totals are sums of invocations. Reducer `info` stays unseeded; the ports under `internal/sessionstate/` and `web/src/lib/sessionstate/` are not edited.
- **#151.** PR #158: `input = max(0, inputTokens − cachedInputTokens − cacheWriteInputTokens)`; the existing test's 75 becomes 70. Merge it first.
- **#147.** The tree shows a task scope's task once, from `ScopeInfo.Task`, and omits the value keyed `task` when the scope has a task. The runtime keeps recording the value because `ScopeText` and the planner's previous-task record read it.
- **#13.** Superseded by #156 and this. Context-window, quota windows, MCP status, and raw-payload retention are declined, not built. The quota research is under `ephemeral/research/quota-telemetry/` for later.
- **#152.** Not absorbed. A message row shows its model and five token cells and no dollar cell; cost is real at turn level and above. No proportional attribution.
- **#169.** Edit zero. After it, a finished run is reduced from `run.jsonl` and `sessions/*/*.jsonl` through the same `Store.Lifecycle` and `Store.Event`, so every past run gets this fold, and there is no checkpoint to default, label, or copy.

## The change, in edit order

Each edit compiles and its tests pass before the next. Helpers stay private; no new exported name in the root package; `internal/sessionstate/` and `web/src/lib/sessionstate/` untouched. Line references are to `ad6f2f6`.

### 0. #169

Serve finished runs from their logs. Delete `checkpoint.go`, the write at `Store.Close`, `loadCheckpoint`, and `observation.json`.

### 1. PR #158

Codex cache write. Merge as is.

### 2. Snapshot and fold, Go (`internal/observation/snapshot.go`, `store.go`, `usage.go`, tests)

`ScopeInfo` gains `Began, Ended int64` (Unix ms, `omitempty`). `RunInfo` gains `Started, Ended, Updated int64` (Unix ms, `omitempty`). `Invocation` gains, before `Snapshot`:

```go
Prompt      string           `json:"prompt,omitempty"`
OutputType  string           `json:"outputType,omitempty"`
Result      string           `json:"result,omitempty"` // the recorded JSON text
Started     int64            `json:"started,omitempty"`
Ended       int64            `json:"ended,omitempty"`
Duration    int64            `json:"duration,omitempty"` // ms
Error       string           `json:"error,omitempty"`
Interrupted bool             `json:"interrupted,omitempty"`
Usage       map[string]Usage `json:"usage"` // by model
```

No new map and no new type in the snapshot: the invocation is the turn record, keyed by turn id, with the placement the tree needs. `RunSnapshot` keeps its three fields.

**Record decoding.** `Store.Lifecycle` decodes what it needs from `Lifecycle.Record`, the JSON it already receives: the record's `time`, and for `turn_started` the prompt and output type, for `turn_ended` the result, error, interrupted, duration, and `usage` array. `run.go`, `event_persistence.go`, root `usage.go`, and the lifecycle event types do not change. The log record is the input on the live path and on the replay path from #169; one decoder serves both.

**Fold rules** (binding the browser in edit 4 too):

- Times are Unix ms; RFC 3339 record times convert once, duration ns divides by 1,000,000.
- `run_started` / `run_ended` set `Run.Started` / `Run.Ended`. `Run.Updated` is the max timestamp of every accepted lifecycle and native frame, including text deltas (`event.created`).
- `scope_began` / `scope_ended` set `Began` / `Ended` on the scope. Existing status, error, task, value, decision behavior is unchanged.
- `turn_started` creates the invocation (`invocationLocked(at)`, now before the first native event) and sets prompt, output type, started. A subscriber joining after `turn_started` sees the prompt in its first snapshot.
- `turn_ended` sets result, ended, duration, error, interrupted, and **replaces** `Usage` with the record's array keyed by model, including an empty array. The record is the runtime's final answer; the store does not decide again. The replacement may rename the key (Claude: `haiku` becomes `claude-haiku-4-5-20251001`) and add cost; totals at every level may change at turn end. That is correct, not a bug.
- Native events: `session.step.ended`, and `session.step.failed` only when it carries both `cost` and `tokens` non-null, add to `inv.Usage[run.Sessions[at.Session].Model]`. This is the live figure for a turn that has not ended, the same predicate `session.go` uses. `session.usage.updated` still replaces `RunInfo.Usage[session]` and is never added to a turn.
- `snapshotLocked` copies the new fields and maps.

**Delete** `RunInfo.ScopeUsage`, `Store.ScopeUsage`, the containment helper, `internal/observation/usage_test.go`'s scope cases, `web/src/routes/runs/[runID]/usage.remote.go`, `ScopeRef`, `web/observation_live_test.go`, and the header line in `+page.svelte`. Its only consumer is that line; the tree computes the number in the browser. `RunInfo.Usage` and `foldUsageLocked` stay: they are the session totals. Regenerate bindings through `just build`; if that removes the generated `types.ts`, the `Usage` TypeScript shape is declared in `web/src/lib/observation/index.ts`.

### 3. Prices (`#172`)

The embedded price table and its private lookup land with #172. This issue consumes them; if #172 is not merged first, edit 5 ships with every model at "no price" and the cost cell is filled when it lands.

### 4. Mirror and sums, browser (`web/src/lib/observation/index.ts`, `index.test.ts`)

- `ScopeInfo`, `RunInfo`, and `Invocation` gain the same fields as Go; `usage: Record<string, Usage>`.
- `foldLifecycle` applies the fold rules above from the record's JSON (`Date.parse(record.time)`), so it takes the invocations map as well as `run` and `scopes`. `apply` for a native event adds step usage under the session's configured model with the same predicate. `replace` copies everything; a replacement snapshot yields exactly the new state.
- **Sum rule**, one function on the existing `RunObservation`, computed on demand from the current invocations, no cache: scope `s` contains turn `t` iff `s == ""`, `t.scope == s`, or `t.scope` starts with `s + "/"`. For every scope key, the per-model sum of the contained turns' five token fields and stated cost. Parent equals direct turns plus immediate children; nothing is added twice. `attempt.1` does not contain `attempt.10`.
- **Duration.** A turn's duration is its record's `duration`; while open, `Run.Updated − started`. A scope's duration is `ended − began` wall time, or `Run.Updated − began` while open. The run's is `Ended − Started`. Wall time is not summed: two `attempt` children of a `Group` that overlap each take their own wall time, and their sum exceeds the parent's. The page shows that and does not compute a busy-time figure.
- **Priced cost.** For each per-model line, `input × price.input + cacheRead × price.cache_read + cacheWrite × price.cache_write + (output + reasoning) × price.output`, per million, from #172's table looked up by the model key. Reasoning is priced as output because models.dev has no reasoning price and providers bill it as output; say so on the page once. A model with no row shows **"no price"**. An aggregate (turn, scope, run) whose models all have prices shows the sum; one that contains any unpriced model shows "no price" and its per-model lines still show the priced ones. Never $0 for an unknown model. The harness's stated cost (Claude) is not replaced: the per-model line shows it beside the priced figure when it is nonzero, labelled stated. The table's date (its download date from #172) is shown once beside the cost header.

### 5. The page (`web/src/lib/observation/`)

`RunViewer.svelte` keeps the observation, the SSE connection, the revision, and the run header, and renders one tree in place of the scope list and the invocation list. Whether the rows are a recursive snippet or a component is the implementer's choice; the revision dependency is carried through recursive rendering, and expansion state is reactive (`SvelteSet` or replacement assignment), keyed by row kind plus full scope, turn, or message id. Deleted: the flat scope sections and the flat invocation sections.

Rows, three kinds:

- **Scope.** Label: the run name for `""`, otherwise the key's last segment. Bar from `began` to `ended`. Cells from the sum rule. Expanded: the task's name and description if the scope has a task, error if any, decisions, values (omitting `task` when a task exists), then child scopes and the turns placed directly in it, ordered by start then full key.
- **Turn.** Label: the session's name and the turn ordinal from the turn id. Bar from `started` to `ended`. Cells from `invocation.usage` summed across models, plus duration and priced cost. Expanded: the prompt (first line, rest in `<details>`), output type, result, error or interruption, then the turn's `SessionTimeline`, today's transcript with its message rows. The drill ends in the transcript itself; no anchors, no second copy.
- **Message.** The existing assistant row, showing model and five token cells and its own interval; the `$` label is removed (#152). Other message kinds stay inspectable without token cells.
- **By model.** Under every expanded scope or turn row, a `<details>` with one line per model: five token cells, priced cost or "no price", stated cost when nonzero.

Seven cells on scope and turn rows: input, cache read, cache write, output, reasoning, duration, cost. One axis from `Run.Started` to `Run.Ended`, or to `Run.Updated` while the run is open; an open interval draws to the axis end and is marked open. Missing times draw no bar; a zero duration draws a point; coordinate division is bounded away from zero. The root bar spans the run lifecycle including cleanup, so it ends after the root scope's body. Wide content scrolls in its own container. `svelte-autofixer` on every changed component, including the route page.

### 6. Checks

`just build`, `go test -count=1 ./...`, `go vet ./...`, `cd web && pnpm test`, `cd web && pnpm run check`.

## What the validator must observe on the proof run

Implementer subagents do the edits. A separate validator receives this issue, the diff, and the run directory, never the implementer's notes, and writes `ephemeral/attest/usage-by-scope/result.md` with the configured and resolved model ids, run id, port, and every number it read, with the `run.jsonl` line it came from. Numbers are read and added by hand; no reconcile script.

The proof program is `ephemeral/attest/usage-by-scope/main.go`, set up as `ephemeral/attest/issue-149/main.go` is: fresh project directory, `web.NewRuntime` on a loopback port, four scratch workdirs `a`..`d`, signal cancellation, server held open after the run, nonzero exit on failure, run URL printed. Cheap tier only: `claude-haiku-4-5-20251001`, `gpt-5.6-luna`, `gemini-3.8-flash-low`. The body:

```go
preamble := strings.Repeat("A token budget includes new input, cached input, output and reasoning.\n", 160)
err := runtime.Run(ctx, "usage-proof", func(ctx context.Context) error {
    // created at the root, used in a child scope: the placement case
    shared := gimble.NewSession(ctx, "shared", codex.New(), "gpt-5.6-luna", a)
    if _, err := shared.Generate[gimble.Text](ctx, "Define a token budget in one sentence, without tools."); err != nil { return err }
    if err := gimble.Scope(ctx, "research", func(ctx context.Context) error {
        _, err := shared.Generate[gimble.Text](ctx, "Name one useful budget measurement, without tools.")
        return err
    }); err != nil { return err }

    // concurrent siblings; the preamble makes Claude's cache read and write nonzero
    var first, second gimble.Text
    g := gimble.Group(ctx, "compare")
    g.Go("attempt", func(ctx context.Context) error {
        s := gimble.NewSession(ctx, "writer", claude.New(), "claude-haiku-4-5-20251001", b)
        if _, err := s.Generate[gimble.Text](ctx, preamble+"Explain this in 200 words without tools."); err != nil { return err }
        var err error
        first, err = s.Generate[gimble.Text](ctx, preamble+"Give a concrete example in 200 words without tools.")
        return err
    })
    g.Go("attempt", func(ctx context.Context) error {
        s := gimble.NewSession(ctx, "writer", agy.New(), "gemini-3.8-flash-low", c)
        var err error
        second, err = s.Generate[gimble.Text](ctx, "Explain token caching in 300 words without tools.")
        return err
    })
    if err := g.Wait(); err != nil { return err }

    // two deep
    if err := gimble.Scope(ctx, "review", func(ctx context.Context) error {
        return gimble.Scope(ctx, "verdict", func(ctx context.Context) error {
            _, err := shared.Generate[gimble.Text](ctx, fmt.Sprintf("Compare these explanations in one sentence, without tools:\n%s\n%s", first, second))
            return err
        })
    }); err != nil { return err }

    // one real Loop dispatch, so a task scope with a task exists
    dispatched := false
    l := gimble.Loop(ctx, "dispatch", "Dispatch one task to write a two-sentence explanation of cache reads versus new input; no file changes are needed.", shared)
    for taskCtx, task := range l.Tasks {
        dispatched = true
        worker := gimble.NewSession(taskCtx, "worker", codex.New(), "gpt-5.6-luna", d)
        answer, err := worker.Generate[gimble.Text](taskCtx, gimble.ScopeText(taskCtx)+"\nComplete the assignment without tools: "+task.Description)
        if err != nil { return err }
        if err := gimble.Set(taskCtx, "worker result", string(answer)); err != nil { return err }
        break // one actual dispatch; no claim that the planner certified completion
    }
    if err := l.Err(); err != nil { return err }
    if !dispatched { return fmt.Errorf("proof requires an actual Loop task dispatch") }
    return nil
})
```

(Adjust the `Set` call to whatever #159 makes it if that lands first.)

1. **Live.** With the page open before completion: two successive views during `compare.1` in which an open bar has advanced and a turn's cells have risen after a step, and `compare.1/attempt.1` and `attempt.2` overlapping. Compare their `scope_began`/`scope_ended` times in the new `run.jsonl`; overlap must be observed, not assumed.
2. **Placement.** Expanding `research.1` shows `shared.1/turn.2` with its exact `turn_started.prompt`; `shared.1/turn.1` is a direct root turn and `turn.2` is not. Record the line numbers of both starts and ends. The child turn counts once in the root's inclusive total.
3. **Turn equals its record.** For every `turn_ended` in `run.jsonl`: the turn row's by-model lines equal that record's `usage` array, model for model, five token cells and stated cost each. Write the table.
4. **Scope equals its parts.** For `research.1`, `compare.1`, each `attempt.N`, `review.1`, `review.1/verdict.1`, `dispatch.1`, `dispatch.1/task.1`, and the root: cells equal direct turns plus immediate children, added by hand from the table in 3. Tokens exact; USD within 1e-9.
5. **Root equals the run.** The root's per-model lines equal every `turn_ended` usage grouped by model. Claude's stated cost is nonzero; Codex and Antigravity stated cost is zero and does not appear as $0 (only the priced figure shows).
6. **Duration.** For each turn, the duration cell equals its `turn_ended.duration` / 1e6 to the ms. For each scope, `ended − began` from `run.jsonl`. `attempt.1` plus `attempt.2` exceeds `compare.1`, and the page shows all three. Root duration equals `run_ended − run_started` and is longer than the root scope's body.
7. **Price.** For one Claude turn and one Codex turn, recompute the priced cost by hand from the five token cells and #172's committed table rows; match to the cent. Antigravity's `gemini-3.8-flash-low` resolves to `gemini-3.8-flash` and is priced. Confirm no cell anywhere reads `$0`. If #172 has not landed, every cost cell reads "no price" and this step records that instead.
8. **Drill.** Expand `dispatch.1/task.1`: the task's name and description appear once, no `task` value line; then the worker's turn; then its prompt, result, and the transcript with its message rows; one assistant message's model, tokens, and interval match its `session.step.ended` line in `sessions/**/*.jsonl`. Click open and closed while live frames arrive: expansion persists.
9. **Session card.** Each session's total still equals its last `session.usage.updated` in its session log. It is labelled as the session total and not compared to turn-token sums.
10. **From the log** (#169's proof, repeated here). Restart the server on the same project directory; the finished run renders the same tree and numbers as 3 through 7. Then open the saved issue-149 run (`ephemeral/attest/issue-149/logs/runs/20260912-205306.issue-149/`, logs that predate this change): it renders a tree with times, turns, and per-scope sums, and its root per-model totals equal the sum of its six `turn_ended` records (Claude 20 / 130 / 220 / 66341 / 8269, stated $0.0249421; Codex 15210 / 112 / 0 / 30208 / 0; Antigravity 30006 / 101 / 0 / 0 / 0).

Screenshots for 1, 5, 6, 8, and 10 go beside `result.md`.

## Unit tests (gates, not proof)

- `internal/observation/store_test.go`: a session created at `""` with a turn placed in `lap.1/task.2` is charged there; live step sums are replaced by the turn report, including an empty report and a report that renames the model key; a failed step with only cost or only tokens adds nothing; scope times, run times, and turn fields survive `Snapshot`; a subscriber joining after `turn_started` sees the prompt; `Run.Updated` advances on a text delta.
- One Go test that reduces the saved issue-149 run directory through the #169 path and asserts the literal per-scope numbers in step 10. It is a test of the serve path, not a harness; no TS twin.
- `web/src/lib/observation/index.test.ts`: the same folds on the browser side; the sum rule on a three-level tree with a turn at each level; `attempt.1` versus `attempt.10`; two models in one scope stay separate; duration for open and closed rows; priced cost for a priced model, "no price" for an unknown one, and an aggregate that contains both; a replacement snapshot yields exactly the new state.

## Exit

Per `docs/definition-of-done.md`: when steps 1 through 10 are observed, exit and merge at 90 to 95 percent; presentation quirks are filed, not fixed in the branch.

## Open decisions for Tyler

None required to start. Two choices are stated above and easy to reverse: reasoning priced as output, and an aggregate with any unpriced model showing "no price" rather than a partial sum.

