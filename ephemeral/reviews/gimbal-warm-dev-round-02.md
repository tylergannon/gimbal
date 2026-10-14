# Adversarial review: Gimbal warm development, round 02

Date: 2026-10-14. Worktree `/Users/tyler/src/gimbal-worktrees/gimbal-warm-dev`,
branch `codex/gimbal-warm-dev`, base `1dc4fca8`, head `7a4118fc` (seven
commits, 51 files; 37 outside the deleted `internal/builtin/pages` JSON,
lockfile and `go.sum`). Read-only review; only this file was written. Scratch
output went to the session scratchpad, not `/tmp`.

## Review target and scope

Target: the complete Gimbal work for SKGO issue #311 as it stands at head,
judged against the issue text (`/tmp/gimbal-warm-dev-issue311.json`),
`AGENTS.md`, `docs/definition-of-done.md`, the round-01 review, and the
caller's restated user context: ship the completed bounded gains; remaining
edit-to-visible latency stays open in SKGO #311; the user still wants the
lowest correct latency; no subsecond-reload or completed-#311 claim is made.

Caller statements about the earlier causal claim (moving `net.Listen` after
`NewHandler`) and about the Polytype CLI versus the retained Worker were
treated as hypotheses to check against source and logs, not as conclusions.
Both are assessed below. No narrowing of scope was requested or applied.

## Evidence inspected

- Full diff `1dc4fca8..7a4118fc` and the three commits since round 01
  (`c0aeabef..7a4118fc`: pages removal, dependency pin to SKGO v0.32.0 /
  adapter 0.32.0, worklog and review notes).
- Head source: `internal/generate/appgen/{dev.go,dev_inputs.go,main.go}` and
  tests, `internal/generate/{stock.go,write.go,workflow_page.go,definition.go}`,
  `web/runtime.go`, `web/server.go`, `Justfile`, `README.md`, `go.mod`,
  `lefthook.yml` presence.
- Pinned modules as actually resolved by `go.mod` with no `replace` and no
  `go.work` (`go env GOWORK` is empty here):
  `$GOMODCACHE/github.com/tylergannon/skgo@v0.32.0` (`dev/dev.go`,
  `internal/dev/{server.go,reload.go,watch.go,inputs.go}`, `document.go`,
  `internal/gen/worker_projection.go`) and
  `$GOMODCACHE/github.com/tylergannon/polytype@v1.9.0` (`codegen/codegen.go`,
  `codegen/worker.go`, `polytype/main.go`, `internal/builder/output_plan.go`).
- Installed adapter `web/node_modules/@skgo/sveltekit-adapter/skgo-adapter/env.js`
  (supervisor reload endpoint, watcher hooks).
- Proof: `/tmp/gimbal-published-{browser,dev,dependency-tag,native-config,tests,
  vet,lint,web-check,web-test,build,format}.log`, harnesses
  `/tmp/gimbal-published-{browser,dependency-tag,native-config}.cjs`, and the
  screenshots `/tmp/gimbal-published-{hmr,api,diagnostic,repair,dependency-tag}.png`,
  each opened and compared with the log it belongs to.
- PR draft `/tmp/gimbal-warm-dev-pr-body.md`; worklog
  `ephemeral/worklog/gimbal-warm-dev.md`.
- Commands run by this review against the committed module graph
  (`GOWORK=off`, no replacements): `go build ./...` (only the five
  `plugins/*` packages fail with "function main is undeclared", which is the
  pre-existing `-buildmode=plugin` layout and untouched by this branch; `just
  build` builds `./cmd/gimbal`), `go vet ./internal/generate/... ./workflow/...
  ./web/...` clean, and `go test -count=1 ./internal/generate/... ./workflow/...
  ./internal/invocation/...` all passing (`internal/generate` 25.3s,
  `appgen` 3.3s). `go tool polytype --typescript <scratch>` from `workflow/`
  to compare the CLI's output with the committed `web/src/lib/workflow/types.ts`.
  `git status` clean before and after.

## Status of round-01 findings

1. **Head did not build from its own `go.mod`: fixed.** `go.mod` pins SKGO
   v0.32.0 and Polytype v1.9.0, `go.sum` carries only those, no `replace`,
   no `go.work`. Build, vet and tests pass here with `GOWORK=off`; the
   worker's `/tmp/gimbal-published-{tests,vet,lint,build}.log` agree (full
   `go test ./...` green, vet and lint logs empty).
