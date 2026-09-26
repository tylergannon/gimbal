# Owned Pi port: Claude draft

Draft for the coordinator's synthesis. Every upstream path below is under `/Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi` at `d6af72e1`; Go donor paths are under the sibling `sky-valley--pi` at `e6b56e72`. Line numbers were read from those trees today.

## 1. Overview and use cases

Gimbal gets a fifth `HarnessAdapter` whose agent loop, tools, session store, and compaction run inside the Gimbal process. Only tool subprocesses (the shell, `rg`, `fd`) are spawned. A workflow that binds a role to a Router model gets the same `Session.Generate`, `Steer`, `Fork`, and scope-driven `Close` it gets from Codex or Claude, with no Node or Bun child per agent.

Use cases the plan must serve, in the order they are proven:

1. A coding turn on `pi/diffusion/deepseek-4.1-flash` that reads a file, edits it, runs a command, and returns text.
2. A schema turn whose JSON passes Gimbal's validator and re-ask loop in `session.go:206-262` without native schema enforcement.
3. A second turn on the same session, a steer that lands mid-turn, a cancelled turn, and a fork whose parent and child continue independently.
4. Persist and resume across process restarts, and automatic compaction on a long session.
5. Later: more providers behind the same loop.

## 2. Which upstream is the reference, and what is out

The pinned tree carries two runtime stacks. The coding agent uses the established one: `packages/coding-agent/src` imports `@earendil-works/pi-agent-core` 53 times and touches `pi-agent-core/harness/*` and `experimental/pico3` only from `src/experimental/mini/`. So the reference is:

- `packages/agent/src/agent-loop.ts` (898 lines), `agent.ts` (613), `types.ts` (500).
- `packages/ai/src/types.ts`, `utils/transcript.ts`, `utils/event-stream.ts`, `utils/validation.ts`, `utils/estimate.ts`, `utils/retry.ts`, `utils/overflow.ts`, `models.ts` (cost and thinking-level helpers), `api/openai-completions.ts` (1,726 lines).
- `packages/coding-agent/src/core/agent-session.ts` (4,023 lines, ported as a subset), `session-manager.ts` (2,008), `compaction/*`, `tools/*`, `system-prompt.ts`, `skills.ts`, `resource-loader.ts` (context files and skills only), `model-config.ts`, `provider-composer.ts` (subset), `runtime-credentials.ts`, `messages.ts`, `utils/shell.ts`.

Deliberately not ported: `packages/agent/src/harness/**` (pico3, runtime/drive, session/jsonl), `packages/durable`, `packages/protocol`, `client`, `server`, `chord`, `tui`, `telemetry`; the extension runtime (`core/extensions/**`) and every `_extensionRunner.emit*` hook site; `modes/**` including RPC; `settings-manager.ts` (replaced by an explicit Go config struct that Gimbal fills); `cache-warmer.ts`, `bug-report*.ts`, `export-html/**`, `package-manager.ts`, `auth-storage.ts` and OAuth; `branch-summarization.ts`; `powershell.ts`; image reading and resizing (`read.ts` image branch, `utils/image-*.ts`).

Explicit scope recommendations rather than silent drops:

- **Provider breadth.** Milestone one ships `openai-completions` only. `anthropic-messages` and `openai-responses` get reserved package names and are deferred until the Router milestone is qualified. The loop contract does not change when they arrive.
- **Native session format.** Keep Pi's JSONL v3 tree format on disk under a Gimbal-owned directory. The store is being ported anyway, the format is what makes `test/fixtures/before-compaction.jsonl` and `large-session.jsonl` usable as fixtures, and it gives import of real Pi sessions for free. Not `~/.pi`; never the user's own sessions.
- **Project context and skills.** Port `loadProjectContextFiles` (`resource-loader.ts:175-213`, AGENTS.override.md > AGENTS.md > CLAUDE.md up the ancestor chain) and `loadSkills` (`skills.ts:409`). Treat the workdir as trusted, since Gimbal already runs coders in it; make that one boolean in the config struct.
- **Extensions.** Out. This is the largest fidelity loss and should be stated to Tyler as such: anything Pi users do with `.pi/extensions` will not run.
- **Compaction.** In, including the overflow and threshold triggers (`agent-session.ts:2599-2910`). Branch summaries out.
- **Auto-retry** on provider errors (`agent-session.ts:3379-3441`, regressions 3317 and 6019). In, because the Router path is the one most likely to flake.

