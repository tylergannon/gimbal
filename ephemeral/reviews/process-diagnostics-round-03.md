# Adversarial review: process-leak diagnostics, round 03

## Target

Branch `codex/process-leak-diagnostics` at a251b230. Changes since round 02: `0d5cdac5` and `a251b230`. These touch `internal/processdebug/*`, `executor.Launch`, `commandexecutor.Request`, `command.go`, `taskdaemon/process.go`, and the worklog. The review is against `ephemeral/plans/process-leak-diagnostics.md`, `AGENTS.md`, and the round 01 and 02 reviews with the adjudication note.

The launch prompt imposed only operating constraints (read-only, no provider turns, no shared service lifecycle changes). I took no narrowing of scope from it.

## Evidence inspected

- The full diff since c403fdfd, all of the current `process.go`, `daemon_unix.go`, and the executor and command callers.
- The new `process_test.go` list: `TestSharedDaemonStandin`, `TestLongServiceGrace`, the isolated-subprocess fixtures, `resultChanged` coverage, and the pure helper tests.
- Commands run:
  - `go build` of the root package, `codex`, `agy` and `./internal/...`: clean.
  - `go vet` of `processdebug`, `executor`, `commandexecutor`, `agentexecutor` and `taskdaemon`: clean.
  - `go test -race -count=2 ./internal/processdebug`: passes.
  - `go test -count=1 ./internal/executor ./internal/commandexecutor ./internal/agentexecutor`: pass.
- I did not see the built-application proof (flag off and on, disposable state, worker and service lifetimes, luna turn). The plan and adjudication still require it, and it is outside what I can assess here.

## Round 02 findings: disposition

1. **Baseline timing: addressed by disclosure.**
   - `first_census` now records `requested_at`, `census_at` and `discovery_delay_ms`.
   - The baseline and detail text say the first census may include work begun before or during discovery. A reader can therefore see how late the baseline was.
   - Residual: early work is still hidden in the baseline. That is a stated, visible limitation, not a defect. I accept it.
2. **Test coverage: addressed.**
   - A shared-daemon stand-in through the `resolveDaemon` seam exercises the real `sample` path.
   - Baseline children are checked not to appear as `candidate_survivor`; `baseline_alive_count` and later first-seen candidates are checked.
   - `TestLongServiceGrace` asserts `suspect_survivor` at 10s and 60s after a stop, and the stderr warning.
   - Fixtures run in isolated subprocesses, so `-count>1` is stable. I confirmed this with `-race -count=2`.
3. **Stale result and run correlation: addressed.**
   - `Launch` stats `result.json` before `cmd.Start`.
   - `resultChanged` requires a different file, size or mtime, and a `result_baseline` event records a replay.
   - Command workers now carry the run through `DiagnosticRun` (`json:"-"`, so the accepted-request match is unaffected).
4. **Log flooding and drop count: addressed.** `dropped_process` is emitted once per owner.
5. **Round 02 finding 5: unchanged.** `lastDrops` still starts at 0 per observer, so each new observer replays the process-wide drop total. Bounded noise.

## Findings

### 1. nitpick: the diagnostic plumbing is spread across more production code than "a few hooks" and removability suggest

- The plan asks for a concentrated, removable observer with a few hooks into existing ownership boundaries.
- `executor.Launch` now takes `session, run` and re-implements the flag check (`os.Getenv("GIMBAL_DEBUG_PROCESSES")`) and a `os.Stat` of `result.json` outside `processdebug`. It passes `os.FileInfo` and an error into `Executor` and `ExecutorCandidates`.
- `commandexecutor.Request` gained `DiagnosticRun`, and it is filled in two unrelated callers (`command.go`, `taskdaemon/process.go`).
- Removal now means touching about a dozen call sites and a public-looking internal signature, not deleting a package and a few one-line calls. A stat helper in `processdebug` taking the result path would keep the env gate and the prior-state capture in one place.
- Impact: maintainability only. Behavior is correct.

### 2. nitpick: unrelated edits in the diff

- `files.go`: `<-decoded //nolint:errcheck // …` is on a channel receive that errcheck does not analyse, so the comment documents nothing.
- `pruneLogs` and `safeID` conditions were rewritten by De Morgan with identical behavior.
- These are churn in a diff that is supposed to stay removable. Revert them.

### 3. nitpick: agent workers still log no run

- `agentexecutor.Submit` passes `""` for `run`, so agent-worker owners log `run: unavailable` while carrying a session. This is recorded explicitly, as the plan permits ("explicitly record missing correlation"). It is a limit to state in the proof report, not a defect.

### 4. nitpick: `dropped_owner` is still replayed per observer

- As in round 02 finding 5. Each newly started observer logs historical drops once and each active observer logs each new drop. Bounded by log rotation.

## Not found

- Flag-off still creates no sampler, goroutine, log or subprocess. The new env check in `Launch` performs one `os.Getenv` and no I/O when the flag is off.
- Nothing kills, retries, gates results or enforces caps. Stop initiation is neutral, and survivors are labelled suspect or candidate with the uncertainty in the text.
- The `resultChanged` logic handles a missing, existing, replaced-inode, resized or re-timed result, and treats an unreadable prior state as an observation gap rather than assuming completion.
- No new race: the census map is replaced, never mutated, and observers share it under the mutex.

## Outcome

only nitpicks remain

Caveat: the outcome covers code and tests. The built-binary proof the plan requires is not part of what I could review, and it is still outstanding.
