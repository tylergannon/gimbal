# Issue 611 release follow-up: adversarial review, round 02

## Target

Everything on `codex/issue-611-release-fix` since `955b7e02` (`chore: release
v0.16.0`): commit `54645f6a` (`fix: restore executor release qualification`,
12 files) plus the uncommitted worktree changes (`command_test.go` and three
generated TypeScript files under `web/src/lib/`). Authoritative sources, as in
round 01: `/tmp/issue611-release-fix.md`, the qualification log
`/tmp/issue611-release-qualification.log`, the merged PR text
`/tmp/issue611-pr.md`, AGENTS.md and `docs/definition-of-done.md`. The current
PR description and proof are `/tmp/issue611-release-fix-pr.md`.

No caller narrowing was applied beyond the read-only boundary and the
artifact path.

## Evidence inspected

- Every hunk of `54645f6a` and of the uncommitted diff, with the surrounding
  code for each changed return, the `dispatch` switch in
  `internal/effects/connection.go`, and the full `runCapturedCommand` path in
  `command.go` through `commandexecutor.Submit`, `Run` and `executor.Launch`.
- `golangci-lint run ./...` at the pinned **v2.14.0** on the current tree
  (committed plus uncommitted): **0 issues**. This is the gate that failed.
- `go vet ./...`: clean.
- `go test -count=1 -race -run TestRunCommand .`: pass. The two rewritten
  tests run twice warm: `TestRunCommandRecordsEachOutcome` 2.5 s to 2.6 s,
  `TestRunCommandThatExitedKeepsItsExit` 0.09 s to 0.10 s.
- `TestLocalStatePreservesPendingCleanup`: pass.
- `go test -count=1 ./internal/generate/... ./internal/devgenerate/...` with
  the three generated TypeScript files backed up first: pass, and none of the
  three files was rewritten (round 01's note about tests rewriting them did
  not reproduce; `git status` was identical before and after).
- `cd web && vp fmt --check`: clean on the worktree. The same check on the
  HEAD (compact) form of `observation/types.ts`, and on fresh
  `go tool polytype --typescript` output, reports format issues. So HEAD as
  committed fails `just fmt-check`, and the uncommitted formatted files are
  what makes it pass.
- Release CI build log (run 39181179149, "Prepare shared release inputs"):
  `go tool skgo build` runs the frontend formatter over 168 files before
  packing, so the qualify job sees formatted files regardless of which form is
  committed. The formatted worktree files therefore match what CI will check.
- Frontend tests: the globally installed `vp` fails at startup
  (`vite_ssr_environment_not_runnable`); the project-local
  `web/node_modules/.bin/vp test --run` passes 44 files, 201 tests, as the
  PR text says. `vp run check` (svelte-check): exit 0.
- `gh run list --workflow=release.yml`: every release since v0.15.2 failed in
  "Qualify release source" at `lint-go`, so `just test`, `just fmt-check` and
  `just check-web` have never run on the CI runner. The author's local runs
  are the only evidence those three pass; I reproduced them above.
- Issue #614 exists for the v2.15 errcheck forward risk from round 01.

## Status of round 01 findings

1. `ineffassign` fix and regression test: unchanged, still correct.
2. Cleanup-only versus durable returns: unchanged, still correct.
3. `writeWorkflowPages` removal: unchanged.
4. Lint v2.15 forward risk: filed as #614. Closed.
5. Proof report: the PR text now reports the author's own runs (v2.14.0
   lint zero issues, vet, full `go test`, race on affected packages, the
   negative run of the new test, 201 frontend tests, fmt and svelte checks).
   Closed.
6. `command_test.go` timeout: now committed with a comment and explained in
   the PR text as an observed startup cost. See finding 1 for what remains.

## Findings

### 1. `TestRunCommandThatExitedKeepsItsExit` no longer tests cancellation (nitpick)

Uncommitted `command_test.go:219-220`: the 300 ms `AfterFunc` cancel is
replaced by `cancel()` after `RunCommand` returns. Because `runCapturedCommand`
has already read `ctx.Err()`, waited, and built `ended` by then, a cancel after
return cannot influence the result. The test still proves something useful:
a command whose background child (`sleep 1 &`) holds stdout open returns at
exit in about 100 ms with exit 7 and no interruption, i.e. the executor does
not block on the inherited pipe. But the new doc comment ("keeps its exit
status when its ctx is cancelled after completion") describes a tautology. If
the original intent (ctx ends while the leftover child still holds the pipe)
is worth keeping, the deterministic form is to cancel once `stdout.log`
contains `done`, not on a timer. Not material: the behavior under test is
unchanged and the PR text states the reason honestly.

### 2. Generated files are committed in a form their generator does not emit (nitpick, pre-existing direction flipped)

The three uncommitted TypeScript files are the formatter's output over
polytype/devgen output. `go tool polytype --typescript` writes the compact
form; `just build-assets` then formats it. Committing the formatted form makes
`just fmt-check` pass on a plain checkout (today it fails, see evidence), but
`just dev`'s `beforeBuild` polytype step will dirty the tree with the compact
form on every dev session. Either committed form has churn in one direction;
the formatted form is the one CI checks, so committing it is the better of the
two. The real fix is for the generator to format its own output, which is out
of this follow-up's scope. Mentioning it so the "formatter output only" line
in the PR text is understood as a deliberate choice, not noise.

### 3. The pending-cleanup consequence is still untested on the local path (nitpick, carried from round 01)

`internal/runner/task_cleanup.go` skips `Cleanup` when a local `state` call
reports `Pending == 0`. The new effects test covers the dispatch regression
the issue names; the decision that consumed the wrong value has no local-path
test. Acceptable for this follow-up; noting so the new test is not mistaken
for coverage of that branch.

## Repository-rule check

- Conventional Commit title `fix:` on the commit; the patch release path is
  selected. Pass.
- No new exported names, no wrappers, no shims; `ephemeral/` receives a
  worklog line and this review. Pass.
- Proof is the author's own report in the PR text, reproduced here. Pass.
- No live model runs were needed; the change touches no execution path
  beyond error handling and the restored `state` reply.

## Outcome

`only nitpicks remain`. Nothing blocks committing the `command_test.go` and
generated TypeScript changes alongside `54645f6a` and merging.
