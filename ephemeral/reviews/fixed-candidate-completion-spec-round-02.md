# Fixed candidate completion: specification review, round 02

## Target

The current `ephemeral/plans/fixed-candidate-foundation.md`, for finishing draft PR #664 (`codex/fixed-release-candidate`, head `17deb5ba`). Preapproval, read-only. Nothing was executed.

Ignored narrowing: none. The caller supplied only read-only and artifact-path constraints.

## Evidence inspected

- The plan.
- `ephemeral/worklog/fixed-candidate-completion.md`.
- `candidate-exchange.md` in the design worktree. It was readable this round; round 01 could not read it.
- Round 01 review.
- `.github/workflows/release.yml` and `auto-release.yml`.
- `internal/automation/prepare-release/main.go` and `ci-scope/main.go`.
- `skills/gimbal-release/references/distribution.md`.
- The repository `AGENTS.md`.

## Round 01 disposition

- Publisher path not exercised: now stated explicitly as unexercised (plan lines 10-11, 73-75). Accepted.
- Stray stable tag and `prepare-release`: the plan now covers coordination, an exclusive window, deletion of the task's own tag and verification of its absence (lines 87-95). Accepted. Confirmed that `prepare-release` takes the highest stable tag as `previous` and fails on `merge-base --is-ancestor previous source` (lines 115-118), so a stray tag really would break automatic releases.
- Qualify not tied to the candidate: C5 now records the `qualify` job conclusion (lines 66-68). Accepted.
- Budgets: now 300 and 440 against 283 and 419 nonblank, so growth is bounded. Accepted.
- Platform-output independence and expiry: addressed (lines 66, 73). Accepted.
- Conformance to `candidate-exchange.md`: the plan matches its build output, artifact-identity handling, seam and exclusions. It does not enable the publication gate or claim qualification.

## Findings

### 1. Issue: the "merge" handback option contradicts "publication remains excluded", and the unexercised publisher would run for real

The plan's last decision offers a handback that "includes squash merge", with "Publication remains excluded in either case" (lines 121-122). Merging cannot honour that.

- `ci-scope` treats only `skills/`, `docs/`, `ephemeral/` and root `*.md` as documentation-only (`ci-scope/main.go:25`).
- The PR changes `.github/workflows/release.yml` and `internal/automation/release-assets/*`, so it is release-scope.
- On a push to `main`, `auto-release.yml` then runs the `release` job. It prepares a tag and dispatches the tagged `release.yml` with the default `publish=true`, then waits for publication (`auto-release.yml:107-162`).

So the squash merge itself triggers a real signed release. That run would be the first live execution of the rewritten publisher: `needs: [qualify, candidate]`, `artifact-ids` download, the verify step, and the retry-attempt wiring. It would include the GitHub Release and the Homebrew update. These are exactly the steps the plan says it will report as "unexercised".

The plan already concedes that the publisher is not proven, yet offers a merge as a possible completion. That is an incorrect completion path, not just a caveat.

Requirement missed: "Publication remains excluded"; also the pre-release, no-users caution in `AGENTS.md`.

The plan should do one of two things:
- remove squash merge from the options; or
- state that merge is a publication event, require Tyler's separate explicit approval of that release, and say what happens if the first publisher run fails after partial publication. Recovery steps (replacing assets, retrying the Homebrew update) exist in `distribution.md`.

### 2. Nitpick: the DoD does not say how "no release or Homebrew update results" is established

C4 requires that publication is skipped and that no release or Homebrew update results. It does not say how this is observed. For example: the `publish` job conclusion is "skipped", `releases/tags/<tag>` returns 404, and the tap has no new commit. State the observation so the report is checkable.

### 3. Nitpick: the plan names parties the reader cannot resolve

"Terra 6.1" and "parent chat A" (lines 79, 89) are not defined in the plan or the repository. Ownership of tag cleanup and of the coordination window rests on them. Name the actual owner, or say "Tyler".

## Outcome

material findings remain
