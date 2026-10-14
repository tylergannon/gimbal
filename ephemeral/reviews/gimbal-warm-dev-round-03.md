# Adversarial review: Gimbal warm development, round 03

Date: 2026-10-14. Worktree `/Users/tyler/src/gimbal-worktrees/gimbal-warm-dev`,
branch `codex/gimbal-warm-dev`, base `1dc4fca8`, head `8eef21dc` (eight
commits; one commit, `8eef21dc`, since round 02). PR #534 head is the same
SHA. Read-only review; only this file was written. Scratch output went to the
session scratchpad, not `/tmp`.

## Review target and scope

Target: the complete Gimbal integration for SKGO issue #311 as it stands at
head, judged against the issue text (fetched live from `tylergannon/skgo#311`
including its five comments), `AGENTS.md`, `docs/definition-of-done.md`,
the round-01 and round-02 reviews, PR #534's description as currently
published, and Gimbal issue #535 as currently published.

The launch prompt named the target, the artifact path and the read-only
constraint. It did not narrow scope and none was applied.

## Evidence inspected

- Full diff `1dc4fca8..8eef21dc`, with particular attention to
  `7a4118fc..8eef21dc`: `internal/generate/appgen/dev.go` (the new
  `regenerateCompiledWorkflow` guard), the new
  `internal/generate/appgen/dev_test.go`, `README.md:351-357`, and the two
  worklog lines added to `ephemeral/worklog/gimbal-warm-dev.md`.
- Head source: `internal/generate/appgen/{dev.go,dev_inputs.go}`,
  `web/runtime.go:150-245`, `Justfile`, `go.mod` (SKGO v0.32.0, Polytype
  v1.9.0, no `replace`; `go env GOWORK` is empty here).
- Pinned SKGO as resolved by `go.mod`:
  `$GOMODCACHE/github.com/tylergannon/skgo@v0.32.0/internal/dev/{digest.go,
  digest_linux.go,digest_darwin.go,digest_other.go,watch.go,server.go}`.
- GitHub state read live: PR #534 (body, no reviews, no comments, mergeable,
  check run "Check release source" succeeded at 09:48Z and "Tag and publish"
  skipped as expected for an unmerged PR); Gimbal #535; SKGO #311 and its
  comments; SKGO PR #318 (open), PR #316/#317 (merged), closed SKGO #265;
  SKGO tags `v0.32.1` and `v0.32.0`, and the `v0.32.0...v0.32.1` compare
  (one commit, the lazy dev SSR change, adapter files and one test only);
  issue searches across `tylergannon/{skgo,polytype,gimbal}` for
  "Linux", "timestamp", "ctime", "digest", "freshness", "collision",
  "stale bytes", "watcher".
- Proof: `/tmp/gimbal-published-*` listing with modification times. Every
  log, screenshot and harness there is from 03:22–03:33 local; nothing under
  `/tmp/gimbal-*` is newer than 03:40, so no supervisor or browser run
  exists after commit `8eef21dc` (03:48). The round-02 inspection of those
  logs and screenshots stands and was not repeated.
- Commands run by this review with `GOWORK=off` against the committed
  module graph: `go build ./cmd/gimbal ./internal/... ./web/... ./workflow/... .`
  (exit 0), `go vet ./internal/generate/... ./web/... ./workflow/...` (exit 0),
  `go test -count=1 ./internal/generate/... ./workflow/...` (all pass;
  `internal/generate` 23.1s, `appgen` 1.6s including the new test).
  `gofmt -l workflow/polytype_gen.go` empty. `git status` clean before and
  after.
- `workflow/polytype_gen.go` has modification time 03:36:06, between the
  published-pin run and the guard commit, with `git status` clean: the
  startup CLI's output equals the committed codec, so the new guard does not
  fire on a clean tree. This is the check round 02 nitpick 3 implied and the
  worker's unit test does not perform.

## Status of round-02 findings

1. **Gimbal-owned serving gap: recorded.** Gimbal #535 exists, names the
   `web/runtime.go` ordering (`net.Listen` at `:189` before `NewHandler` at
   `:206`), states correctly that moving the listen call alone does not
   remove assembly from the critical path under `StopBeforeStart`, names
   waiting acquisition as a candidate without adopting it, and sets a
   phase-separated proof requirement. PR #534 and SKGO #311's latest comment
   link it. The round-02 finding is closed.
2. **TypeScript formatting churn on every start: unchanged** (nitpick 2
   below).
3. **Restart-input blind spots: two of three fixed.** The startup codec
   race is fixed by `regenerateCompiledWorkflow` (`dev.go:153-169`): the
   codec bytes are compared around the CLI run, a change stops startup with
   an explicit restart message and leaves the repaired file for the next
   compile, and `dev_test.go` covers changed, unchanged and failing
   regeneration. `README.md:355-357` now states that `dev-go` does not
   regenerate or rebuild. `_test.go` files are still restart inputs
   (nitpick 1 below).
4. **Failed build retires the serving application: documented.**
   `README.md:352-353` now says so.

## Findings

### 1. Issue: the merge-blocking SKGO watcher defect is recorded only in Gimbal prose, with no issue, owner or exit criterion

The last worklog line (`ephemeral/worklog/gimbal-warm-dev.md:21`) and the
final sentence of PR #534 hold this integration's merge for "newly reported
Linux kernel timestamp collisions" that "invalidate the released shared
watcher's freshness assumption and require a producer correction before
this integration merges".

