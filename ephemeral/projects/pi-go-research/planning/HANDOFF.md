# Handoff: start here

Written 2026-09-25 by a Claude session that ran from the wrong repository
(skgo-project) and handed the job to a fresh session in this worktree.
Read [REQUIREMENTS.md](REQUIREMENTS.md) first: it is Tyler's direction and
overrides PLAN.md where they differ. Then read this file, then
[WORKFLOW.md](WORKFLOW.md) and [MODULES.md](MODULES.md).

## State

- Branch `codex/pi-go-research`, rebased onto `main` at `6dc64f83` (includes
  the Pi RPC harness from #392 and model discovery docs from #393). Pushed.
- Nothing implemented yet. No workflow written yet.
- ChatGPT/Codex is unusable on Tyler's Mac (server-side 401 naming an
  `sk-svcacct` key; OpenAI's problem, not local config). No Codex roles.
- Tyler will paste a temporary `DIFFUSION_API_KEY`. Keep it out of the
  repository. He deletes it at the end of the day.
- The untracked `corpus/` folders and `.gimbal/` run state in this worktree
  are not needed and are not to be committed.

## Coder model (research, 2026-09-25)

The Router's catalog is not public: `GET https://router.diffusion.io/v1/models`
without a key returns 401 / "request unavailable", and diffusion.io publishes
no catalog. **First action once the key arrives:** list chat+tools models
(command in `docs-site/src/routes/docs/harnesses/+page.svx`) and record the
real IDs here. Tyler expects more than the IDs `pi/adapter.go` hardcodes,
including a non-flash DeepSeek if one exists. Check for Kimi and Qwen too.

IDs known locally (context windows in `pi/adapter.go`): `deepseek-4.1-flash`
(1M), `glm-5.3-flash`, `glm-5.3`, `glm-5.3-vision`, `glm-5.2-vision`
(524K), each with `-background` variants. Only `deepseek-4.1-flash` has been
exercised through Pi.

Public 2026 evidence (mostly vendor-reported, not reproduced under one
harness): GLM-5.3 is rated the top open-weight coder on several boards
(FrontierSWE 78.1, Terminal-Bench 3.0 open-source best, Vals 95.4%).
DeepSeek V4.1 Flash scores 90.6% on Terminal-Bench 2.1 and is very cheap.
Kimi K3 is strong but reported to drop reasoning content across tool calls in
long loops. Qwen3.8 ranks lower. Public V4.1 is flash-only; "V4 Pro" is the
previous generation.

Default: `glm-5.3` as primary coder, `deepseek-4.1-flash` as the cheap
fallback, unless the live catalog shows something stronger. Confirm with one
head-to-head on a real module before fanning out.

## Tyler's working expectations

Drive the job to done without asking about routine steps (rebase, obvious
next actions). Answer research questions yourself, preferably through
cheaper subagents (Sonnet) so the main context stays small. Verify against
the live source (the Router's real API, upstream code), not what local code
happens to mention. Only real scope or product decisions go to Tyler.

## Core loop files (research, 2026-09-25)

Upstream root `U` = `/Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi`
at `d6af72e1857cfb10b41d8ff8e69f0d72b4cf6d31`. `A` = `U/packages/agent`,
`L` = `U/packages/ai`, `C` = `U/packages/coding-agent`.

Required for a working Router-backed loop:

| TypeScript | Lines | Role |
|---|---|---|
| `C/src/core/sdk.ts` | 454 | builds Agent and AgentSession, registers tools |
| `C/src/core/agent-session.ts` | 4023 | session lifecycle; much of its bulk is compaction, export, themes (drop) |
| `C/src/core/messages.ts` | 196 | `convertToLlm`, history to LLM projection |
| `A/src/agent.ts`, `A/src/agent-loop.ts`, `A/src/types.ts` | 613, 898, 500 | the established loop, queues, steering, tool scheduling |
| `L/src/types.ts` | 1133 | model, message, usage types |
| `L/src/api/openai-completions.ts`, `simple-options.ts` | 1726, 92 | Chat Completions request, stream, replay |
| `L/src/api/transform-messages.ts`, `L/src/utils/json-parse.ts`, `provider-retry.ts`, `overflow.ts` | small | shared stream/retry/replay primitives |
| `C/src/core/tools/` read, bash, edit, edit-diff, write, grep, find, ls, path-utils, truncate, file-mutation-queue, output-accumulator, tool-definition-wrapper, index | about 3400 | default tools: `createCodingTools()` = read, bash, edit, write; read-only set adds grep, find, ls |

**Correction, from Tyler:** an earlier draft of this file listed
persistence, compaction, resources and settings as "deferred" and proposed an
in-memory session. That was wrong. Tyler cut only other provider families,
OAuth and PowerShell/Windows. Everything else in PLAN.md's headless engine
stays, including events, durable session history, compaction and branch
summaries, skills, prompt templates, system prompt assembly and settings.
The table above lists only the core loop files. See the scope section below
for the full set.

Router request facts from `openai-completions.ts`: `baseURL` comes from the
provider config; `stream_options.include_usage = true`; `store = false`;
`max_completion_tokens` (not `max_tokens`) for an unknown host such as
`router.diffusion.io`; `reasoning_effort` via the generic OpenAI-style branch;
no host-specific quirks apply to the Router. Gimbal's current `pi/adapter.go`
writes exactly that provider shape and runs Node Pi over RPC; the Go port
replaces that adapter.

Upstream tests for the slice run offline with in-process fakes (no keys):

| Test | Cases |
|---|---|
| `A/test/agent.test.ts` | 30 |
| `A/test/agent-loop.test.ts` | 31 |
| `L/test/openai-completions-retry.test.ts` | 3 |
| `L/test/abort.test.ts` | 39 (local mock servers) |
| `C/test/tools.test.ts` | 82 |
| `C/test/path-utils.test.ts` | 13 |
| `C/test/file-mutation-queue.test.ts` | 7 |
| `C/test/edit-tool-legacy-input.test.ts` | 8 |
| `C/test/agent-session-concurrent.test.ts` | 7 |
| `C/test/default-tools-setting.test.ts` | 5 |

Running upstream tests: npm workspaces, `package-lock.json`, vitest. After
`npm install` at `U`: `cd U/packages/ai && npx vitest run test/<file>`.
Nothing is installed yet.

## Scope: MODULES.md minus three families

Keep every MODULES.md assignment except those Tyler cut:

| ID | Package | Status for this job |
|---|---|---|
| 01 | `internal/pi/model/` | keep: messages, events, tool and history contracts |
| 02 | `internal/pi/wire/` | keep: streaming, retry, partial JSON, overflow, replay |
| 03 | `internal/pi/providers/openai/` | narrow to `openai-completions.ts` (Chat Completions). Drop Responses, Codex and Azure |
| 04 | anthropic | **cut** |
| 05 | google | **cut** |
| 06 | bedrock, mistral, pi-messages | **cut** |
| 07 | auth | **cut** OAuth. Keep only API-key resolution such as `$DIFFUSION_API_KEY`, inside 08 |
| 08 | `internal/pi/config/` | keep: settings, model config, models store, config values. Multi-provider catalog reduced to the Router |
| 09 | `internal/pi/files/` | keep: path utils, truncation, mutation queue, image processing |
| 10 | `internal/pi/readtools/` | keep: read, grep, find, ls |
| 11 | `internal/pi/edittools/` | keep: edit, edit-diff, write |
| 12 | `internal/pi/shell/` | keep bash and output accumulator. **Cut** PowerShell |
| 13 | `internal/pi/resources/` | keep: resource loader, skills, prompt templates, system prompt |
| 14 | `internal/pi/history/` | keep: `session-manager.ts` durable v3 JSONL, tree, branches, reopen |
| 15 | `internal/pi/compact/` | keep: compaction, branch summarization |
| 16 | `internal/pi/agent/` | keep: loop, events, queues, steering, follow-ups, tool scheduling |
| 17 | `internal/pi/session/` | keep: AgentSession assembly, settlement, auto-compaction, reopen, fork |
| 18 | `internal/pi/adapter/` | keep: Gimbal harness binding with events, usage, steer, fork, cancel |
| 19 | `internal/workflows/piport/` | keep: the fan-out delivery workflow |
| 20 | integration | keep: `go.mod`, notices, cutover from the #392 RPC adapter, `cmd/pigo/` temporary CLI |

That leaves 16 assignments. Waves follow PLAN.md: 01 and 19 first. Then 02,
09, 14 and 16. Then 03, 08, 10, 11, 12, 13 and 15 in parallel. Then 17, 18
and 20.

## Writing the fan-out workflow (research, 2026-09-25)

Tyler wants `internal/workflows/piport`: like `implement` for each module, but
all ready modules at once under a `Group`, then serial integration.

- **`implement` today** (`internal/workflows/implementation/implementation.go`,
  `Implement(ctx, env, Params{OutcomesFile, MaxTasksPerOutcome})`):
  `gimbal.Iterate(ctx, "outcome", outcomes)` runs outcomes one after another.
  Per outcome: a planner (`RoleSprintPlanning`) drives `gimbal.PromiseLoop`.
  Each `loop.Tasks` item opens a fresh `coding` worker and runs
  `Generate[gimbal.Text]`, then `gimbal.Check` on the task's command. After
  that, an independent validator (`RoleQAOrchestration`) returns an
  `Assessment`. Every step has a scope coach (`RoleArchitecturalCritique`)
  via `WithSupervisor`. It does not commit or touch worktrees.
- **Group** (`group.go`): `g := gimbal.Group(ctx, "name")`,
  `g.Go("child", func(ctx) error)`, `g.Wait()` (always call it). The first
  error cancels siblings unless its cause is `Killed`. Names are constants.
- **Constraint** (`internal/generate/expr.go`): `Go` must be called in the same
  body that declared the group. A `Go` inside a `for` loop is not read, and the
  diagnostic is "Go on a group declared outside this body is not read". So write
  one explicit `g.Go("openaicompat", ...)` per module, not a loop.
- **Examples of named fan-out:** `pyramidsummary` (six children),
  `researchdocument` (five), `validateproduct` (three).
- **Best template:** `internal/workflows/implementinterview/implementinterview.go`
  (179 lines, not registered). It has a Group of named children, then a
  serial planner, coder, `RunCommand` checks and validator.
- **Other primitives:** `Iterate[T](ctx, scope, items)`;
  `RunCommand(ctx, name, workdir, cmd, args...) (exit, stdout, stderr, err)`,
  where a nonzero exit is not an `err`, so check `exit`; `Check(ctx, key,
  workdir, cmd, args...)`; `NewSession(ctx, role, workdir)`, which panics if
  the role is unbound; `Generate` prompts must be constants (GIMBAL108).
- **Registration:** add `//go:generate go tool polytype --validate` and
  `//go:generate go run .../internal/generate/gimbalgen -entry <Entry> -name
  <name>` in the package. Add an entry to `internal/builtin/workflows.go`
  `Workflows`, and role defaults to `internal/builtin/defaults.json`. The
  defaults currently point at Codex models, so override the roles with flags.
  `just build` runs `go generate ./...`, producing `workflow_gen.go` and
  `cmd/gimbal/<pkg>_gen.go`. `just vet` runs `gimbal lint`
  (GIMBAL101–109).
- **Roles for this job:** coder `pi/diffusion/<model>`; planner, validator and
  coach on Claude Sonnet. No Codex.

## Go donor

`/Users/tyler/.local/share/gimbal-pi-research/sources/sky-valley--pi`
(module `github.com/sky-valley/pi`, MIT, Sky Valley 2026; upstream Pi is MIT,
Mario Zechner 2025). Keep both notices with translated code. Coverage:
`ai/providers/openai*.go` (about 4800 lines, thorough tests), `agent/agent.go`
+ `loop.go` + `types.go` (about 2000 lines, heavy tests), `coding/tools.go`
and session files (parity-named tests), `difftest/scenarios/` (61 scenarios,
possibly pinned to a different upstream commit). Earlier checks: its
`ai/providers` and `agent` tests passed; two `coding` tests failed. Using it
as a starting point for the provider and loop, verified against the pinned
TypeScript tests, is the fastest honest route to a working slice.
