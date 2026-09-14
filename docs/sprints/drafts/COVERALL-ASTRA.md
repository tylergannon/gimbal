## 1. Intent

Replace #157, #155, #152, #151, #147, and #13 with one change that charges tokens to the scope where each turn executes. Let Tyler expand a timeline from the whole run through scopes and task prompts to individual messages, seeing new input, cache reads, cache writes, output, and reasoning per model, with stated cost at turn level and above. Use the same observation snapshot, fold, and page live and after the run.

## 2. What is true today

- Placement belongs to the executing context, independently of session creation; root is `""`, ancestry is slash-delimited (`ephemeral/research/api/API.md:773-802`; `session.go:124-136`; `run.go:141-143,185-191`). The saved planner is created at root but runs in `without-plan.1` (`ephemeral/attest/issue125/run.jsonl:6,8-9`).
- #156 supplied five normalized counts, session totals, and final per-model turn accounting. `turn_ended` carries result, error, usage, duration in nanoseconds, and interruption; its usage is already the final answer, including an empty report (`usage.go:8-34,59-64`; `events.go:111-120`; `session.go:182-221`). Session totals accumulate step tokens and report **cost only**, so their tokens need not equal final turn-report tokens (`session.go:143-145,168-185`).
- The store folds workflow metadata and one native projection per turn; `run.usage[session]` replaces the previous cumulative total. Its scope query instead sums sessions by creation scope (`internal/observation/store.go:138-164,210-234`; `internal/observation/usage.go:50-85`). Exact timestamped lifecycle JSON already reaches the store (`event_persistence.go:38-56,112-118`; `internal/observation/store.go:25-32`).
- #146 supplied scope name/status/error/task/values/decisions, preserved cancellation, and recorded output-validation failure on the turn; the snapshot still has no turn records or intervals (`internal/observation/snapshot.go:67-87`; `internal/observation/store.go:125-164`; `session.go:212-221`).
- The page reads the separate scope query above flat scope/transcript lists; message tokens come from projections and session totals from `run.usage` (`web/src/routes/runs/[runID]/+page.svelte:4-13`; `web/src/lib/observation/RunViewer.svelte:55-78`; `web/src/lib/observation/SessionTimeline.svelte:17-32`; `web/src/lib/observation/MessageRow.svelte:41`). New projections have empty `info`; both ports only update an existing info record (`internal/observation/store.go:224-234`; `internal/sessionstate/reduce.go:84-88`; `web/src/lib/observation/index.ts:26,70-79`; `web/src/lib/sessionstate/index.ts:48`).
- Codex's `last` usage fills an open step, not a turn report; `TurnResult` supplies no usage (`codex/events.go:358-408`; `codex/codex.go:105-106,158-162`). In `ephemeral/attest/issue-149/logs/runs/20260912-205306.issue-149/`, resumed Codex input/output/cache-read are `4402/59/20224` in `sessions/codex.1/codex.1.jsonl:121` and `run.jsonl:24`; Claude's step cost is zero but its final turn cost is `$0.0198747` (`sessions/claude.1/claude.1.jsonl:34`; `run.jsonl:12`).

## 3. Decisions absorbed

- **#155:** Keep `run.usage` as the session-total contract and `run.sessions` as workflow session metadata; leave unused reducer `info` unseeded. This avoids duplicating a cumulative session across per-turn projections; it closes the issue by choosing the observation contract, not by claiming reducer info was populated (projection and consumer facts above).
- **#152:** Show stated cost on turns and aggregates, and tokens/model on individual assistant messages; remove the message dollar label. Claude's saved step/report distinction above makes turn cost the supported finest grain; do not allocate dollars proportionally or add prices.
- **#151:** Subtract cache writes as well as cache reads from Codex input, with a zero floor. The source investigation in #151's comment is accepted; replace the provisional assumption and expectation at `codex/events.go:559-577` and `codex/events_test.go:24,51`.
- **#147:** Render task metadata once within its task scope, suppressing the `task` value only when that metadata exists. Keep the stored value because Loop passes local scope text back to its planner (`loop.go:113-128`; `scope.go:153-164`).
- **#13:** Supersede its usage requirements with #156 and this issue; decline its remaining context-window, quota/reset, MCP, and full raw-payload retention requirements. None is needed for this inspection goal; existing native-reference sidecars are provenance, not a promise to retain full provider payloads (`internal/observation/snapshot.go:57-65`; `internal/observation/store.go:171-184`).

