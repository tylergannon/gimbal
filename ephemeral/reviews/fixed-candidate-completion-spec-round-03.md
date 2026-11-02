# Fixed candidate completion: specification review, round 03

## Target

The current `ephemeral/plans/fixed-candidate-foundation.md`, for finishing draft PR #664 (`codex/fixed-release-candidate`, head `17deb5ba`). Preapproval, read-only. Nothing was executed.

Ignored narrowing: none. The caller supplied only read-only and artifact-path constraints.

## Evidence inspected

- The current plan and `ephemeral/worklog/fixed-candidate-completion.md`.
- Rounds 01 and 02 of this review, and the sources they cite:
  - `release.yml`, `auto-release.yml`
  - `prepare-release/main.go`, `ci-scope/main.go`
  - `distribution.md`
  - `candidate-exchange.md`
  - `AGENTS.md`
- I did not re-read the sources for this round beyond what rounds 01-02 established. The plan changes since round 02 are confined to the C4 observation method, execution roles, and the handback, which now excludes merge.

## Round 02 disposition

- **Merge handback:** resolved. The plan now hands back a merge-ready PR only, says "do not merge in this lane", and says merge can activate the automatic release path (lines 105-108, 129-130). This matches my round 02 trace: the PR is release-scope, so a merge on `main` dispatches a `publish=true` release.
- **C4 observation of "no release/Homebrew update":** resolved. The publisher job must be skipped, no GitHub Release may exist for the test tag, and the tap revision must be unchanged, with a rule for unrelated tap activity (lines 61-63).
- **Role names:** resolved. C, D, chat B and parent chat A are now identified (lines 82-94).

## Findings

No material findings remain. The plan is consistent with the exchange document, its exclusions, and the repository rules on proof and on building only what was asked. The remaining items are nitpicks.

### 1. Nitpick: repair after a failed run is ambiguous against the fixed revision and tag

Step 4 allows a "bounded retry" after cause inspection, and the decisions section allows "cause-based repairs/retries". The `candidate` job requires `refs/tags/$TAG^{commit}` to equal `github.sha` (`release.yml`, the assemble step), and the plan forbids repointing a tag.

A repair that changes any file therefore needs a new commit and a new tag, with the tag deletion and window coordination repeated. Only a rerun of an unchanged revision is a plain retry. Say which cases are covered by the existing approval and which need Tyler to approve the new revision and tag. The plan's own "own newly created tag" rule suggests that deleting and recreating the task's tag is allowed, but it does not say so.

### 2. Nitpick: the candidate.go budget mixes units

Line 30 sets "300 nonblank production lines (currently 297 total)". The file currently has 283 nonblank lines, so the budget allows 17 more. The comparison reads as if only 3 are allowed. State the current nonblank count.

## Outcome

only nitpicks remain