2. **Browser proof ran against frozen producers: fixed.** The published-pin
   run exists (`/tmp/gimbal-published-dev.log`, 03:24–03:29). Its stats lines
   carry `TypeScriptHits`/`CodecHits`, fields that exist in
   `skgo@v0.32.0/internal/gen/worker_projection.go:15`; no workspace is active
   in this environment, so `go run ./internal/generate/appgen dev` resolves
   the pins. The scripts assert eight literal rows after every change, check
   `badge: string` in the generated route `types.ts`, drive a real compile
   error through both the Vite overlay and the standalone diagnostic page
   (`HTTP_ERROR 500` then `skgo-dev-diagnostic`), and restore sources in
   `finally`. Screenshots match the logs (API badge on all eight rows,
   overlay with `gimbalProofBroken`, repaired literal on all eight rows,
   `TAG ALTERNATE LITERAL` on all eight rows, HMR literal in the heading).
3. **Dead `internal/builtin/pages` output: fixed.** Writer and eight JSON
   files are gone (`git ls-files internal/builtin/pages` is empty, no
   references remain), and `TestStockRecoversMissingAndStaleOutput` now fails
   if the directory reappears (`stock_test.go:102`).
4. **Serving gap and API-edit cost: open by decision; the round-01 causal
   claim was wrong.** See finding 1. The caller's correction is right: under
   `StopBeforeStart` the predecessor is already gone when the candidate
   starts (`server.go:504-523`), so reordering `net.Listen` and `NewHandler`
   in `web/runtime.go:189-210` only changes when "replacement ready" is
   logged; the assembly time stays on the save-to-visible path.
5. **Nitpicks: partly fixed, one was wrong.** The unused `output` buffer is
   gone (`dev.go` now returns `nil`). The round-01 claim that the startup
   subprocess is redundant was incorrect: the CLI (`polytype/main.go:88-98`,
   `builder.Run` with no output flags) selects JSON Schema and the Go codec
   and writes `workflow/polytype_gen.go` plus TypeScript, while the retained
   Worker is configured with `TypeScript` only (`dev.go:93-96`), which takes
   the TypeScript-only path at `codegen.go:159`. The caller's description is
   accurate. README drift on `dev-go` flags remains (nitpick 3).

## Findings

### 1. Issue: about one second of every visible reload is Gimbal's own startup ordering, and it is recorded only as an SKGO item

