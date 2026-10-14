# Adversarial review: release critical path, round 03

Date: 2026-10-14 (local). Branch `codex/release-critical-path` at 9bd7c2dc plus four uncommitted changes (`auto-release.yml`, `release_timing.py`, `release_timing_test.py`, deleted `scripts/public-xcodebuild/xcodebuild`). PR #540 open and mergeable; its "Check release source" run succeeded on the committed tree.

## Target

Same authoritative sources as rounds 01 and 02: `AGENTS.md`, `skills/gimbal/SKILL.md` → `skills/gimbal-release/`, and the objective and decisions in `ephemeral/worklog/release-critical-path.md` (main-to-feed/Homebrew publication under ten minutes aiming for five, validation and Mac usability preserved; tag reserved before the full suite; one shared preparation; concurrent platform packaging; full Ubuntu qualification of the versioned amd64 archive in its build job; publish gated on qualification and every platform; prepared inputs tied to the exact revision including deletions; Mac work bounded to four workers; app bundles the versioned distributed CLI; `skgo native build` prepare-only; completion requires a published release with measured timings, all required checks, and observed Mac resource behavior).

Caller constraints honored: read-only except this artifact. The caller supplied evidence for adjudicating round 02 issues 1 and 2; that evidence was verified against the tree rather than accepted, and it did not narrow the review scope.

## Evidence inspected

- `git diff HEAD` (the four uncommitted changes) and `git diff main...HEAD`; the full current `.github/workflows/auto-release.yml` and `.github/workflows/release.yml`; `scripts/dispatch-release.py`; `scripts/pre-commit.py` (`smoke`, `--warm`); `scripts/fast_hook_test.py` and `scripts/staged_hook_test.py`; `scripts/release_timing.py` (`measure`, `ensure_issue`); `scripts/release_timing_test.py`; the worklog.
- `git grep public-xcodebuild` outside `ephemeral/`: no references remain.
- Ran `python3 -B -m unittest discover -s scripts -p '*_test.py'` on the working tree: 56 tests OK in 14.8s (this includes `FastHookTest.setUpClass` running `pre-commit.py --warm`, which executed real `go test` and `node --test`).
- `gh pr view 540`: open, mergeable, "Check release source" SUCCESS, "Tag and publish checked source" SKIPPED (PR event). `gh run list -w release.yml`: the three most recent runs (v0.13.11–v0.13.13) are all pre-branch `workflow_dispatch` runs on tag refs. `gh release list`: v0.13.13 is latest.
- Round 01 and round 02 findings re-verified against the current tree (below).

## Adjudication of round 02

### Issue 1 (new pipeline never run; a dry run against `v0.13.13` was available)

Caller's evidence is correct and the round 02 recommendation was wrong on the mechanism:

- `release.yml` checks out `${{ inputs.tag || github.ref_name }}` in `prepare-inputs`, both `linux-cli` matrix jobs, `macos-release` and `publish`. A dispatch of this branch's ref with `inputs.tag=v0.13.13` builds the v0.13.13 tree, which has no `just build-assets` target and no `scripts/release-assets.py`; the `prepare-inputs` step would fail immediately and exercise nothing new.
- `macos-release` has `if: github.ref_type == 'tag' && startsWith(github.ref_name, 'v') && (!inputs.tag || inputs.tag == github.ref_name)`. A dispatch from a branch ref has `ref_type == 'branch'`, so the Mac job is skipped and `publish` (which `needs` it) is skipped with it. No branch-ref dispatch can exercise the Mac path.
- `dispatch-release.py` dispatches with `ref=<tag>` and `inputs[tag]=<tag>`, so the only way the new workflow runs is on a tag whose tree contains it, which is the normal post-merge candidate run.

The substance of the finding (the completion criterion is unmet because no run of the new pipeline exists) remains true, and the worklog does not claim completion: its own criterion still requires the published release with measured timings and observed Mac resource behavior. Publication by that run is gated on full amd64 qualification plus every platform job plus signing/notarization, exactly as the previous pipeline was; a first-run failure leaves a reserved tag and changelog commit whose recovery is the documented re-dispatch. Adjudication: **not a defect; reclassified as a proof status note.** It is recorded under Notes and does not count toward the outcome.

### Issue 2 (`auto-release.yml` installs Go, pnpm and Node on main pushes for nothing)

Caller's evidence is correct and round 02 overstated the finding:

- `FastHookTest.setUpClass` runs `python3 -B scripts/pre-commit.py --warm` with `check=True`; `smoke()` runs `go test -mod=readonly … ` over the smoke packages and `node --test web/tests/workflow/view-state.test.mjs`. The "Check release automation and hooks" step therefore needs real Go and Node on every push where `release == 'true'`, including main. The only `pnpm` mentions in the script tests are stub executables that must *not* be called.
- Before the uncommitted change, that step ran before `setup-node`, so on main it depended on the runner's preinstalled Node; the change moves the step after `setup-go`/`setup-node`, which is a correctness improvement, not just a reordering.
- `pnpm/action-setup` is now PR-only and gated on web scope; `setup-node` on main runs with `cache: ''` (no pnpm store restore). `cache-dependency-path` with an empty `cache` is ignored by the action.

Adjudication: **resolved.** The remaining main-path cost (setup-go, setup-node without cache) is required by the tests that run there.

## Round 02 nitpicks

3. `scripts/public-xcodebuild/xcodebuild`: deleted in the working tree; no references remain outside `ephemeral/`. Resolved.
4. Timing fixture: `release_timing_test.py:136` now names `CLI (linux_amd64)`, and `ensure_issue` states that the Linux amd64 job time includes full candidate qualification. `parallel_platform_critical_path_seconds` still sums per-platform maxima by `CLI (` prefix, so it includes qualification minutes; the issue body now says so. Resolved.

## Findings

No material findings. One genuine nitpick:

### 1. Nitpick: the four fixes are uncommitted, so PR #540's green check does not cover them

`git status` shows `auto-release.yml`, `release_timing.py`, `release_timing_test.py` modified and the xcodebuild shim deleted, none staged. The "Check release source" SUCCESS on #540 ran against 9bd7c2dc, which still had the pre-reorder `auto-release.yml`. The reordered workflow (Node set up before the hook tests, pnpm PR-only) has not run in CI yet; it is exercised here only by the local 56-test run. `AGENTS.md`: commit and push whenever something interesting has happened. Impact: low; the change is small and reads correctly, and the PR check will re-run on push.

## Notes (not findings)

- Proof status: the completion criterion in the worklog (published release with measured timings, all required checks, observed Mac resource behavior) is unmet by construction until the first post-merge candidate run. Nothing in the tree can produce that proof before merge, for the reasons in the issue 1 adjudication. Whoever merges should watch that run and record the measurement in the worklog.
- `setup-node@v4` with `cache: ''` is the documented way to disable caching; the expression `${{ cond && 'pnpm' || '' }}` yields `''` when `cond` is false because the `||` operand is a string.
- On main, `setup-go` still uses `check-latest: true`, which may differ from the toolchain the hook smoke tests saw locally. The tests run `-mod=readonly`, so a toolchain change would surface as a clear failure rather than a silent difference.
- All round 01 findings (skgo flags, Linux extraction, Python 3.9 restore, documentation, bundled-CLI precheck) remain fixed in the current tree; nothing in the uncommitted diff touches those paths.

## Outcome

only nitpicks remain
