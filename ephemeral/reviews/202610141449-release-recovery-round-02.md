# Release recovery, observer-first checkpoint: adversarial review, round 02

## Target

Current worktree on `codex/release-diagnosis` (staged: Go observer
`internal/automation/release-timing/`, deleted `scripts/release_timing*.py`,
workflow edits, execution.md, worklog; unstaged: issue wording change; untracked:
`recovery-adjudication.md`, round-01 review). Compared with `diagnosis.md`,
`execution.md`, `recovery-adjudication.md` and AGENTS.md. Checkpoint scope is
observer recovery; the remaining Python migration is authorized but not in this diff.

No narrowing instructions were present in the launch prompt.

## Evidence inspected

- Full diff against HEAD, including the one-line unstaged change
  (`still pending at` -> `not published by`, `main.go:595`).
- `go vet` clean; `go test -count=1 ./internal/automation/...` passes.
- Real binary, read-only: `--scan --measure` against the live repository exits 0
  with 46 measurements and 0 observation errors; the two reversed-timestamp job
  records are exposed as per-stage `timing_error`, not fatal (same result as round 01).
- Adjudication claim check via the releases API: v0.13.15 is published
  (2026-10-14T15:47:13Z); v0.13.17, v0.13.18 and v0.13.19 return 404, so no
  published GitHub release exists for them. This matches the adjudication.

## Round-01 findings, disposition

1. Backlog of advisory issues on first run: resolved by adjudication. The
   unpublished sources are real failed deliveries (verified above), policy and the
   Python observer behaved the same, and the wording no longer implies an active
   release. I accept not moving the cutoff. One residual: the reported elapsed
   time for those sources still grows with every scan, but issues are deduplicated
   by source marker, so it appears once at creation; acceptable and now worded
   honestly.
2. Unseen real-workflow proof: unresolved by design and correctly stated as
   outstanding in the adjudication (merge, dispatch on main, verify both jobs and
   artifacts, before starting the migration). This is a gate, not a code defect.
3. and 4. Nitpicks stand; no change needed.

## Findings

### 1. [nitpick] Wording change has no test

`main.go:595` changed text that appears in a filed issue body, but no test asserts
it, so a revert would pass. Cheap to add beside
`TestIssueDeduplicatesSourceAcrossRetriesAndClosedIssues`; not required.

### 2. [nitpick] `TestHourlyHookJobIndependentOfObserver` still matches YAML text

`main_test.go:406-422`: unchanged from round 01; the check that matters is the
dispatched run.

### 3. [nitpick] Worklog and adjudication are not yet committed

`recovery-adjudication.md` and the round-01 review are untracked, and the worklog
mentions an inline Python server in `local_service_fulfillment_test.go` as a later
item. Neither blocks the checkpoint; commit them with the PR so the reasoning
travels with it.

## Assessment

The implementation fixes the diagnosed defect (verified live, read-only), keeps
mutations un-retried, bounds read retries, isolates per-source failures and
decouples the hourly hook job. No incorrect implementation, bug, race or
over-engineering found; no new public API; tests live beside code. The phase-one
checkpoint is acceptable to merge, with post-merge dispatch verification still
required before further migration.

## Outcome

only nitpicks remain