## 3. Package map and assignments

Root: `internal/pinative/`. The name is chosen so it cannot collide with whatever package `codex/issue-386-pi` lands; that branch has no `pi/` directory yet at `5f0f8a8e`. No new exported root-Gimbal API. The adapter reaches Gimbal only through `internal/binding.Adapter` and `internal/modelalias`, both internal.

Each row is one worker's exclusive ownership. A package may contain frozen contract files owned by the contract owner (column "frozen"); a worker never edits those. Classification: **P** ported, **A** adapted from a Go donor after inspection, **G** provided by Gimbal, **D** deferred.

| # | Package | Wave | Owns (upstream) | Class | Relevant upstream tests | Donor to inspect |
|---|---|---|---|---|---|---|
| 0 | `ai` (contracts) | 0 | `ai/src/types.ts` messages, content, `Usage`, `Model`, `Tool`, `StreamOptions`; `utils/transcript.ts`; `utils/event-stream.ts`; `utils/estimate.ts`; `utils/text.ts`; `models.ts:1187-1245` cost and thinking clamps; the executable tool contract from `agent/src/types.ts:420-470` | P | `ai/test/message-types`, `transcript-tool-changes`, `event-stream`, `context-estimate`, `tool-call-id-normalization` | `sky-valley/ai/{types,transcript,eventstream,estimate}.go` |
| 0 | `agent` frozen: `types.go` | 0 | `agent/src/types.ts` events, hooks, `AgentLoopConfig`, queue modes | P | compile-only until #4 | `sky-valley/agent/types.go` |
| 0 | `coding/session` frozen: `entry.go` | 0 | `session-manager.ts:41-230` header, entry union, tree node, projection types | P | `test/session-manager/load-entries` (shape only) | `sky-valley/coding/session_tree.go:21-75` |
| 0 | `testkit` | 0 | `ai/src/providers/faux.ts` deterministic provider; fixture loaders | P | `ai/test/faux-provider.test.ts` | none |
| 1 | `ai/validation` | 1 | `utils/validation.ts` tool-argument validation, `utils/json-parse.ts` partial JSON salvage | A | `ai/test/validation`, `openai-completions-empty-tools` | uses `santhosh-tekuri/jsonschema/v6` already in go.mod |
| 2 | `ai/retry` | 1 | `utils/retry.ts`, `utils/provider-retry.ts`, `utils/overflow.ts`, `utils/error-body.ts` | P | `ai/test/retry`, `provider-retry`, `overflow`, `context-overflow` | `sky-valley/ai/retry_*.go` |
| 3 | `ai/frame` + `ai/openaicompat` | 1→2 | `utils/assistant-message-frame.ts`; `api/openai-completions.ts` request build (`buildParams` L797-975), SSE parse (L299-730), compat detection (L1585-1700), usage parse (L1511-1552), `api/openai-prompt-cache.ts` | P | all 14 `ai/test/openai-completions-*.test.ts`, `tool-call-without-result`, `cross-provider-handoff` | `sky-valley/ai/providers/openai_stream.go`, `transform.go`, `difftest/scenarios/deepseek*.json`, `empty-text-part-openai-completions.json`, `replayed-args-*` |
| 4 | `agent` (loop) | 1 | `agent-loop.ts`, `agent.ts` | A | `agent/test/agent-loop.test.ts` (31 cases), `agent.test.ts` (30), `e2e.test.ts` with faux | `sky-valley/agent/loop.go`, `agent.go` passed `-race` on 2026-09-25 |
| 5 | `coding/fstools` | 1 | `tools/read.ts` text path, `write.ts`, `edit.ts`, `edit-diff.ts`, `path-utils.ts`, `truncate.ts`, `file-mutation-queue.ts` | P | `test/tools.test.ts` read/write/edit, `edit-tool-legacy-input`, `edit-tool-no-full-redraw`, `file-mutation-queue`, `path-utils`, `builtin-tool-strict-mode` | `sky-valley/coding/editmatch.go`, `tools.go` |
| 6 | `coding/shelltool` | 1 | `tools/bash.ts`, `tools/output-accumulator.ts`, `utils/shell.ts:214-247` process-group kill, `core/exec.ts` | P | `test/tools.test.ts` bash, `suite/regressions/5208-late-bash-output`, `5303-bash-output-truncation`, `8935-parallel-preflight-abort` | `sky-valley/coding/proc_unix.go`; its `TestBashCapturesOutputPastExit` failed in the sample and is the 5208 case |
| 7 | `coding/searchtools` | 1 | `tools/find.ts`, `grep.ts`, `ls.ts`; both spawn `fd`/`rg` (`find.ts:216`, `grep.ts:168`) | P | `test/tools.test.ts`, `regressions/3302-find-path-glob`, `3303-find-nested-gitignore`, `6104-find-root-relativization` | `sky-valley/coding/glob.go` |
| 8 | `coding/session` (store) | 1 | `session-manager.ts` load/migrate (L350-372, L627), tree and branch (L1404-1660), `createBranchedSession` (L1627), `forkFrom` (L1810), append and persist (L1166-1300), projection (L476-596); `core/messages.ts` custom kinds and `convertToLlm` (L148-196) | A | `test/session-manager/*.test.ts` (8 files), `session-file-invalid`, `session-id-readonly`, `agent-session-branching` | `sky-valley/coding/session_store.go`, `session_tree.go` |
| 9 | `coding/prompt` | 1 | `system-prompt.ts`, `skills.ts`, `resource-loader.ts:175-213` | P | `test/system-prompt.test.ts`, `system-prompt-updates`, `skills.test.ts`, `test/fixtures/skills/**` | `sky-valley/coding/systemprompt.go`, `resources.go`; its skill-count failure came from ambient `$HOME`, not proven a defect |
| 10 | `coding/modelconfig` | 1 | `model-config.ts` models.json schema, `provider-composer.ts:523-700` compose and header resolution, `resolve-config-value.ts` `$ENV` only, `runtime-credentials.ts`, `ai/src/env-api-keys.ts` | A | `test/model-runtime-auth-options`, `runtime-credentials`, `resolve-config-value` (`$ENV` cases), `model-runtime-modify-models-compat` | `sky-valley/ai/models_store.go`, `catalog.go` |
| 11 | `coding/compaction` | 2 | `compaction/compaction.ts`, `compaction/utils.ts` | P | `test/compaction.test.ts` (655 lines), `compaction-serialization`, `regressions/7048`, `8328`, `5217`, `fixtures/before-compaction.jsonl` | `sky-valley/coding/compaction.go` |
| 12 | `coding/tools` (registry) | 2 | `tools/index.ts`, `tool-definition-wrapper.ts` | P | `default-tools-setting`, `regressions/5109-exclude-tools` | worker #5 takes this after #5 lands |
| 13 | `coding/runtime` | 2→3 | `agent-session.ts` subset: prompt (L1609-1755), queues (L1814-1907), abort and idle (L2075-2097), settle (L873-977), persistence on `message_end`, `_checkCompaction`/`_runAutoCompaction` (L2599-2910), `_prepareRetry` (L3379), tool loadout and prompt refresh (L1374-1470), fork via `createBranchedSession` (`agent-session-runtime.ts:262-350`) | A | `test/suite/agent-session-{prompt,queue,boundaries,compaction,retry-events,runtime}.test.ts`, `regressions/6363-agent-settled-event`, `8724-in-memory-fork-active-tool`, `7253`, `9340-9777`, `pre-prompt-compaction-no-continue` | `sky-valley/coding/session.go` (thinner; not an oracle) |
| 14 | `harness` (adapter) + `internal/binding`/`modelalias` route | 3 | none; this is Gimbal code modelled on `claude/claude.go`, `claude/events.go`, `opencode/events.go` | G | Gimbal-side tests with a fake Chat Completions server; `issue-386.md` acceptance list | none |
| 15 | integration and qualification | 3→4 | `third_party/pi/LICENSE` and NOTICE; differential fixtures; live Router run | G | see §7 | `sky-valley/difftest/README.md` method |

