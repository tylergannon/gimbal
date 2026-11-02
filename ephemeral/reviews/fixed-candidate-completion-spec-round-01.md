# Fixed candidate completion: specification review, round 01

## Target

Proposed task and definition of done in `ephemeral/plans/fixed-candidate-foundation.md`, for finishing draft PR #664 (`codex/fixed-release-candidate`, head `17deb5ba`). Preapproval, read-only review. Nothing was executed.

Ignored narrowing: none. The caller supplied only read-only and artifact-path constraints.

## Evidence inspected

- The plan under review.
- `ephemeral/worklog/fixed-candidate-completion.md` and `ephemeral/worklog/fixed-release-candidate.md`.
- `ephemeral/reviews/fixed-release-candidate-round-01.md`.
- The PR diff stat against `main`, and `.github/workflows/release.yml` in full for the changed jobs: `qualify`, `macos-release`, `candidate` and `publish`.
- `.github/workflows/auto-release.yml`.
- `internal/automation/prepare-release/main.go` tag selection.
- `skills/gimbal-release/references/distribution.md`, "Fixed candidate artifact" and the dispatch rules.
- The repository `AGENTS.md`.

**Not inspected:** `/Users/tyler/.codex/worktrees/release-qualification-design/gimbal-view/ephemeral/release-qualification/candidate-exchange.md`. Both Read and Bash were refused because the path is outside this session's allowed directory. Conformance of the plan to that exchange document is therefore **unverified**, and no finding below depends on it. It should be reviewed in a round that can read it.

## Findings

### 1. Issue: the changed publisher path is never exercised, yet the goal claims it is preserved

The goal says "Preserve the existing normal publication behavior". The PR rewrote that path.

- `publish` now depends on `[qualify, candidate]`.
- It checks out `github.sha` instead of the tag.
- It downloads by `artifact-ids` with `merge-multiple: true`.
- It adds a Go verify step with `CANDIDATE_RUN_ATTEMPT` taken from `needs.candidate.outputs.run-attempt`.
- It no longer recomputes `checksums.txt`.

Evidence: `release.yml` `publish` job diff.

The plan runs only `publish=false`, which skips `publish` (`if: inputs.publish`). C3 admits "no claim of a live publisher retry is made" and rests on source inspection.

- C5 downloads the artifact by hand. That does not show what `actions/download-artifact@v4` with `artifact-ids` plus `merge-multiple` actually lays out under `dist/`.
- The worklog says that layout was learned by reading the action's source, not by observing it.
- The first real execution of this path would be the next real release: a signed, notarized, public, hard-to-reverse publication.

Impact:
- The plan can pass every DoD item and still ship a publisher whose first run is production.
- If the layout or an input is wrong, the failure occurs after tagging, possibly after the release is partly created.

Requirement missed: the "preserve" claim and "Proof is what you saw".

Change the plan to do one of the following:
- add a bounded, non-publishing observation of the exact download and verify steps from a real artifact ID, for example a throwaway branch-only dispatch that does not touch `publish`; or
- make the completion report list "publisher consumption of the candidate: unproven" as an unmet claim, and drop "Preserve" from the goal in favour of "preserve by inspection".

### 2. Issue: a stable-format tag becomes the next release's base, and the plan has no cleanup

The plan's tag handling is a "fresh collision check" and "do not repoint an existing public tag". It also requires an "unpublished version tag" at a PR-branch revision, because `macos-release` runs only when the dispatch ref is a `v*` tag.

`prepare-release` lists all tags (`git tag --list`), keeps those matching `^v[0-9]+\.[0-9]+\.[0-9]+$`, sorts them, and uses the highest as `previous`. It then computes the next version and the CHANGELOG compare links from that tag (`prepare-release/main.go`, lines 63-125).

A stray stable tag, higher than the latest release and not on `main`, can therefore distort the next automatic release's version and CHANGELOG, or make it fail. This is outward-facing, and the plan never mentions it.

The plan also has no step to delete or retire the tag after the run, nor to confirm that no automatic release is running or about to run. The `release-<tag>` concurrency group and runner coordination are mentioned only for the runner.

Fix the plan so the approval request names all of these:
- the tag form;
- why it is safe for `prepare-release`, or an explicit pre-run check of the highest stable tag and of whether `auto-release` could run during the window;
- who deletes the tag after C5, and when;
- confirmation that deleting a tag the plan itself created is covered by the approval.

### 3. Issue: the candidate is not gated on source qualification, and the DoD cannot show the two are tied

The `candidate` job needs only `[linux-cli, macos-release]`. It does not need `qualify`.

- A candidate can be uploaded for a source whose `qualify` failed.
- The plan states "Candidate existence alone does not establish source qualification", which is correct.
- The only link to qualification is C4's run-level "all required build/source jobs succeed". The manifest and summary carry no qualification evidence, and C5 does not require the reader to check it.

The plan therefore leaves the following unstated: what a consumer must check before treating the artifact as "of a qualified source", beyond the run page. It should either:
- require C5 and the report to record the `qualify` job conclusion for the same run and attempt, by job ID or link; or
- state that this check is out of scope and that candidate existence means "built" only.

Without this, the phrase "source gates remain intact" in C3 and the completion report can overstate what the artifact conveys.

### 4. Issue: the budgets contradict the stated scope

- Current counts: `candidate.go` has 283 nonblank lines (297 total) and `release.yml` has 419 nonblank lines.
- The plan sets budgets of 340 and 490, which allow roughly 57 and 70 new lines.
- Yet the task is "making only necessary repairs with focused affected checks", with no new exported API and no speculative framework. `AGENTS.md` says "Build what was asked" and "As simple as possible".

A budget set above the current size, with no identified work to fill it, invites growth. State what, if anything, is expected to change, or set the budgets to the current size and require a reason for any increase.

### 5. Nitpick: the C5 independence and expiry wording is loose

- "Compare payload bytes to the same run's platform outputs" does not say how those outputs are obtained. They should be downloaded from the run's `cli-*` and `macos-release` artifacts by ID or digest, not from the candidate job's own workspace.
- With the two-day retention left unchanged, the report must state the expiry time. The plan does list "expiry".
- All of C5, including the platform-output comparison, must finish inside the two-day window. The plan does not say so, and the decision on retention is still open.

## Outcome

material findings remain
