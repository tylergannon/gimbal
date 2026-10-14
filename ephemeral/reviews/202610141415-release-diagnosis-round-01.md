# Adversarial review: release diagnosis, round 01

## Target

`ephemeral/research/release-diagnosis/diagnosis.md`, judged against the
request it states (why release workflows failed, why Python entered a Go/TS
project, how the morning's proof missed failures, what is broken now,
double-check everything), `AGENTS.md`, `scripts/release_timing.py`,
`.github/workflows/release-performance.yml`, and live GitHub state.

The launch prompt only supplied the target, evidence location, artifact path,
and read-only limits. These are valid operating constraints; nothing narrowed
scope, so none was ignored.

## Evidence inspected

- `scripts/release_timing.py` lines 40-120 and 270-320 (`timestamp`, `seconds`,
  `stages`, `measure`, `main`, top-level handler).
- `.github/workflows/release-performance.yml` (scan step, then `if:`-gated
  hourly Go/Node/warm/benchmark steps).
- `/tmp/gimbal-release-audit-cancelled-jobs.json`: job 115944920729 is
  completed/cancelled, started 15:36:32Z, completed 15:36:31Z. Confirmed.
- `gh run view 38629348565 --log-failed`: "Timestamp order is invalid:
  2026-10-14T15:36:32Z -> 2026-10-14T15:36:31Z". Confirmed.
- Hourly run 38623966932 (schedule): scan step failed; Go, Node, warm, and
  hook-measure steps skipped. Confirmed.
- Hourly run 38605329199: scan failed on `gh api .../runs/38523301011/jobs`
  with HTTP 504, later steps skipped. Confirmed.
- Run 38590853348 created 15:36:42Z, failure. Confirmed.
- Release-performance history: since 15:36Z there are 37 failures and 1
  cancelled run; the last success is 15:35:14Z, before the cancelled job.
- Tag v0.13.23 dereferences to `8fcff5a8`; its message carries
  `Release-Source: ac958bf0`. Confirmed. Annotated tag object is `46927405`.
- Publish/Release/Automatic release/Redirect runs for v0.13.23 all `success`.
- PR merge times (#485, #514, #520, #530, #540, #541, #544, #545, #548) agree
  with the dates and Costa Rica local times in the diagnosis.
  `fe834bb7` (#485) and `ae13d09f` (#530) are the commits that introduced the
  Python scripts named.

I did not re-download the seven assets or re-run codesign, Gatekeeper, or the
Sparkle signature check; those claims rest on the diagnosis's own account and
the local evidence files, and I do not vouch for them independently.

## Findings

No critical findings and no incorrect claims were found. The central
diagnosis is reproduced from the source and the logs: one cancelled job with a
negative duration makes `seconds()` raise, `main()` has no per-record
isolation, the top-level handler exits 1, and the workflow gates the hourly
hook measurement behind that step. The separation of publication (healthy)
from observation (broken) is correct, and the stated limits of fresh proof are
honest.

### 1. nitpick: two distinct failure modes are blended

The diagnosis presents the timestamp ValueError as "the" active defect and
mentions the 504 only inside the impact paragraph. These are different
causes. The 504 (run 38605329199) is a transient API error with no retry that
equally aborts the scan; the timestamp error is deterministic and permanent.
Only the second is why every run since 15:36Z fails. A reader fixing only the
timestamp path would leave the scan fragile to any transient `gh api` failure
(the handler at `release_timing.py:311-316` catches `CalledProcessError` and
exits 1). Say so in one sentence.

### 2. nitpick: scale of the standing failure is not stated

"Repeated failed Actions runs" understates the evidence: 37 consecutive
failures at a 5-minute cadence since 15:36Z, and 6 open "Release timing
exceeded" advisory issues (#529, #531, #532, #537, #539, #542) that the
broken observer can neither update nor close. Quantifying this makes the
"what is broken right now" answer concrete and shows the failure is noisy
rather than silent.

### 3. nitpick: the review section is still a placeholder

`## Independent review` says "Pending". Before the diagnosis is presented, it
must record this review and its adjudication. Note it should also keep the
distinction that I confirmed source/log claims but did not independently
re-verify the artifact-signing claims.

## Outcome

only nitpicks remain