That is sixteen assignments, three of which (0, 13, 14) belong to a frontier model, one of which (12) is a tail on assignment 5. Wave 1 has nine genuinely independent workers.

## 4. Shared contracts that must settle before fan-out

Wave 0 is one serialized commit by the contract owner, reviewed by a second frontier session, tagged as the baseline. It contains only types, small pure helpers, and the faux provider, and every later worker compiles against it.

**Messages, usage, model.** Go structs for `SystemMessage`, `UserMessage`, `AssistantMessage`, `ToolResultMessage` (`ai/src/types.ts:515-577`), `Usage` with the Pi cost block, `Model` with `contextWindow`, `maxTokens`, `reasoning`, `thinkingLevelMap`, `compat`, and the OpenAI compat override struct (`types.ts:754-810`). JSON tags match Pi's field names exactly, because the session store writes them to disk and the differential harness compares them. Message values are immutable after append: content slices are never shared between a parent and a fork. This is the shallow-snapshot problem the RECOMMENDATION found in Pi Agent Go; the Go contract states it and the fork test checks it.

**Transcript replay.** `getCurrentSystemMessage`, `getCurrentTools`, `getToolStateChanges`, `toToolDeclaration` (`utils/transcript.ts:47-170`). The loop's `declareToolChanges` (`agent-loop.ts:332-370`) and the completions request builder both depend on these, so they are contract, not loop internals.

