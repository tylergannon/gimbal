# Adversarial review: Gimbal warm development, round 01

Date: 2026-10-14. Worktree `/Users/tyler/src/gimbal-worktrees/gimbal-warm-dev`,
branch `codex/gimbal-warm-dev`, base `1dc4fca8`, head `c0aeabef` (four commits,
37 files). Read-only review; only this file was written.

## Review target and scope

Target: the complete Gimbal implementation for SKGO issue #311 as it stands at
head, judged against the issue text (`/tmp/gimbal-warm-dev-issue311.json`),
`AGENTS.md`, `docs/definition-of-done.md`, and the caller's restated user
context: Gimbal must use SKGO's shared retained generator/dev supervisor, avoid
needless generator/application compilation coupling, preserve frontend HMR, and
show correct Go body, API, tag, config and dependency changes plus error
recovery in the browser, with lowest correct visible latency as the criterion
and no native hotpatch required.

Caller statements treated as operating context, not as scope limits: "SKGO's
new API release is still in flight", "no absolute replacement is committed",
and "published Polytype v1.9.0 is pinned". Those describe the state of the
tree; they do not make the resulting build and proof gaps out of bounds, and
they are reported below. No other narrowing was requested.

## Evidence inspected

- Full diff `1dc4fca8..c0aeabef`, and the surrounding code it touches:
  `internal/generate/appgen/{dev.go,dev_inputs.go,main.go}`,
  `internal/generate/{stock.go,write.go,workflow_page.go,definition.go}`,
  `workflow/{description.go,schema.go,graph.go}`, `registry.go`, `roles.go`,
  `internal/builtin/`, `web/runtime.go`, `web/server.go`, `web/vite.config.*`,
  `Justfile`, `README.md`, `go.mod`, `go.sum`.
- SKGO's dev supervisor as actually consumed: `/tmp/gimbal-dev-producers/skgo`
  (`dev/dev.go`, `internal/dev/server.go`, `internal/dev/watch.go`), and the
  live worktree `/Users/tyler/.codex/worktrees/skgo-warm-dev` at `b9cfabe`.
- Polytype as pinned (`$GOMODCACHE/.../polytype@v1.9.0`) versus the frozen
  copy `/tmp/gimbal-dev-producers/polytype`.
- Worklog `ephemeral/worklog/gimbal-warm-dev.md`; proof logs
  `/tmp/gimbal-{browser-final,browser-integration,fast-body,quiet-body,dependency-tag,registry-api,warm-dev-frozen,stock-registry-mutant}.log`
  and the supervisor logs `/tmp/gimbal-{final,fast,dependency,registry,latest}-dev.log`;
  the Playwright scripts `/tmp/gimbal-{browser-final,dependency-tag,registry-api,fast-body,quiet-body}.cjs`;
  screenshots `/tmp/gimbal-{final-diagnostic,latest-repair,dependency-tag,registry-api}.png`.
- Commands run by this review (outputs in the session, nothing written to the tree):
  - `GOWORK=off go build ./...` and `GOWORK=off go vet ./internal/generate/appgen`
    against the committed `go.mod`.
  - `go test -count=1 ./internal/generate/... ./workflow/... .` with the
    frozen workspace `GOWORK=/tmp/gimbal-dev-frozen.work` (the shell default here).
  - `GOWORK=off go test -count=1 ./internal/generate/` (published pins).
  - `go vet ./...` under the frozen workspace: clean.
  - `go tool polytype --typescript <scratch>` from `workflow/` to compare the
    authoritative tool output against the committed and worker-written
    `web/src/lib/workflow/types.ts` (identical modulo Prettier formatting).
  - `diff -rq` of frozen producer copies against the published module and the
    SKGO worktrees.

## Findings

### 1. Critical: the committed head does not build with the committed module graph

`internal/generate/appgen/dev.go:20-21` imports `github.com/tylergannon/skgo/dev`
and uses `dev.Config{... StopBeforeStart, RestartFiles, WatchInputs, Debug}`
and `skgo.Worker`. `go.mod:34` pins `github.com/tylergannon/skgo v0.31.0`,
which has no `dev` package, and no `replace` or `go.work` is committed.

Reproduction against the tree as committed:

```
$ GOWORK=off go build ./...
internal/generate/appgen/dev.go:21:2: no required module provides package github.com/tylergannon/skgo/dev
```

The only environment in which the tree compiles is an uncommitted
`GOWORK=/tmp/gimbal-dev-frozen.work` pointing at `/tmp/gimbal-dev-producers/{skgo,polytype}`.
Nothing in the repository records which SKGO revision is required; the frozen
SKGO copy matches no committed SKGO commit (it differs from
`skgo-warm-dev@b9cfabe` in `generate/generate.go`, `internal/gen/worker.go`,
`internal/gen/worker_projection.go`, `cmd/skgo/dev.go`, and its `go.mod`
still says `polytype v1.8.0` plus a `replace` to the frozen polytype).