## 4. The change, in edit order

Assign each numbered edit to an implementer subagent. After each edit, a separate validator receives this issue, the source/diff, and executable checks, **never the implementer's notes**; it checks that step before the next begins. Keep helpers private, write the workflow inline, and leave `internal/sessionstate/` and `web/src/lib/sessionstate/` untouched.

1. **`codex/events.go`, `codex/events_test.go`:** Set uncached input to `max(0, inputTokens - cachedInputTokens - cacheWriteInputTokens)`; preserve the other normalization fields. Correct the comment and the existing `100/25/5` expectation from `75` to `70`; cover the zero floor and both raw-response and notification paths.

2. **`internal/observation/snapshot.go`, `store.go`, `usage.go`, `checkpoint.go`, their observation tests:** Add the following snapshot data. `turnInfo` is private; `Usage`, `SessionInfo`, and `Invocation` retain their existing shapes (`internal/observation/usage.go:13-37`; `internal/observation/snapshot.go:34-65`). No changes to `run.go`, `event_persistence.go`, root `usage.go`, or lifecycle events are needed: decode the small time/turn header from `Lifecycle.Record` in the observation package.

   ```go
   type RunInfo struct {
       ID       string                 `json:"id"`
       Name     string                 `json:"name"`
       Status   string                 `json:"status"`
       Error    string                 `json:"error,omitempty"`
       Sessions map[string]SessionInfo `json:"sessions"`
       Usage    map[string]Usage       `json:"usage"` // session cumulative
       Started  int64                  `json:"started,omitempty"`
       Ended    int64                  `json:"ended,omitempty"`
       Updated  int64                  `json:"updated,omitempty"`
   }
   type ScopeInfo struct {
       Name      string                     `json:"name"`
       Status    string                     `json:"status"`
       Error     string                     `json:"error,omitempty"`
       Task      json.RawMessage            `json:"task,omitempty"`
       Values    map[string]json.RawMessage `json:"values,omitempty"`
       Decisions []json.RawMessage          `json:"decisions,omitempty"`
       Began     int64                      `json:"began,omitempty"`
       Ended     int64                      `json:"ended,omitempty"`
   }
   type turnInfo struct {
       Scope       string           `json:"scope"`
       Session     string           `json:"session"`
       Prompt      string           `json:"prompt"`
       OutputType  string           `json:"outputType"`
       Result      string           `json:"result"` // recorded JSON source text
       Started     int64            `json:"started"`
       Ended       int64            `json:"ended,omitempty"`
       Duration    int64            `json:"duration"`
       Error       string           `json:"error,omitempty"`
       Interrupted bool             `json:"interrupted,omitempty"`
       Usage       map[string]Usage `json:"usage"` // per model
   }
   type RunSnapshot struct {
       Run         RunInfo               `json:"run"`
       Scopes      map[string]ScopeInfo  `json:"scopes"`
       Turns       map[string]turnInfo   `json:"turns"`
       Invocations map[string]Invocation `json:"invocations"`
   }
   ```

   **Fold rules, also binding the browser in edit 3:** All timestamps are Unix milliseconds; convert lifecycle RFC3339 times to milliseconds and duration from nanoseconds by integer division by `1_000_000`. Set run start/end from `run_started/run_ended`, scope begin/end from `scope_began/scope_ended`, and `run.updated` to the maximum timestamp of every accepted lifecycle/native frame, including text deltas (`event.created`). Preserve existing status/error/task/value/decision behavior. Insert turns by the full lifecycle turn ID on `turn_started`, retaining its execution scope, session, prompt, output type, and start; on `turn_ended`, set result/end/duration/error/interrupted and **replace the entire usage map** from `event.usage`, even `[]`, removing provisional model keys. Before that terminal replacement, add each `session.step.ended` usage to its turn under the configured session model; a `session.step.failed` contributes only if **both** cost and tokens are present and non-null; absent ended-step fields zero-fill. These predicates match `session.go:226-251`. Session usage events only replace `run.usage[placement.session]`; never add them to turns. Copy all new maps when detaching snapshots and default missing maps when loading; absent legacy times remain absent/zero, not invented.