**Event stream.** `EventStream[T, R]` as a channel-backed type with `Push`, `End`, `Result` (`utils/event-stream.ts:26-105`), and `AssistantMessageEvent` (`types.ts:732-748`). The `StreamFn` signature is the provider seam: `func(ctx, *Model, TranscriptContext, *SimpleStreamOptions) *AssistantMessageEventStream`. The contract in `agent/src/types.ts:21-36` is copied verbatim: a provider never returns an error; failures are a final `AssistantMessage` with `stopReason` `error` or `aborted`.

**Tool contract.** `Tool` declaration plus `Execute(ctx, callID, args json.RawMessage, onUpdate) (ToolResult, error)`, `ExecutionMode`, and `ToolResult{Content, Details, Usage, Terminate}` (`agent/src/types.ts:420-470`). Argument validation is a function of the registry, not the tool. No reflection: tools declare their JSON Schema as a literal, the way `pi-agent-go/tool.go:148-158 Raw` does, and decode with `encoding/json`.

**Loop hooks and events.** `AgentEvent` variants (`types.ts:485-500`), `AgentLoopConfig` with `ConvertToLLM`, `TransformContext`, `PrepareRequest`, `PrepareNextTurn`, `FinishTurn`, `BeforeToolCall`, `AfterToolCall`, `GetSteeringMessages`, `GetFollowUpMessages`. Only the subset `agent-session.ts` actually installs (`_installAgentToolHooks` L532, `_installAgentRequestProjection` L611, `_installAgentNextTurnRefresh` L690) is exercised in the runtime, but the loop is ported whole so its 61 upstream tests translate one to one.

**Session entries.** The v3 entry union (`session-manager.ts:57-197`), header, and `ProjectedSessionEntry`. Compaction and the runtime consume these while the store is still being ported.

**Cancellation and steering.** `context.Context` replaces `AbortSignal` everywhere. `Agent.Abort` cancels the run context; `WaitForIdle` blocks until `agent_end` listeners settle (`agent.ts:341-353`). Steering is a queue drained at the points `agent-loop.ts:167-260` drains it: after a turn's tool results, before the next request, one at a time by default. `Steer` returns `landed=true` only if the message was drained into the transcript before the run ended; the harness checks that by watching for the `message_start` of its own message, the way `_handleAgentEvent` matches queue text (`agent-session.ts:897-915`).

**Adapter boundary.** `RunTurn` = prompt, block on settle, return the last assistant text (`getLastAssistantText`, `agent-session.ts:3972`) as a JSON string, or the extracted JSON object for a schema turn. Events project to the `session.step.*`, `session.tool.*`, `session.usage.*` names that `claude/events.go:206-505` and `opencode/events.go:200-360` emit, so the run page needs no new code. Usage maps Pi `Usage{input, output, cacheRead, cacheWrite, reasoning, cost.total}` onto `gimbal.Tokens` with `Input` = Pi input (already net of cache), `Output` = output minus reasoning, per `usage.go:9-30`. `Fork` = `createBranchedSession(leafId)` into a new file, which is Pi's `clone` (`rpc-mode.ts:619-630`) and exactly Gimbal's meaning. `Close` closes the file handle and kills tracked shell children for that session only.

Wave 0 is also where `go.mod` is settled. The port needs no new module: JSON Schema validation uses `santhosh-tekuri/jsonschema/v6`, already required. Workers may not touch `go.mod` or `go.sum`; a worker that believes it needs a dependency reports it in its result and the integrator decides.

## 5. Dependency waves