Impact:

- `just build`, `just dev` (`go run ./internal/generate/appgen dev`), CI, and
  the commit-hook smoke tests all fail for anyone without that private
  workspace. `README.md:342-349` documents `just dev` as working.
- There is no single environment in which the test suite passes. Under the
  frozen workspace, 16 pre-existing fixture tests in `internal/generate` fail
  with `current directory is contained in a module that is not one of the
  workspace modules listed in go.work` (seen this session; only the four new
  tests set `GOWORK=off`). Under published pins, `internal/generate` passes
  (23.9s) but `internal/generate/appgen` cannot compile at all, so
  `TestGeneratorBuildInputsExcludeObservedApplication`,
  `TestGeneratorInputsSeparateImplementationFromObservedSource` and
  `TestWorkflowWorkerPreservesSourceUnionAndRepairsOutput` only ever run in the
  private workspace.

Issue #311 asks that the bounded gains "land" so other lanes can proceed; a
head that cannot be built from its own `go.mod` has not landed. Either the SKGO
release must be cut and pinned before this merges, or the dependency must be
expressed in the tree (the caller notes no absolute replacement is committed,
which is correct, but that leaves the requirement unmet rather than satisfied).

### 2. Issue: the browser proof did not exercise the pinned dependencies

Every supervisor and browser log cited as proof was produced with the frozen
`/tmp/gimbal-dev-producers/polytype`, which is **older** than the pinned
v1.9.0 and differs in exactly the retained-replay path:

- `goload/session.go:244-251` (v1.9.0) identifies source-checked packages by
  captured bytes with `ExportFile` cleared; the frozen copy appends a source key
  only for `changedSources`. `session.go:290` changes the rediscovery trigger
  from `len(changedSources) > 0` to `discover != nil`.
- `internal/builder/programmatic_worker.go:506-514` (v1.9.0) preserves
  `//go:build` placement during acquisition and handles `import "C"`; the
  frozen copy lacks both. The tag-change proof (`gimbal-dependency-tag.cjs`)
  therefore ran against a builder that does not have the build-constraint fix
  the pinned release ships.
- `codegen/worker.go` and `goload/view.go` differ in retention documentation
  (`SetMaxUniverses`); the frozen SKGO also still requires polytype v1.8.0.

`go.mod` pins v1.9.0, `go.sum` carries both v1.8.0 and v1.9.0, and the worklog
and README describe the loop as using the pinned release. The PlanHits,
SourceCaptures and replay counts in `/tmp/gimbal-*-dev.log` are therefore
evidence about an unreleased polytype snapshot, not about what a reader of
`go.mod` will run. The published-pin run I could perform here
(`GOWORK=off go test ./internal/generate/`) passes but does not touch the
dev loop. A `/tmp/gimbal-published-browser.cjs` exists (02:30, after head) with
no corresponding log, so that gap was recognised and is still open.

Per AGENTS.md ("Proof is what you saw"), the body/API/tag/config/dependency and
error-recovery sequences themselves are credible: the scripts assert eight
rendered rows per literal, verify hydration through an ordinary navigation
before trusting HMR, check generated `badge: string` in the route `types.ts`,
drive a real compile error through the Vite overlay and the standalone
diagnostic page, and restore sources in `finally`. The screenshots match the
logs. What is missing is one run of the same sequence against the module graph
the tree actually pins.

### 3. Issue: dead generated output is still produced and committed

Commit `45fb79de` deleted `internal/builtin/workflow_pages.go`, the only reader
of `internal/builtin/pages/*.json`, so that generator output changes would stop
rebuilding the generator. `internal/generate/workflow_page.go:101` still writes
those eight files on every stock generation, and `git ls-files
internal/builtin/pages` shows all eight committed. A repository-wide search
finds no other reference (`WorkflowPageJSON`, `builtin/pages`, `pages/`) except
the new guard in `build_inputs_test.go:41`, which forbids embedding them but not
producing them. The same `sourcePage` JSON is already serialised into each
`workflow_gen.go` definition (`definition.go:43`).

Impact: every workflow edit under `just dev` dirties eight JSON files with no
consumer, `TestStockRecoversMissingAndStaleOutput` now asserts their stability
as if they mattered, and AGENTS.md's "delete what is replaced" is not met.
Remove the write at `workflow_page.go:100-103` (and `writePageJSON`), delete
the directory, and drop the `pages/` branch of the build-inputs test.

### 4. Issue: visible latency includes a serving gap Gimbal controls and did not address

The issue's criterion is source save through the correct visible browser
result. Measured in the final logs:

| edit | visible (ms) | generation | compile | replacement ready |
|---|---|---|---|---|
| Go body (`browser-final`) | 7,409 | 463–643 ms | 3.5–3.9 s | ~0.8 s |
| Go body (`quiet-body`) | 4,335 / 5,874 | ~0.5 s | 1.4–3.1 s | ~0.8 s |
| API field (`browser-final` / `registry-api`) | 11,817 / 9,393 | 5.2–5.7 s | 1.8–3.8 s | ~0.8 s |
| repair after error | 5,995 | 481 ms | 3.5 s | 0.9 s |

Two costs sit outside generation and compilation:

- `web/runtime.go` binds the listener, then reads the dev manifest from Vite and
  assembles `NewHandler` (`web/runtime.go:203-209`), and only afterwards logs
  "web application listening" (`web/runtime.go:238`). SKGO declares
  "replacement ready" on TCP readiness, and because `StopBeforeStart: true`
  (`dev.go:316`) has already stopped the predecessor, the browser's reload
  request waits in the accept backlog for the whole assembly. In
  `/tmp/gimbal-dependency-dev.log` that is 02:15:53 (control socket) to
  02:15:59 (web listening) on the cold start; warm replacements show roughly a
  one-second gap, and `skgo: rendering /workflows: context canceled` appears at
  02:17:05 as the previous request is dropped. The worklog records this and
  leaves it. Assembling the handler before `net.Listen` (or signalling
  readiness after assembly) removes it from every edit for free; it is
  Gimbal's own code, not SKGO's.
- An API/type edit costs 5.2–5.7 s inside `worker.Run(cfg)` on every run
  (`TypeScriptRuns` +1, `PlanHits` unchanged), roughly half the visible time.
  That is SKGO's selection, not Gimbal's, but the issue's "select only affected
  projections" goal is visibly not met for the one edit class users feel most,
  and no Gimbal- or SKGO-side issue records it.

Neither item invalidates the correctness proof; both are the difference
between "works" and the stated criterion.

### 5. Nitpick: `dev.go` carries duplicated and inert work

- `dev.go:230-234` runs `go tool polytype --typescript ../web/src/lib/workflow`
  once at startup, then `dev.go:274-277` has the retained workflow `Worker`
  generate the same Go codec and TypeScript into the same directories on the
  first `project` call (`workflow={Analyses:1 Emissions:1}` in every cold log).
  The subprocess is redundant. The comment's rationale (codecs are part of the
  compiled generator model) is already enforced by `RestartFiles` including
  `workflow/*.go`.
- `dev.go:256` declares `output bytes.Buffer` that nothing writes; every
  return of `output.Bytes()` is empty, so generation failures reach the
  diagnostic page with error text only.
- `TestWorkflowWorkerPreservesSourceUnionAndRepairsOutput` compares the worker
  against `codegen.Generate` with the same programmatic
  `workflowConfiguration()`, not against the `go tool polytype` path that
  `workflow/schema.go:13-14` (`SealedUnion[Operation]("kind", Snake)`)
  defines. I verified by hand that the two agree modulo Prettier formatting;
  the test does not establish it.
- `README.md:350` still says `just dev-web` and `just dev-go` remain available;
  they do (`Justfile:28,31`), but `web/dev/run.mjs` is gone and `dev-go` no
  longer matches the supervisor's build flags (`-w`), so the two paths now
  produce different binaries. Minor documentation drift.

## What holds up

- Generator/application decoupling is real and tested:
  `TestGeneratorBuildInputsExcludeObservedApplication` runs `go list -deps`
  on the actual generator and fails if it compiles `gimbal`,
  `internal/observation`, or any `internal/workflows/*`.
  `workflow/description.go` with aliases in `registry.go`/`roles.go` is the
  minimal cut; it is not a wrapper.
- The `!jsonschema`/`jsonschema` registry split is load-bearing:
  `/tmp/gimbal-stock-registry-mutant.log` shows the fixture failing when the
  guard is reversed, and the projection stub panics rather than silently
  omitting registration.
- `writeGeneratedFile` plus the fixed-mtime assertion in
  `TestStockRecoversMissingAndStaleOutput` genuinely prove that unchanged
  output does not wake watchers.
- `generatorFiles` restart inputs are derived from `go list -deps` of the
  generator itself, including embeds, `go.mod`/`go.sum` and `GOWORK`; SKGO
  fingerprints them by content (`internal/dev/watch.go:148-192`), so identical
  rewrites do not trigger spurious restart diagnostics.
- Frontend HMR is preserved: the proof confirms a Svelte edit renders in
  ~785 ms with zero generator calls and the page object survives, and the
  `workflowSource` Vite plugin is still configured.
- `StopBeforeStart` is the correct choice given the exclusively owned catalog;
  the worklog's record of the candidate failing to acquire the project is the
  evidence.

## Outcome

material findings remain
