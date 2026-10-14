# Adversarial review: Gimbal warm development, round 04

Date: 2026-10-14. Worktree `/Users/tyler/src/gimbal-worktrees/gimbal-warm-dev`,
branch `codex/gimbal-warm-dev`, base `1dc4fca8`, head `d7f65586` (nine
commits; one commit, `d7f65586`, since round 03). PR #534 head is the same
SHA. Read-only review; only this file was written. Scratch output went to the
session scratchpad, not `/tmp`.

## Review target and scope

Target: the complete Gimbal integration for SKGO issue #311 as it stands at
head, judged against the issue text (fetched live from `tylergannon/skgo#311`
including all seven comments), `AGENTS.md`, `docs/definition-of-done.md`,
the round-01 to round-03 reviews, PR #534's description as currently
published, Gimbal #535, SKGO PR #319, and the published-dependency proof the
launch prompt named: `/tmp/gimbal-corrected-browser.cjs`,
`/tmp/gimbal-corrected-browser.log` and `/tmp/gimbal-corrected-dev.log`.

The launch prompt named the target, the proof files, the artifact path and
the read-only constraint. It did not narrow scope and none was applied.

## Evidence inspected

- The one commit since round 03, `d7f65586`: `go.mod`/`go.sum` move SKGO
  from v0.32.0 to v0.32.2 (the `/go.mod` hash is unchanged, so SKGO's own
  module file did not change); `web/package.json`, `web/pnpm-lock.yaml` and
  `web/pnpm-workspace.yaml` move the adapter from 0.32.0 to 0.32.1; the
  round-03 review and one worklog line are added. No Go or Svelte source
  changes. `web/node_modules/@skgo/sveltekit-adapter/package.json` reports
  0.32.1, matching the lock.
- Head source again: `internal/generate/appgen/{dev.go,dev_inputs.go}`,
  `web/runtime.go:183-245`, `web/server.go:86-104`, `Justfile`,
  `README.md:338-357`. None changed since round 03.
- Pinned SKGO as resolved by `go.mod`, in `$GOMODCACHE`: a recursive diff of
  `skgo@v0.32.0` against `skgo@v0.32.2` (non-test files) touches only
  `internal/dev/{digest.go,server.go,watch.go}` (the #319 correction:
  `fileDigests` and the three platform `digest_*.go` files are deleted;
  every freshness check now opens and hashes the file through a scan-local
  buffer), `internal/gen/cache_source.go` (#318, the ownership-predicate
  reorder) and `internal/adapter/skgo-adapter/{entry.js,env.js}` (lazy dev
  SSR from #316: the generated node table becomes `componentLoaders` and
  `universalLoaders` thunks, awaited per rendered branch). The SKGO compare
  `v0.32.0...v0.32.2` lists exactly those three commits.
