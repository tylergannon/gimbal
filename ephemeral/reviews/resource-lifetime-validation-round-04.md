# Resource-lifetime validation, round 04

Reviewer: Sonnet 5.5 (independent). Read-only except this artifact. No live provider or service operations this round. I made only read-only local inspections: `git`, `ps`, and file reads.

## Target

Branch `codex/codex-resource-lifetime`, HEAD ce6dfded ("test: declare fixed native concurrency qualification shape"), clean tree. Runtime source is unchanged since be889c1c: `git diff be889c1c HEAD --name-only -- . ':!ephemeral'` lists only `*_test.go` files, so the output of that grep for non-test files is empty. Reviewed against the acceptance plan, AGENTS.md, the earlier rounds, the adjudication note, `/private/tmp/gimbal-665-pr-body.md`, and `/private/tmp/gimbal-resource-665-be889c1c/observations.md` (finalization update included).

## Evidence inspected

- The ce6dfded diff: `TestNativeGroupSessionsLive` now uses two explicitly named `group.Go("first"/"second", ...)` children with relative barrier files. This resolves the custom lint issue about a dynamic child loop. The `ready-N` / `crossed-N` overlap assertions are preserved. The post-run `os.Stat(filepath.Join(project.Path(), "crossed-N"))` still matches because the worker's working directory is `project.Path()`.
- The PR body, compared claim by claim against what I observed in rounds 02 and 03, and against the record contents (below).
- The adjudication note `ephemeral/reviews/resource-lifetime-validation-adjudication.md`.
- The executor checkpoint `~/.gimbal/executors/sessions/session_01M6D36WYVB711G7070NTS67ZQ`, which the adjudication cites. It contains `native.json` with a native thread ID and model `gpt-gimbal-665-invalid`, plus `closed.json` with `"session_closed_reason":"explicit close"`.
- Current process state, read-only: daemon 48413 and code-mode host 48733 are alive and preserved; central 97934 is gone, as the observations say it was deliberately stopped after qualification.

## Correction to my round 03

The adjudication is right and my round-03 finding 2 was wrong. I wrote that the unsupported-model runs had "no connection, thread or tool" to release. The executor checkpoint shows a native thread was allocated before the provider rejected the turn, and the session was closed explicitly. These runs therefore do exercise failed-start release (not connection loss), as the adjudication states. The PR body does not repeat my mistaken claim. Nothing else in round 03 changes.

## PR body versus evidence

| PR claim | Check |
|---|---|
| Runtime candidate be889c1c, later commits tests and notes only | Confirmed by diff. |
| Three normal, three cancelled, three rejected-model CLI runs, with IDs | Run records existed and matched in round 03. |
| Normal commands too brief to sample; tool-child evidence comes from Group and native recovery cases, not the pre-tool cancellations | Matches my round-03 finding 1. |
| Sonnet daemon lsof counts back to 75 after five adapter cases, then 73 after Group, workers zero | Matches my round-03 samples. The PR also correctly warns that lsof lines and numeric FDs are different units. |
| Failed-redial test cancels explicitly; recovery otherwise pauses for operator or deadline; not used as evidence for no-replay | Matches `recovery.go` and the test. |
| Successful-recovery test shows exactly-once effect | I did not rerun it. Source-level support only. |
| Final Group run `01M6D4BKEWG0YV9KEDE89DBCD3` (6.21s) | The test's temp project is cleaned up, so I could not inspect that run. The duration equals the earlier conversation check's 6.21s. The observations file gives distinct timestamps for it (21:20:28-34), so this is plausibly a coincidence. It remains a single unverified reference. |
| `just build`, race checks, lint, `go vet`, custom lint, 79-package suite | Reported by the caller. I did not rerun them this round (no-additional-operations constraint). I ran the codex and host non-live tests in round 03 at an earlier head. |
| Not claimed: historical causation, Desktop internals, native locked-use, release | Appropriately scoped. |

## Findings

No critical or issue-level findings. The implementation and the proof now line up with the plan's claims 1-6, with the scoping the PR body already states.

### Nitpicks

1. **Live host tests write session state into the real `~/.gimbal/executors`.** `executor.Root("")` defaults to the user's home, and the host live tests (conversation and failed-start sessions) leave `session_*` directories there. There are 43 session directories now. These are small persisted records, not process or handle growth, but they are retained state the tests do not clean up. The PR's statement that "runtime output is not committed" is true, and it says nothing about this residue. If the tests are meant to be repeatable, they should point the executor root at the test's temp directory.
2. **Final Group run is attested only by the caller.** The run ID and the 6.21s duration are in the PR body and the observations file, but the record was in a temp directory that was cleaned. A reader cannot verify it afterwards. I verified an equivalent run myself in round 03 (6.82s, both barriers held, workers zero afterwards, daemon back to baseline) at 52e292b4. The ce6dfded change to the same test is a mechanical rewrite to two named children.
3. **Round-02/03 carry-overs, optional:** `startupLockPath` runs `MkdirAll` on every call, and no test exercises `connect` through the lock. The adjudication correctly treats these as non-requirements.

## Residual gaps (stated in the PR; not findings)

- No real daemon cold start was run, by Tyler's authorized replacement. The startup lock is proven only by subprocess exclusion and recheck tests with a fake CLI.
- No loaded-thread sample at every production CLI boundary. The archive/unload test shows zero loaded threads after close.
- Historical ~30-process causation is not established. No growth recurred in the measured workload.
- Strict FD constancy is not claimed. The 56-59 numeric range and the 73-75 line-count range are small, non-monotone and explained by the SQLite and socket inventory in the observations.

## Outcome

only nitpicks remain
