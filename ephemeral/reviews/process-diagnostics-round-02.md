# Adversarial review: process-leak diagnostics, round 02

## Target

Branch `codex/process-leak-diagnostics` at c403fdfd. Changes since round 01: `f3d20841`, `9385ffb0`, `c403fdfd`. These touch `internal/processdebug/*`, the `executor.Launch` signature and its callers, `service.go`, and the adjudication note. The review is against `ephemeral/plans/process-leak-diagnostics.md`, `AGENTS.md`, and `ephemeral/reviews/process-diagnostics-adjudication.md`, which I assessed as caller reasoning.

The launch prompt imposed only operating constraints (read-only, no provider turns, no shared service lifecycle changes). I took no narrowing of scope from it.

## Evidence inspected

- The full diff since 311ef68a.
- All of `process.go` and `process_test.go`, plus the Launch callers in `agentexecutor` and `commandexecutor`.
- Commands run:
  - `go build` of the root package, `codex`, `agy`, `executor`, `agentexecutor` and `commandexecutor`: clean.
  - `go vet` of `processdebug` and the executor packages: clean.
  - `go test -count=1 ./internal/processdebug` passed twice.
  - `go test -race -count=3 ./internal/processdebug` now **passes**. Round 01's repeatability failure is fixed by running the fixture in a re-exec'd child with its own cache dir.
- No built-binary proof (flag off, flag on, luna turn) was available to me. The adjudication keeps it as required. It is still unreviewed.

## Assessment of the adjudication

- **Accepted: baseline, stop-neutrality and repeatability.** The code implements baseline separation (`baseline`, `classify`, `baseline_alive_count`), neutral stop observation (`StopRequested`, `stop_observation`) and a repeatable fixture. See findings 1 to 3 for what is still incomplete.
- **Accepted: no maximum observation duration for an active owner.** An operation may legitimately run indefinitely; the observer is constant-bounded (128 owners, 1024 identities) and takes no cleanup action. I withdraw round 01 finding 4's request for a lifetime cap.
- **Residual from that finding, now fixed.** Capacity drops are exposed promptly by `dropped_owner` events from active observers. I only note log noise (nitpick 5).

## Findings

### 1. issue: the shared-daemon baseline is taken after a multi-second subprocess preamble, so early turn children can be silently baselined

- Where: `process.go` `observe`. For shared owners, `selectedDaemon()` runs `codex app-server daemon version` and `lsof` (up to 2s timeout each, plus two census calls) before the first `sample()`. The first census is the `baseline`.
- Requirement: "Shared-daemon descendants without established task ownership are labelled candidates"; unrelated clients must not be conflated with Gimbal work, and missing ancestry must be recorded rather than invented.
- Impact:
  - Anything the turn's own tool processes spawned during that preamble is classified `baseline`. It is then excluded from `candidate_survivor` and `candidate_count` and reported only through `baseline_alive_count`.
  - For short turns the baseline can be taken after the turn finished its work. A genuine leaked child of this turn is then reported as a "baseline" process and never as a candidate.
  - The detail string says "changes before sampling … unknown", but the owner start event gives no timing, so a reader cannot tell how late the baseline was.
- Fix direction: take the baseline census in `start()` before launching the preamble, or record the baseline timestamp and the preamble duration.
- Related: if the first census fails (`return -1` before `firstSample = false`), the next successful census becomes the baseline, which is later still.

### 2. issue: shared-daemon and service-stop behavior is only tested through pure helpers, so the required "live client is not mislabelled" check is still unproven for the shared path

- Requirement: "Real subprocess tests demonstrate … an intentional live service/separate client is not mislabelled as an owned leak."
- Tested: `discover` and `classify` on hand-built maps, which is good for PID reuse and ancestry. Also tested: the service stop through a real `sleep` process, the capacity drop, a missing root, flag-off for all four constructors, and the log-write failure.
- Not tested:
  - `sample`'s shared branch: `baseline_alive_count`, exclusion of baseline from `candidate_survivor` and `candidate_count`, and `shared_daemon` versus `owned` labelling. `selectedDaemon` is a hard-wired function with no seam or stand-in, so no test can reach this integration.
  - The post-stop outcome. The test asserts the neutral `stop_observation` but never that a still-alive process becomes `suspect_survivor` after the 10s and 60s grace samples. The grace behavior that qualifies a stop is untested.
  - The stderr survivor warning from the real observer path. Only the formatting function is tested.
  - Cancellation.
- Everything is in one monolithic fixture test, so the first failure hides the rest.

### 3. issue: executor owner correlation is still weak and a stale result file ends ownership immediately

- Where: `files.go` `Launch` passes `string(req.Session)` for agent workers and `""` for command workers. `Executor` records no run. The owner end is "`result.json` exists".
- Impact:
  - Command-worker owners log `session` and `run` as `unavailable`, so a survivor from a command worker cannot be tied to a run or session even though the caller has them. Plan: "Associate run/session/turn identity where actually available". The missing correlation is recorded, which satisfies the second half, but it was available at the `Submit` sites.
  - If `Launch` reuses a root that already contains `result.json` (the `executor.Match` resume path in `Submit`), the first tick ends ownership, and a live worker is immediately reported as `suspect_survivor`. I did not reproduce this, but the stat is unconditional and round 01 flagged it. It was not addressed or rejected in the adjudication.
- Fix direction: pass run IDs through where `Submit` has them. Ignore a result file whose mtime predates the launch.

### 4. nitpick: the dropped-process path floods the log and inflates the drop count

- `discover` returns `dropped` per pass, and it is counted again on every 500ms sample while the tree stays at `maxProcesses`. Each sample writes a `dropped_process` event and adds to `l.dropped`. A fork-heavy tree at capacity therefore fills the 1 MiB × 2 rotation window with identical events, evicting earlier survivor evidence, and the `dropped=` figure no longer counts distinct processes.
- Emit once per owner, or only when the count changes.

### 5. nitpick: `dropped_owner` is replayed per observer

- `lastDrops` starts at 0 in each observer, so every newly started observer immediately logs the historical process-wide drop total, and every active observer logs each new drop. N active observers produce N copies of one fact. This is bounded noise, not a defect.

## Not found

- Flag-off creates no sampler, goroutine, log or subprocess. All four constructors are covered by the test.
- No kill, retry or gating behavior was added. Stop is observed without being treated as a leak; survivors are labelled "suspect" or "candidate" and the warnings say so.
- `safeID` now permits `.` and `/` segments but still rejects empty, `.` and `..` segments, spaces and absolute paths; it is tested.
- Capacity: 128 owners and 1024 identities per owner are enforced and tested. Log rotation, pruning, PID-reuse parent-chain breaks and missing ancestry are tested at the helper level.
- Round 01 findings 2 (service stop mislabelled) and 3 (repeatability) are fixed. Finding 1 is only partly fixed (see finding 1 above). Finding 4 is rejected by the adjudication, and I accept that.

## Outcome

material findings remain