- GitHub state read live: PR #534 (body as quoted below, no reviews, no
  comments, mergeable; "Check release source" passed on `d7f65586` in
  3m48s, "Tag and publish" skipped as expected); Gimbal #535 (unchanged
  since round 03, no comments); SKGO #311 with its two newest comments
  (v0.32.2 publication and the Gimbal re-qualification table); SKGO PR #319
  (merged 10:13Z, merge `a7f15469`); an SKGO issue search for
  "digest", "timestamp", "freshness", "Linux" (no new issue; the defect
  went straight to #319).
- Proof. `/tmp/gimbal-corrected-browser.cjs` (03:57) is the harness;
  `/tmp/gimbal-corrected-{generate,format,tests,build}.log` (04:27-04:28),
  the four `gimbal-corrected-*.png` screenshots (04:29:00-04:29:18),
  `gimbal-corrected-browser.log` (04:29:18), `gimbal-corrected-dev.log`
  (04:29:35) and `gimbal-corrected-final-format.log` (04:29:36) are one
  run, finished 53 seconds before commit `d7f65586` (04:30:28). The dev log's
  "Re-optimizing dependencies because lockfile has changed" shows the run
  used the new adapter lock; the Go pin in force at run time cannot be read
  from the logs, so this review re-ran the Go gate on the committed pin
  (below). All three R2 screenshot URLs in the PR body and the earlier
  dependency/tag URL return 200. The three corrected screenshots were
  inspected: API and repair show the Workflows page with eight rows each
  carrying the literal in question; the diagnostic shows the SKGO "did not
  build" page beneath the Vite overlay quoting `undefined: gimbalProofBroken`
  at `page.server.go:20:101`, matching the dev log.
- Harness read line by line. It restores the Go file's mtime after the body
  edit (`fs.utimesSync` with the pre-edit stat), which is the #319 case;
  requires exactly eight matching rows after body, API and repair edits
  (`count8`); checks `badge: string` literally in the generated
  `types.ts`; requires zero `gimbal dev generation:` lines during the
  frontend HMR step, so it relies on `GIMBAL_DEV_TRACE` being set; navigates
  fresh to the diagnostic and asserts its text; and restores both edited
  files in `finally`. It logs `pageerror`, failed requests, 4xx/5xx
  responses and Vite `full-reload`/`error` frames, but does not watch
  browser console warnings.
- Every number in the PR body and the latest #311 comment was checked
  against the logs: visible 5588/7230/3833 ms, generation 541/3209/398 ms,
  compile 2901/1592/1482 ms, TCP-ready 795/851/836 ms, HMR 791 ms with zero
  generation calls, `PlanHits` 1 to 2 on the body edit with
  `TypeScriptRuns` and `CodecRuns` unchanged, then `TypeScriptRuns` 10 to 11
  with `TypeScriptHits` 9 and `CodecHits` 1 on the API edit. All match.
- Commands run by this review with `GOWORK=off` against the committed
  module graph: `go vet ./...` (exit 0) and `go test -count=1 ./...` (exit
  0, 62 packages ok, none failed). `go build ./...` exits 1 only for the five
  `plugins/*` packages, which are `-buildmode=plugin` mains without `main`
  (`plugins/auth-files/main.go:1`), untouched by this branch and not built
  that way anywhere in the repository; `./cmd/gimbal` and every other
  package build. `git status` clean before and after; `go env GOWORK` empty.

## Status of round-03 findings

1. **Linux watcher hold: lifted.** The defect was corrected by SKGO PR #319
   (merged, in v0.32.2) rather than by an issue; `go.mod` pins v0.32.2; the
   PR body replaces the prose hold with the #319 reference and the pin; the
   newest #311 comments record the publication and Gimbal's re-qualification.
   The pinned `digest.go` reads bytes on every check, and the harness's
   mtime-restoring body edit was seen reloading. Closed.
2. **`_test.go` files as restart inputs: unchanged** (nitpick 2 below).
3. **TypeScript formatting churn on every start: unchanged** (nitpick 3
   below). The run again ended with a `vp fmt` pass
   (`gimbal-corrected-final-format.log`, 137 files) and both
   `web/src/lib/{workflow,observation}/types.ts` carry its 04:29:35 time.
4. **Lazy SSR review notes for another repository: superseded.** The branch
   now consumes that change through v0.32.2 and adapter 0.32.1, so the notes
   at least concern a dependency Gimbal runs. What remains is that the
   assessment those notes were meant to precede has not happened; see
   finding 1.

## Findings

### 1. Issue: the adapter 0.32.1 run logs a Svelte hydration mismatch that nothing examined, and the lazy SSR assessment #535 defers is still owed

`/tmp/gimbal-corrected-dev.log` records, right after the API-edit
replacement came up:

```
2026/10/14 04:29:11 gimbal: web application listening on 127.0.0.1:59701 (dev)
4:29:12 AM [vite+] (client) [console.warn] [svelte] hydration_mismatch
Hydration failed because the initial UI does not match what was rendered on the server
```

- The matching run on v0.32.0 / adapter 0.32.0 (`/tmp/gimbal-published-dev.log`,
  same harness steps, lines 41-53) has no such line; `grep -l
  hydration_mismatch /tmp/gimbal-*.log` matches only the corrected log. The
  only change between the two runs is the dependency pin, and the only
  adapter change in that pin is lazy dev SSR (`entry.js`, `env.js`).
- Svelte recovers from a hydration mismatch by discarding the server DOM and
  rendering client-side, so the harness's eight-row assertion and the API
  screenshot pass either way. The run therefore does not establish that the
  document the Go dev SSR produced after an API edit matched the client, and
  the harness could not have caught it: it watches page errors, failed
  requests, 5xx responses and Vite frames, not console warnings.
- Neither the PR body ("Images were inspected") nor the #311 comment nor the
  worklog mentions it. Gimbal #535 still says "Assess the newer lazy SSR
  release separately; PR534's current evidence is 0.32.0", which is now
  false on both counts: the evidence is 0.32.1 and no assessment exists.
  The same issue's observed-cost section ("Cold Vite startup added about 5s
  between control/socket readiness and actual serving") is also stale: the
  corrected cold start shows the control socket at 04:28:41 and web serving
  at 04:28:42, consistent with the renderer no longer evaluating every route
  at startup.

This review could not reproduce or diagnose the warning without starting a
supervisor, which is outside its remit. The cause may be benign (the
harness writes `page.server.go` and `+page.svelte` in the same tick, so the
client HMR'd the component with `badge` undefined five seconds before the
Go replacement served the field) or a lazy-SSR module freshness defect.
Either way `docs/definition-of-done.md` requires that a remaining quirk be
filed, and `AGENTS.md`'s proof rule requires saying what was seen. Record
it: either explain it in the PR body with a reproduction that shows the
server and client markup agreeing, or file it (Gimbal or SKGO, whichever
the diagnosis points at), have the harness fail or at least log on
`hydration_mismatch`, and update #535's lazy-SSR sentence and cold-start
number.

### 2. Nitpick: `_test.go` files remain generator restart inputs

Unchanged from rounds 02 and 03. `generatorFiles` (`dev_inputs.go:42`)
adds `<pkg>/*.go` for every main-module package the generator compiles, so
editing `workflow/roles_test.go` or `internal/generate/*_test.go` raises the
restart requirement although the compiled generator is unchanged.

### 3. Nitpick: every `just dev` start still rewrites committed TypeScript in a different formatting

Unchanged from rounds 02 and 03; the worker's own run again needed a
closing `vp fmt` (`gimbal-corrected-final-format.log`). Not a regression
over `web/dev/run.mjs`, and `lefthook.yml` normalises before commit.

### 4. Nitpick: `GIMBAL_DEV_TRACE` is load-bearing for the proof and undocumented

The dev log's `gimbal dev generation:` lines, which the harness counts to
prove zero generation calls during frontend HMR and which supply every
generation figure in the PR body, print only when `GIMBAL_DEV_TRACE` is set
(`dev.go:77`). `README.md:343-345` documents `GIMBAL_DEV_PORT` and
`GIMBAL_DEV_VITE_PORT` but not this variable, so a developer reproducing the
PR's numbers from the README gets no timings.

### 5. Nitpick: the PR body understates its own Go gate on the final pin

The body says the v0.32.0 consumer passed `go test ./...` and `go vet ./...`
and that the v0.32.2 pin "additionally passed normal generation, frontend
build, appgen/invocation tests" (`gimbal-corrected-tests.log` holds exactly
those two packages). Full `go vet ./...` and `go test -count=1 ./...` on the
committed pin pass (this review, above) and CI passed on the head SHA, so
the claim can simply be made.