3. **`web/src/lib/observation/index.ts`, `index.test.ts`; `internal/observation/usage.go`, `usage_test.go`; `web/src/routes/runs/[runID]/+page.svelte`, `usage.remote.go`; `web/observation_live_test.go`:** Mirror the snapshot/fold above, including whole-state replacement on reconnect. Keep session-total replacement in the observation layer; define the existing `Usage` TypeScript shape locally instead of importing the query-generated type (`web/src/lib/observation/index.ts:1-4`). Delete both Go `ScopeUsage` methods and their containment helper, the remote query, its page subscription/header, and its obsolete tests/helpers; retain tests of the session-total contract. Those query paths currently call `snapshot.Run.ScopeUsage` and cannot see sibling turn records (`web/src/routes/runs/[runID]/usage.remote.go:54,61,102`; `web/observation_live_test.go:68-151`). Regenerate bindings through `just build`, removing obsolete query-generated artifacts through generation, never hand-editing generated source (`justfile:4-9`). Preserve the snapshot/SSE path and connection-generation guards (`web/src/lib/observation/index.ts:47-65`).

4. **`web/src/lib/observation/index.ts`, `index.test.ts`, `RunViewer.svelte`, `SessionTimeline.svelte`, `MessageRow.svelte`:** Expose computed scope totals through a getter on the existing `RunObservation`, shared by the page and its unit tests; do not persist or cache them. Replace the flat lists with one expandable scope → turn → message tree, using a recursive snippet in `RunViewer` and the existing transcript components inside each turn. Delete the flat invocation list; no aggregate endpoint or new component is needed.

   **Sum rule:** In that single scope-rollup implementation, scope `s` contains a turn iff `s == ""`, `turn.scope == s`, or `turn.scope` starts with `s + "/"`. Add each contained turn's six usage fields once, separately by model; a scope's displayed total is the sum of its model entries, and the run is scope `""`. Thus parent = direct turns + immediate child totals; never add both descendants and their already-summed parents. Message numbers are the assistant row's own normalized tokens/model, not a source for re-summing final turns; session cumulative totals are labelled separately and never substituted for scope totals.

   Show five token cells and stated USD at scope/turn level, with expandable per-model entries and inspectable unrounded values. Show assistant model, five token cells, interval, and expandable existing message content; other message types remain inspectable without attributed token spend. Expanding a turn exposes its full prompt, output type, result and error/interruption; expanding a scope exposes task name/description/definition of done/validation, other values, decisions and errors. Apply #147's suppression only inside that task scope.

   Root label is the run name; other scope labels use the final key segment with ordinal. Order child scopes/direct turns by start, then full key; namespace row identity by kind plus full scope/turn/message identity. Preserve expansion across frames using reactive replacement, and propagate the observation revision through nested rendering. Use one axis from `run.started` to `run.ended`, or `run.updated` while open; scope/turn/message intervals use their own begin/end, with open intervals visibly marked and extending to that axis end. Missing times get no bar, zero durations get a point, and coordinate division is bounded away from zero. Root spans the run lifecycle, including cleanup: its body ends earlier (`scope.go:79-96`; `run.go:87-99`). For a legacy checkpoint without recorded turns/times, retain its transcript and session totals, label aggregate accounting/timing “not recorded,” and do not manufacture zero usage or replay logs; loading only reads the checkpoint (`internal/observation/registry.go:77-98`). Run **svelte-autofixer on every changed Svelte component**, including the route page.

5. **`ephemeral/attest/usage-by-scope/main.go`, its run artifacts and independent validator evidence:** Write and execute the single program below, then perform section 5. Keep only the ordinary program, recorded logs/checkpoint, screenshots and validator observations; no reconciliation script, replay harness or shared proof-fixture infrastructure.

## 5. What the validator must observe

Use one real run, with logged-in CLIs and exactly these configured models: `claude-haiku-4-5-20251001`, `gpt-5.6-luna`, `gemini-3.8-flash-low`. Set up `web.NewRuntime(ctx, projectDir, web.WithPort(8099))`, a fresh project directory, four separate scratch workdirs `a,b,c,d`, signal cancellation and a post-run server hold, following the setup at `ephemeral/attest/issue-149/main.go:38-60,94-118`. Print the run URL and return a nonzero process exit on runtime/run failure. The inline body is:

```go
preamble := strings.Repeat("A token budget includes new input, cached input, output and reasoning.\n", 160)
err := runtime.Run(ctx, "usage-proof", func(ctx context.Context) error {
    shared := gimble.NewSession(ctx, "shared", codex.New(), "gpt-5.6-luna", a)
    if _, err := shared.Generate[gimble.Text](ctx, "Define a token budget in one sentence, without tools."); err != nil { return err }
    if err := gimble.Scope(ctx, "research", func(ctx context.Context) error {
        _, err := shared.Generate[gimble.Text](ctx, "Name one useful budget measurement, without tools.")
        return err
    }); err != nil { return err }

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
    if err := gimble.Scope(ctx, "review", func(ctx context.Context) error {
        return gimble.Scope(ctx, "verdict", func(ctx context.Context) error {
            _, err := shared.Generate[gimble.Text](ctx, fmt.Sprintf("Compare these explanations in one sentence, without tools:\n%s\n%s", first, second))
            return err
        })
    }); err != nil { return err }

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

1. Run `go run ./ephemeral/attest/usage-by-scope`; open the printed `/runs/<runID>` before completion. Capture two successive live views showing an open interval advance during streaming and usage rise after a step; capture `compare.1/attempt.1` and `attempt.2` with overlapping bars. Compare their scope begin/end timestamps in the new `run.jsonl`; concurrency must actually be observed.
2. Expand `research.1`: `shared.1/turn.2` appears there with its exact `turn_started.prompt`, while `shared.1/turn.1` remains a direct root turn. In the new `run.jsonl`, identify and record the line numbers of those two starts/ends; compare all five counts and cost in each page turn's model entries to that end line's `event.usage`. The child turn contributes to the inclusive root, once, but is not rendered as a direct root turn.
3. After completion, manually total **every** `turn_ended.usage` entry in the new `run.jsonl`, grouped by execution scope and model. Record the actual `run.jsonl:L` operands and compare every scope/turn model row and root total with that arithmetic; also check each parent's direct turns plus immediate children. Token counts must match exactly, USD within `1e-9`.
4. Expand a Claude turn and one assistant message. Find its `session.step.ended` line by turn ID and assistant message ID in the recursively located `sessions/**/*.jsonl`; compare its five message counts, model and interval to the step start/end records. Compare the turn's dollars to its `run.jsonl` end line, and the separately labelled session cumulative total to the session log's **last** `session.usage.updated` line, not to assumed turn-token equality. Record both Claude turns' cache-read/write/new-input values and observe nonzero cache use; inspect the resumed Codex step and its matching final turn, without inventing a missing-step fallback.
5. Expand `review.1/verdict.1` and `dispatch.1/task.1`, then the worker turn and its message. Compare task metadata with that scope's `scope_began.task` line and prompt with its `turn_started` line; screenshot one task description, the separately recorded worker result, the full prompt and the message's model/token cells. Click open and closed while live updates arrive: the selected expansion persists and content remains readable. The finished screenshot must also show all three model totals, root interval, nested scope placement and overlapping group intervals; root endpoints match `run_started/run_ended`, not `complete`.
6. Reload the finished page, then copy **only** `observation.json` to `<fresh>/.gimble/runs/<runID>/observation.json`. From `<fresh>`, run the absolute path to the built `bin/gimble --port 8100` and open `http://127.0.0.1:8100/runs/<runID>`; the command serves `.gimble` and the registry requires that layout (`cmd/main.go:33-58`; `internal/observation/registry.go:103-108`). Independently repeat the same tree expansions and numeric comparisons, including session totals, task, prompt, message and intervals; capture the finished view. Record configured/resolved model IDs, run ID, URLs, log line operands and observations in the validator's own evidence, without reading an implementer report.

## 6. Unit tests — gates, not proof

- Codex: corrected cache-write subtraction, zero floor, raw and `last` paths, retaining existing step/turn boundary coverage.
- Observation Go/TS: execution placement; ended-step addition; failed-step missing/null/full fields; unconditional empty/different-model/different-token final replacement; session-total replacement independent of turn totals; unseeded projections; exact times including text-delta clock advance; result/error/interruption; detached checkpoint and initial subscription snapshot; replacement snapshot removing previous state and rejecting stale frames.
- Browser rollup in the existing observation test file: root/direct/child/grandchild arithmetic, `attempt.1` versus `attempt.10`, two models, no parent/descendant double count, absent legacy turns/times. Keep the fold/sum tests small and synthetic; no cross-language fixture runner.
- Gates in order: svelte-autofixer for each changed component; `just build`; `go test ./...`; `go vet ./...`; `cd web && pnpm test`; `cd web && pnpm run check`. Each must pass; the numbered live observations are the proof.

## 7. Open decisions for Tyler

None. No new exported type or function is needed; extend the existing snapshot fields and keep new helper declarations private.