The assumption is real and is in the pinned release. SKGO
v0.32.0 `internal/dev/digest.go:54-55` reuses a cached content digest when
ctime, mtime, size and inode all match, and `digest_linux.go:9-14` takes
ctime from `Stat_t.Ctim`. That cache feeds the authored-input
`fingerprint()` (`watch.go:29-32`), which is the per-request edit
detection the supervisor runs in development. The only published evidence
of the collision is one paragraph in SKGO PR #318's description: an
independent reproduction saw "stale bytes on Linux 6.12 despite matching
ctime/inode/size/mtime" for a *retained-byte cache that was rejected from
that PR*. PR #318 does not change `internal/dev/digest.go`, so the released
digest reuse keeps the same assumption the rejected cache failed on.

What is missing:

- No issue exists anywhere for the released defect. Searches of
  `tylergannon/skgo`, `tylergannon/polytype` and `tylergannon/gimbal` for
  Linux, timestamp, ctime, digest, freshness, collision, stale bytes and
  watcher return only the closed SKGO #265 (a different chokidar throttle
  defect) and the unrelated Gimbal watcher issues #324 and #448. SKGO #311's
  latest comment, written after PR #534, does not mention it.
- The hold therefore has no owner, no reproduction a producer can run, and
  no statement of what lifts it (a fixed SKGO version to pin, or a decision
  that Gimbal's macOS proof suffices). Note that the restart-input
  fingerprint is unaffected: `fingerprintPaths` (`watch.go:146-148`) uses an
  uncached `digestScan`, so `digest.go:58-66` always rereads bytes there.
  The exposure is per-request edit detection on Linux only, which the
  committed `README.md` and PR body do not say.

`docs/definition-of-done.md` ("Remaining quirks and bugs are filed as GitHub
issues") and `AGENTS.md` ("Information lands locally", an issue's text in a
file before an agent is pointed at it) both require this to be an issue,
and the branch already does exactly that for its other deferred item
(#535). Impact: the PR's own stated merge condition cannot be checked or
discharged by anyone reading the PR, the worklog or the SKGO tracker.
File the SKGO issue with the `digest.go:54-55` assumption, the Linux 6.12
reproduction from PR #318, and the unaffected restart-input path; link it
from PR #534 in place of the prose sentence and name the pin that lifts the
hold.

### 2. Nitpick: `_test.go` files remain generator restart inputs

Unchanged from round 02. `generatorFiles` (`dev_inputs.go:42`) adds
`<pkg>/*.go` for every main-module package the generator compiles;
`fingerprintPathsScan` globs it (`watch.go:158-163`). `workflow/roles_test.go`
and `internal/generate/*_test.go` therefore raise "Generator implementation
changed" (`server.go:451-452`) when edited, although the compiled generator
is unchanged. The supervisor's own authored-input walk already skips
`_test.go` (`watch.go:66`); the restart list should too.

### 3. Nitpick: every `just dev` start still rewrites committed TypeScript in a different formatting

Unchanged from round 02. The startup CLI (`dev.go:51`) and the first
`project` call write quoted-property TypeScript into
`web/src/lib/{workflow,observation}`; the committed files are
Prettier-formatted. In this worktree both `types.ts` files carry the
03:30:15 modification time of the worker's `vp fmt` pass after the proof
run, which is the cleanup every developer repeats. Not a regression over
`web/dev/run.mjs`, and `lefthook.yml` normalises before commit.

### 4. Nitpick: the branch carries review notes for another repository's pull request

Commit `c9bac8a1` adds `ephemeral/reviews/lazy-ssr-independent-round-0{1,2}.md`,
two independent reviews of SKGO PR #316 (lazy dev SSR) performed in
`/Users/tyler/.codex/worktrees/skgo-lazy-dev-ssr`. Nothing in this branch
consumes that change: Gimbal pins v0.32.0 and v0.32.1 (the tag that carries
#316, one commit, adapter files only) is explicitly deferred by #535
("Assess the newer lazy SSR release separately"). `ephemeral/` may hold
notes, so the hook accepts them, but they are unrequested content in a PR
titled for Gimbal's supervisor integration and belong with the SKGO work
they review.

## What holds up

- Head builds, vets and tests from its own `go.mod` with `GOWORK=off`; PR
  #534's "Check release source" check passed on this SHA.
- The new startup guard is correct for its stated purpose: the comparison
  brackets the CLI run, a missing codec before regeneration counts as a
  change (so a first compile never runs an absent codec), the repaired file
  is retained, the CLI's own error is returned unchanged, and the committed
  codec equals the CLI's output so a clean tree starts without a spurious
  restart. The restart fingerprint SKGO takes at `server.go:226` is now
  taken after the only startup mutation of a restart input.
- The `8eef21dc` change is startup-only and does not touch the supervisor,
  the generation callback, the browser path or the dependency pins, so the
  round-02 inspection of the published-pin proof
  (`/tmp/gimbal-published-{dev,browser,dependency-tag,native-config}.log`
  and the five screenshots) remains valid for this head. The PR body's
  numbers and claims are unchanged from the version round 02 checked line
  by line, with the two added sentences (guard and merge hold) accurate as
  far as they go.
- Gimbal #535 is a faithful record of round-02 finding 1, including the
  correction that reordering `net.Listen` alone does not help, the
  measured phases, and the constraints (one owner, no OS process discovery,
  no bespoke supervisor).
- Everything round 01 and round 02 listed as holding up (generator and
  application decoupling, the `!jsonschema`/`jsonschema` registry split with
  its mutant-verified fixture, content-stable generated writes, preserved
  Svelte HMR, the dependency/tag and native-config proofs on the published
  Polytype v1.9.0 builder) is untouched by the one commit since.

## Outcome

material findings remain