```
Wave 0  contracts + testkit            (1 frontier session, serialized, reviewed, tagged)
Wave 1  validation | retry | frame→openaicompat | loop | fstools | shelltool
        | searchtools | session store | prompt | modelconfig     (9 workers, parallel)
Wave 2  compaction (needs retry, session) | tools registry (needs 5,6,7)
        | runtime (needs everything in Wave 1 plus compaction)
Wave 3  harness adapter + binding route (needs runtime)
Wave 4  integration proof, differential fixtures, live Router qualification
```

Integration is serialized after each wave: branches merge one at a time into the integration branch, each followed by `go build ./...`, `go vet ./...`, `go test -race ./internal/pinative/...`, and `./bin/gimbal lint ./...`. Wave 2 starts only from a green integration of Wave 1. Assignment 3's `openaicompat` half may begin before Wave 1 integration because it depends only on its own `frame` package and contracts; assignment 13 begins design while Wave 1 runs but compiles only against integrated Wave 1.

Qualification ownership: each worker proves its package with translated tests; the integrator proves the merged tree builds and passes; the frontier session on assignment 15 proves integrated coding behaviour and owns the acceptance decision. No worker's own report is proof.

## 6. The fan-out workflow

`internal/workflows/piport/piport.go`, ordinary Go, constant call-site names, no DAG engine. Assignments live in a JSON file (data, not code) with name, package, wave, upstream files, tests, allowed directories. Sketch:

```go
const (
    roleContract   gimbal.WorkflowRole = "contract-owner"  // frontier
    roleWorker     gimbal.WorkflowRole = "port-worker"     // cheap: Haiku, gpt-5.6-luna, flash
    roleReviewer   gimbal.WorkflowRole = "port-reviewer"   // mid or frontier
    roleIntegrator gimbal.WorkflowRole = "integrator"      // frontier
)

func Port(ctx context.Context, env gimbal.Env, p Params) error {
    plan := readPlan(p.PlanFile)                       // assignments by wave
    gimbal.Set(ctx, "plan file", p.PlanFile)
    gimbal.Set(ctx, "upstream tree", p.UpstreamDir)    // pinned d6af72e1
    gimbal.Set(ctx, "integration branch", p.Branch)

    if !accepted(env.WorkDir, "contracts") {
        if err := gimbal.Scope(ctx, "contracts", func(ctx context.Context) error {
            owner := gimbal.NewSession(ctx, roleContract, env.WorkDir)
            if _, err := owner.Generate[gimbal.Text](ctx, contractsPrompt); err != nil { return err }
            if err := gates(ctx, env.WorkDir); err != nil { return err }           // Check ×4
            judge := gimbal.NewSession(ctx, roleReviewer, env.WorkDir)
            v, err := judge.Generate[Verdict](ctx, contractVerdictPrompt)
            if err != nil || !v.Accepted { return fmt.Errorf("contracts rejected: %v", v.Reasons) }
            return tagAccepted(ctx, env.WorkDir, "contracts")                      // RunCommand git tag
        }); err != nil { return err }
    }

    for waveCtx, wave := range gimbal.Iterate(ctx, "wave", plan.Waves) {
        group := gimbal.Group(waveCtx, "workers")
        for _, a := range wave {
            if accepted(env.WorkDir, a.Name) { continue }                          // restart: skip done
            group.Go("assignment", func(ctx context.Context) error { return assign(ctx, env, p, a) })
        }
        if err := group.Wait(); err != nil { return err }                          // operational failure only
        for intCtx, a := range gimbal.Iterate(waveCtx, "integrate", wave) {
            if err := integrate(intCtx, env, p, a); err != nil { return err }      // serialized merges
        }
    }
    return nil
}
```

`assign` creates a worktree with `RunCommand(ctx, "worktree add", env.WorkDir, "git", "worktree", "add", "-B", "port/"+a.Name, dir, p.Branch)` from the integration branch at the wave's start, so every worker in a wave sees the same baseline. It then iterates bounded attempts:

```go
for attemptCtx, n := range gimbal.Iterate(ctx, "attempt", attempts(p.MaxRevisions)) {
    worker := gimbal.NewSession(attemptCtx, roleWorker, dir)
    coach  := gimbal.NewSession(attemptCtx, roleReviewer, dir)
    if _, err := worker.Generate[gimbal.Text](attemptCtx, workerPrompt,
        gimbal.WithSupervisor(coach, scopeCoaching)); err != nil { return err }
    gimbal.Check(attemptCtx, "ownership", dir, "sh", "-lc", ownershipCheck)   // diff touches only a.Dirs
    gimbal.Check(attemptCtx, "build", dir, "go", "build", "./...")
    gimbal.Check(attemptCtx, "test", dir, "go", "test", "-race", "./"+a.Package+"/...")
    gimbal.Check(attemptCtx, "vet", dir, "go", "vet", "./"+a.Package+"/...")
    reviewer := gimbal.NewSession(attemptCtx, roleReviewer, dir)
    v, err := reviewer.Generate[Verdict](attemptCtx, reviewPrompt)
    if err != nil { return err }
    if v.Accepted { commitAndTag(...); gimbal.Set(ctx, "outcome", "accepted"); return nil }
    gimbal.Set(attemptCtx, "rejection", v.Reasons)   // next attempt's session sees it via scope
}
gimbal.Set(ctx, "outcome", "rejected")
return nil
```

Points the sketch settles:

- **Group semantics.** A rejected assignment returns `nil` after recording its outcome, so siblings finish. Only harness or context failures return an error, which cancels the wave and surfaces from `Wait`. After `Wait`, `integrate` refuses an unaccepted assignment and ends the run naming its branch. This matches `group.go:70-85`: first error cancels, `Killed` does not.
- **Ownership.** The `ownership` check lists files changed since the wave baseline and fails on anything outside the assignment's directories, on `go.mod`, `go.sum`, any `*_gen.go`, the contract files, `third_party/`, or any integration file. The reviewer prompt reads that check from scope context.
- **Locality.** Each worker prompt names absolute paths: the assignment JSON, the upstream files and tests, the donor files, the contract package. Nothing points at a remote. The worker's first instruction is to read the upstream test file before the upstream source.
- **Scope coaching.** The supervisor instruction opposes porting files outside the assignment, adding abstractions the upstream does not have, and "improving" behaviour instead of translating it. Advisory only.
- **Independent review.** The reviewer is a fresh session in the same worktree, sees the check results, and judges test translation coverage against the upstream test list, not the worker's report.
- **Bounded revision.** `MaxRevisions` attempts, each in its own `Iterate` scope so `Check` keys stay constant and single-write (GIMBAL102, GIMBAL103).
- **Integration.** `integrate` merges `port/<name>` with `--no-ff` into the integration branch in the main worktree, runs the four gates, and on failure gives one frontier integrator session a bounded fix in the main worktree before failing the run. Conflicts are impossible by construction when ownership held, so a conflict is itself a finding.
- **Restart.** Acceptance is a git tag `port/<name>/accepted`. Re-running skips accepted assignments, recreates missing worktrees from existing `port/<name>` branches, and resumes the wave. No state file, no run output in the tree.
- **Models.** `roleWorker` binds to a cheap model per `AGENTS.md`. Assignments 0, 13, 14, 15 are not dispatched through `assign`; they are frontier turns written inline in the same file with the same gates, because their interfaces are the uncertain part.
- **Bootstrapping.** The workflow runs on Codex, Claude, or Gemini bindings that exist today. Nothing in it needs the Pi port to build itself.

## 7. Proof

**Per package.** Translated upstream tests beside the code, named after the upstream file (`agent_loop_test.go` cites `packages/agent/test/agent-loop.test.ts`). Fixtures copied from upstream `test/fixtures/**` keep their upstream path in a header comment. A translated test that cannot be made to pass is recorded in the package's `doc.go` with the upstream name and the reason; the reviewer rejects silent omission.

**Differential, no network.** Copy sky-valley's method (`difftest/README.md`): capture the request body pi builds through `onPayload`, which halts before the first byte, and compare canonical JSON against what `openaicompat` builds for the same transcript. The capture script and pinned `npm` install live in `/Users/tyler/.local/share/gimbal-pi-research/scratch/difftest`, outside the repository. Only the resulting fixture pairs are committed, under `internal/pinative/ai/openaicompat/testdata/difftest/`, each with the pi commit in its header. Scenarios to start: a Router-shaped `models.json` provider with `api: openai-completions` and a minimal model entry, then `deepseek`, `empty-text-part`, `replayed-args-*`, `reasoning`, `basic-tools-cache`. Pi is the oracle; a sky-valley divergence is never an excuse.

**Session parity.** Load `test/fixtures/large-session.jsonl` and `before-compaction.jsonl`, build the projection, and compare message-for-message with what pi's `buildSessionContext` produces, captured once into testdata by the same scratch script. Fork test: write a session, fork at the leaf, append different turns to each, reload both, assert neither leaked.