## What holds up

- Head vets and tests clean from its own `go.mod` with `GOWORK=off` on the
  v0.32.2 pin; `./cmd/gimbal` builds; PR #534's check passed on `d7f65586`;
  the tree is clean and no `replace` or workspace is in effect.
- The pin is exactly what the PR and the #311 comments say: SKGO v0.32.2,
  adapter 0.32.1, Polytype v1.9.0, all from the public proxy and registry
  (`go.sum` hashes present, pnpm lock integrity updated, installed package
  0.32.1).
- The #319 correction is in the pinned bytes and is what the round-03
  finding asked for: no metadata-authorised reuse remains anywhere in
  `internal/dev`. The harness's body edit deliberately restores the
  original mtime and was seen reloading into eight updated rows in 5.59 s,
  with the extra cost visible in the compile phase (2.90 s versus 1.59 s and
  1.48 s for the other builds), not hidden in generation.
- Every figure in the PR body and the #311 re-qualification table matches
  the raw logs, the three screenshots show what the body says they show,
  and the duplicate `full-reload` frame cancelling an in-flight navigation
  (`ERR_ABORTED` at 13165 ms and 25713 ms) is reported honestly as
  remaining latency rather than hidden.
- The startup codec guard (`dev.go:153-169`) still does not fire on a clean
  tree: `workflow/polytype_gen.go` was written by the 04:28:34 startup and
  `git status` is clean, so the CLI's output equals the committed codec.
- Everything rounds 01 to 03 listed as holding up (generator and
  application decoupling, the `!jsonschema`/`jsonschema` registry split with
  its mutant-verified fixture, content-stable generated writes, preserved
  Svelte HMR with zero generation calls, the dependency/tag and
  native-config proofs, the #535 record of the handler-assembly gap) is
  untouched by the one commit since.

## Outcome

material findings remain
