# Release recovery, observer-first checkpoint: adversarial review, round 01

## Target

Uncommitted worktree on `codex/release-diagnosis` (HEAD 2f27cbfa): new
`internal/automation/release-timing/` (Go port of the observer), deletion of
`scripts/release_timing.py` and its test, edits to `release-performance.yml` and
`auto-release.yml`, and `execution.md`. Measured against `diagnosis.md`
(remediation plan, desired state, proof items 1-3), `execution.md`, and AGENTS.md.
Checkpoint scope: observer recovery first. The remaining Python replacement
(hook, CI scope, release prep/dispatch/assets, appcast, notary, skill helpers) is
authorized and intentionally not in this diff; I do not count its absence as a defect.

## Review request handling

The launch prompt only named the target, the artifact path and read-only limits.
No narrowing instructions were present or ignored.

## Evidence inspected

- Complete diff and untracked files; full `main.go`, the test list, and the final
  `release-performance.yml`.
- `go vet ./internal/automation/...` clean; `go test -count=1 ./internal/automation/...` passes
  (21 tests, including the cancelled-timestamp, bounded-retry, per-source-continuation
  and independent-hook-job cases).
- Executable evidence, read-only GitHub API: ran the real binary
  `go run ./internal/automation/release-timing --scan --measure --report ...` against
  the live repository. Exit 0, 46 measurements, 0 observation errors. Two stages
  carry the exposed gap, e.g. run 38588557179 "Publish GitHub Release and Homebrew
  Casks: timestamp order is invalid 15:36:32Z -> 15:36:31Z". The cancelled job from
  run 38590047563 is therefore exposed rather than aborting the scan, so the
  diagnosed defect is fixed locally. `--measure` creates no issues.
- Compared the non-measure path (issue creation) against the live issue list.

## Findings

### 1. [issue] First live non-measure run will file issues for incident-era sources, with "elapsed" that grows without bound

`internal/automation/release-timing/main.go:586-616` (ensureIssue),
`main.go:446-452` (elapsed measured to `now` when `end == nil`),
`scripts/release-performance.json` (`enforce_after` 2026-10-14T05:13:48Z).

The observer has been dead since about 15:36 UTC, so every source after that has
never been through the issue path. The live scan above shows four
violations after `enforce_after` with no existing issue: 62e1be2f (725 s, published),
9f597f3c, fab36b0b and 3e6d046d (delivery "not published", 15,000-16,000 s and
still growing). The first scheduled run of the merged observer will create all
four in one scan. For the three "not published" ones I did not establish whether
they were superseded by a later published tag (that would make the issues false
positives, and `measure` only excludes supersession when the dispatch step was
skipped) or are genuine unpublished incident sources. Either way the plan's
"identify unavailable timing explicitly" requirement is only half met: the issue
body reports a pending elapsed time computed against the observation clock, which
is meaningless for a record that will never complete.

Impact: a burst of up to four advisory issues on recovery, and for unpublished
sources an issue whose headline number is an artifact of when the scan ran.
Suggested resolution, in the plan's spirit and without new API: decide explicitly
before merge whether those sources should be reported, e.g. judge against the
final tagged release for the same version, or move the cutoff past the outage.
Report the outcome in the PR.

### 2. [issue] The recovery proof item that matters most (plan proof 3) has not been seen in the real workflow

`.github/workflows/release-performance.yml:35-39`; `diagnosis.md`, proof 3: "The
real observer completes its historical scan ... and the hourly hook measurement
completes. This must be seen in actual runs."

The workflow runs `ref: main`, so nothing in this diff executes in Actions until it
is on main. Local evidence (above) is good but covers `--measure` only; the
issue-creating path, the artifact upload, `go run` cold on `ubuntu-24.04`, and the
new `hook-observe` job are unobserved. `hook-observe` also still shells out to
`python3 scripts/pre-commit.py`, which is by plan for phase one, but means the
"hourly hook completes" proof is the old Python hook plus the Go reporter. The
checkpoint should not be called recovered until a dispatched run on main
(`workflow_dispatch` runs both jobs) is seen to succeed and its artifacts exist.

### 3. [nitpick] Workflow-structure test is string matching on YAML

`main_test.go:406-422` (`TestHourlyHookJobIndependentOfObserver`) cuts the
workflow text at `"  hook-observe:\n"` and checks for `needs:` and two command
substrings. It reaches outside the package via `../../../.github/...`, breaks on
harmless reformatting, and would pass if a `needs` moved onto the `observe` job
beneath a reordered file. Since the independence is visible in 30 lines of YAML, the
test adds fragility more than assurance; the real check is item 2.

### 4. [nitpick] Whole run goes red on any single-source observation error

`main.go:877-879` returns an error (exit 1) when any `scanError` exists, even though
the report is written and later sources were processed. That matches the old
"failure is visible" intent and the plan does not forbid it, but a persistent bad
record of a *new* kind will again produce a red scheduled run every five minutes.
Worth stating the intent (alert vs. advisory) in the PR; no code change demanded.

## Things checked that held up

- Negative-duration jobs become `timing_error` on the stage and mark
  `active_critical_path_complete` / `parallel_platform_critical_path_complete`
  false; no duration is manufactured, and `seconds()` still rejects bad order.
- Read retries are bounded (3 attempts, 250/500 ms), limited to a transient
  pattern, cached on success; POST is never retried; the issue cache is invalidated
  after create.
- A failed release-list read returns early with the error recorded rather than
  reporting sources as undelivered, which is the right conservative choice.
- Hook reporting runs even when the scan recorded errors
  (`TestHookProcessingContinuesAfterGlobalScanError`).
- AGENTS.md: no new public gimbal API, tests beside code, no committed run output,
  no reflection. Python files removed only for the observer.

## Outcome

material findings remain