**Integrated coding behaviour.** A Go test in `internal/pinative/harness` starts a fake Chat Completions server that scripts tool calls, drives the real adapter through `gimbal.Run`, and asserts the `issue-386.md` list: prompt to settle, tool projection, usage, schema re-ask, steer race, cancellation with process-group kill within a deadline, fork independence, close, resume in a new process. This is a test, not a proof program.

**Completion observations.** The port is done when Tyler or the coordinator has seen: one live Router-backed workflow turn read, edit, and run a command and return valid text; a schema turn validated by `session.go`; a second turn resuming the session; a steer that landed; a cancelled turn whose shell child is gone; a fork with two independent follow-ups; and `gimbal run-prompt --model pi/diffusion/deepseek-4.1-flash` working after cutover. Recorded in the PR as observation, with model, Router model id, event sequence, and usage. No logs committed.

**License.** Pi is MIT, Mario Zechner 2025 (`LICENSE`). Add `third_party/pi/LICENSE` and `third_party/pi/NOTICE` listing every Go package that translates Pi files and the pinned commit. Sky-valley is MIT with its own copyright line plus Zechner's; any function adapted from it gets the same treatment in NOTICE. Each translated file header names its upstream path.

## 8. Cutover and the #386 adapter

`codex/issue-386-pi` has not landed a `pi` package at `5f0f8a8e`. The plan does not touch the `pi/` name. During development the binding harness name is `pinative` and the route is `pinative/diffusion/deepseek-4.1-flash`, changed in `internal/modelalias` only. After #386 merges, one named cutover owner rebases, decides whether `pi/...` routes to the native adapter or the RPC adapter, and deletes what is replaced; `AGENTS.md` forbids keeping both behind a shim. Other harnesses are untouched throughout.

## 9. Risks

- **`agent-session.ts` is 4,023 lines with extension hooks woven through.** Assignment 13 is the real risk. Mitigation: the frontier owner writes it against the translated `suite/agent-session-*.test.ts` files and the regressions listed above, and stops at the subset those tests exercise.
- **Late shell output.** Sky-valley's failing `TestBashCapturesOutputPastExit` matches upstream regression 5208; Go pipes and process groups differ from Node's detached spawn. The `shelltool` worker must translate 5208 and 5303 and prove kill-on-cancel with a grandchild that ignores SIGTERM.
- **Router request compatibility.** `buildParams` sends `store: false`, `stream_options`, and `max_completion_tokens` for a non-listed base URL (`openai-completions.ts:830-846`, `detectCompat` L1585). Issue 386 says Router acceptance of those is unverified. The compat override struct exists for exactly this; the live milestone decides the values.
- **Model metadata.** A minimal `models.json` entry yields 128K context, 16,384 max tokens, `reasoning=false`, zero cost. Compaction thresholds and cost reporting are only as good as that entry. The cutover owner sets verified values.
- **Session file locking.** Pi has none (single-threaded). One mutex per session file in the store, and two sessions never share a file.
- **Cheap workers translating subtle code.** The edit fuzzy matcher (`edit-diff.ts`, 556 lines) and the compaction cut-point (`compaction.ts:468-606`) are where a cheap model is most likely to be confidently wrong. Their test lists are the longest for that reason, and the reviewer role should be frontier for those two.
- **Process-global state.** Pi caches `!cmd` results and tracks child PIDs process-wide (`utils/shell.ts:196-212`). The port keys both by session.

## 10. Open decisions for Tyler and the coordinator

1. Accept the extension-runtime exclusion as permanent, or reserve a later assignment for a Go-native hook surface?
2. Keep Pi's JSONL v3 on disk (recommended) or design a Gimbal-native store and give up fixture reuse and import?
3. Ship `find`, `grep`, `ls` in milestone one (they need `rg` and `fd` on the machine) or defer to after Router qualification? Pi's default loadout is `read`, `bash`, `edit`, `write` (`sdk.ts:258`).
4. Which model serves `roleReviewer` for the two hard packages, and is a frontier reviewer worth its cost on the other seven?
5. Where the scratch differential capture lives long-term: outside the repo as proposed, or as a separate-module `difftest/` directory the way sky-valley did?
6. Whether `pinative` becomes the permanent name or collapses into `pi` at cutover.