Published-pin measurements (`/tmp/gimbal-published-browser.log`, time from
the supervisor's settle `full-reload` frame to the navigation committing):

| edit | settle frame | navigated | gap | whole visible |
|---|---|---|---|---|
| Go body | 6,985 ms | 8,040 ms | 1,055 ms | 4,629 ms |
| API field | 14,659 ms | 15,669 ms | 1,010 ms | 7,158 ms |
| repair | 20,304 ms | 21,261 ms | 957 ms | 4,223 ms |

In `/tmp/gimbal-published-dev.log` the same gap is the second between
`replacement ready` and `gimbal: web application listening` on every warm
replacement (03:25:52→53, 03:25:59→26:00, 03:26:05→06, 03:26:12→13,
03:29:08→09, …) and five seconds on the cold start (03:24:59→03:25:04, while
Vite was still bundling dependencies). SKGO declares readiness on TCP accept
(`server.go:543`), Gimbal binds first (`web/runtime.go:189`) and only then
performs the Vite round-trips in `NewHandler` (`ReadDevManifest` at
`web/server.go:43` and `NewDevSSR` at `:94`, each awaiting Vite's `/info` and
the document templates, `skgo@v0.32.0/document.go:230-248,287`). The
browser's reload request sits in the accept backlog for that whole interval,
and the `skgo: rendering /workflows: context canceled` lines at 03:26:06 and
03:29:09 are requests dropped across it.

The caller is right that reordering `Listen`/`NewHandler` does not remove
this under exclusive replacement. What would remove it is Gimbal-side: the
catalog is acquired in `web/runtime.go:156-169` before `startWeb`, and the
handler needs nothing from the catalog to be assembled, so a candidate could
assemble the handler (all the Vite work) while the predecessor still serves,
and only then wait for the exclusive catalog. That requires the acquisition
to wait instead of fail (the worklog records it failing today), after which
`StopBeforeStart` is no longer needed and SKGO's default overlapped
replacement applies. That is a Gimbal design change, not something SKGO #311
can deliver alone; the API-edit cost (3.0–3.2 s `skgo=` with `TypeScriptRuns`
+1 on every API run, `TypeScriptHits` +9) is SKGO's and is correctly left in
#311.

Impact: roughly a fifth to a quarter of the body-edit loop the issue measures
is owned by Gimbal and has no Gimbal-side record. This does not invalidate
the correctness proof and the user has deferred remaining latency; the
material gap is that the deferral names only SKGO #311. Record the
Gimbal-side item (ordering plus waiting acquisition) where Gimbal work is
tracked, or in #311 with its owner named.

### 2. Nitpick: each `just dev` start rewrites committed generated TypeScript in a different formatting

`dev.go:50` runs `go tool polytype --typescript ../web/src/lib/workflow` on
every start, and the first `project` call has the Worker write the same
directory plus `web/src/lib/observation`. Both emit quoted property names;
the committed files are Prettier-formatted. Reproduced here: the CLI's
`types.ts` differs from the committed `web/src/lib/workflow/types.ts` in 562
lines, all of the form `"name": string;` versus `name: string;`. So every
development session dirties committed files until `vp fmt` runs; the worker's
`/tmp/gimbal-published-format.log` (03:30, "Finished in 631ms on 137 files")
is that pass after the proof run. The previous `web/dev/run.mjs` did the same
through `go generate ./workflow`, so this is not a regression, and
`lefthook.yml` normalises before commit. It is still noise on every start;
writing the Worker's output through the same formatter, or committing the
unformatted form for these two directories, would remove it.

### 3. Nitpick: restart inputs and startup regeneration have two small blind spots

- `generatorFiles` (`dev_inputs.go:40`) adds `<pkg>/*.go` for every
  main-module package the generator compiles, which includes `_test.go`
  files (`workflow/roles_test.go`, `internal/generate/*_test.go`). Editing a
  test file raises the "Generator implementation changed" diagnostic
  (`server.go:451-452`) although the compiled generator is unchanged.
- The startup CLI rewrites `workflow/polytype_gen.go` after `go run` has
  already compiled the supervisor and before `Run` takes the restart
  fingerprint (`server.go:226`). If the committed codec were stale relative
  to `workflow/*.go`, the supervisor would run the old codec, regenerate the
  new one, and never ask for a restart. Contrived, since the codec is
  committed and regenerated by hooks, but it is the one restart input the
  supervisor itself mutates before fingerprinting.
- `README.md:352-353` still presents `just dev-web`/`just dev-go` as the
  same thing split in two; `dev-go` (`Justfile:31`) builds with default
  linker flags, so it produces a different binary from the supervisor's
  DWARF-less build and does not rebuild on edits.

### 4. Nitpick: a compile error stops the serving application, including in-flight runs

`/tmp/gimbal-published-dev.log` 03:26:0x: "the application did not build"
is followed by "stopping application pid 27549". That is SKGO's contract
(`settle` retires the previous generation for a failed build too,
`server.go:395-416`, and `watchExit` makes absence, not the last answer, what
requests see), and the proof shows the diagnostic page and automatic repair
working as designed. For Gimbal specifically, whose runs are long-lived agent
sessions, a typo during development terminates active runs rather than
leaving the last good build serving until the repair compiles. Nothing in
#311 asks for the alternative; this is worth a one-line note where Gimbal's
development behaviour is described (`README.md:342-350`) so it is not a
surprise.

## What holds up

- Generator/application decoupling, the `!jsonschema`/`jsonschema` registry
  split with its mutant-verified fixture, `writeGeneratedFile` with the
  fixed-mtime assertion, content-fingerprinted restart inputs, and preserved
  Svelte HMR (`generationCalls: 0`, document object retained across the
  edit) all stand as in round 01, now on published pins.
- The dependency/tag proof genuinely changes build-constraint selection in
  source (`//go:build gimbaldevproof` swapped between two files) and sees the
  alternate literal in all eight rows; the native-config proof removes one
  Swift remote and restores byte-identical output. Both ran against the
  published Polytype v1.9.0 builder that carries the build-constraint fix.
- The supervisor skips `_gen.go` and `_test.go` when fingerprinting
  (`watch.go:66`), so Stock's rewritten `internal/invocation/*_gen.go` after a
  workflow edit does not start a second cycle; the Polytype and SKGO writers
  are content-stable (`output_plan.go:149`, `cache_source.go:651`).
- The PR draft's numbers match the logs line for line (body 4.63 s with
  generation 409 ms and `PlanHits` +1; API 7.16 s with generation 3.23 s, one
  TypeScript run and nine hits; repair 4.22 s; 135 frontend tests), and it
  makes no subsecond or completed-#311 claim.
- The duplicate `full-reload` frames the PR mentions are real (4,122 and
  6,985 ms for the body edit; the first aborts a held navigation with
  `net::ERR_ABORTED`), originate in Vite/adapter watch handling rather than
  Gimbal code, and cost nothing beyond the gap in finding 1 because the held
  request would have waited in the same backlog.

## Outcome

material findings remain
