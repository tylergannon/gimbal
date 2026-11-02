# Adversarial review: process-leak diagnostics, round 01

## Target

Branch `codex/process-leak-diagnostics` at 311ef68a, diff `a37d24d3..HEAD`. That is `internal/processdebug/*`, the hooks in `agy/agy.go`, `codex/codex.go`, `internal/executor/files.go` and `service.go`, measured against `ephemeral/plans/process-leak-diagnostics.md` and `AGENTS.md`.

The launch prompt contained only operating constraints (read-only, no provider turns, no broad tests). I took no narrowing of scope from it.

## Evidence inspected

- The full diff and all of `internal/processdebug` (`process.go`, `daemon_unix.go`, `snapshot_darwin.go`, `snapshot_linux.go`, `snapshot_other.go`, `process_test.go`).
- `executor.Launch`, `ownedService` start/stop/wait, the agy `runOnce` wait path, and the codex `RunTurn` path.
- `internal/commandexecutor/command.go`, to confirm commands run inside executor workers and so are covered by the `Launch` hook.
- Commands run:
  - `go vet`: clean on darwin; `GOOS=linux` and `GOOS=windows` build and vet clean.
  - `go test -count=1 ./internal/processdebug` passed four times, once with `-race`.
  - `go test -race -count=3 ./internal/processdebug` **fails**; see finding 3.
- No proof report (built binary, flag off/on, luna turn) was supplied to me, so the plan's "run the real thing" requirement is unreviewed. It must be in the chat and the PR body.

## Findings

### 1. issue: shared-daemon "candidates" include every pre-existing child of the daemon, so unrelated clients appear as survivors

- Where: `process.go` `observe`/`sample`, first sample for `shared` owners. `codex.go:335` and `files.go:234` both start a shared owner.
- Requirement missed: "without conflating unrelated Desktop clients with Gimbal work" and "reports support inspection of … count trends across completed owners".
- Cause: on the first sample every existing child of the daemon is added to `known` and labelled `candidate`. There is no baseline of what existed before the turn or worker started. After owner end, every still-alive one of those, including long-lived MCP servers and tool processes belonging to Desktop sessions, is emitted as `candidate_survivor`, and `candidate_count` includes them. The count therefore tracks the daemon's total child population, not work attributable to this turn.
- Reproduction (reasoning): with a Desktop session holding N live children of the shared daemon, any Gimbal Codex turn logs N `candidate_survivor` events and `candidate_count=N` at 0s, 10s and 60s, regardless of what the turn did.
- Related cost: a worker run starts `ExecutorCandidates` and the Codex turn inside it starts `Candidates`, so one task is observed twice. Each shared owner also runs `codex app-server daemon version` and `lsof` before sampling, per turn and per worker.
- Fix direction: mark children that predate the owner start as `preexisting`, or report only first-seen-during-owner children as candidates.

### 2. issue: stopping a service reports its own root and children as `suspect_survivor` at age 0, by construction

- Where: `service.go` `beginStop()` calls `observation.End()` before the process is signalled. `finish()` then samples immediately.
- Cause: owner end is declared at stop *initiation*. The root, which is still alive and is about to be terminated, is therefore logged as `suspect_survivor` with `owner_end_age_seconds=0` for every deliberate service stop. The later 10s and 60s samples then clear it, but the 0s record is a guaranteed false positive on the normal path.
- Impact: this weakens the "suspect survivor" signal and works against the plan's rule that an intentionally live or deliberately managed service must not be mislabelled as an owned leak.
- The worklog says End deliberately sits at the stop boundary so a still-alive root can be reported. That reasoning fits executor result publication, where the worker should already be exiting. It does not fit a service that Gimbal is itself about to kill.
- Fix direction: for services, end the owner after `Wait` returns, or distinguish `stop_requested` from `owner_end`.

### 3. issue: the required package tests are largely absent, and the one test is fragile

- Requirement: "Package-local tests must exercise stable process identity/PID reuse, attribution and missing ancestry, bounded state/log rotation, cancellation and flag-off/error behavior … a live service/separate client is not mislabelled."
- Present:
  - `sameProcess` on a unit struct.
  - Log rotation and pruning.
  - A single monolithic subprocess test.
  - A flag-off test for `Start` only.
- Missing:
  - Observer-level PID reuse. Nothing checks that a reused PID with a different birth is not attributed, or that a child is not attached through a stale parent.
  - `maxProcesses` and `maxOwners` bounds and the dropped counts.
  - Missing-ancestry and `unavailable` paths: root reaped before the first sample, an enumeration error.
  - Shared-daemon candidate labelling. `Candidates`, `ExecutorCandidates` and `selectedDaemon` have no test.
  - Flag-off for `Candidates`, `Executor` and `ExecutorCandidates`.
  - Cancellation.
- The "separate client not mislabelled" assertion is vacuous. The `unrelated` sleep process is never a descendant of the observed root, so it cannot appear under any implementation. A meaningful check would use a separate process under a shared-daemon stand-in.
- Fragility, reproduced: `go test -race -count=3 ./internal/processdebug` fails (`diagnostic did not contain "pid":…`, plus `log unavailable`). The package-level `sync.Once` logger keeps a path inside the first `t.TempDir()`, which is deleted after run 1. The test also reads `data` before later steps and asserts on that stale snapshot.
- Repository rule: repeatable checks must live beside the package, so these gaps are plan requirements, not extras.

### 4. issue: observer goroutines and slots have no completion bound for the result-path owner

- Where: `process.go`, the `observe` loop, `Executor`/`ExecutorCandidates` (`result != ""`).
- Cause: an executor owner ends only when `result.json` appears, or when the worker is reaped and 2s pass with no result. If the worker is alive but never publishes (hung, or a daemon-resident attempt), the goroutine, its 500ms ticker and its slot persist for the life of the process.
- Impact: 128 such owners exhaust `slots`. Later owners are then silently skipped, and `droppedOwners` is visible only in later `survivor_count` details. The plan asks for bounded retained state and exposed drops; the drop is exposed only by a coincidental later event.
- Also, the `known` map retains exited entries up to `maxProcesses` for the owner's life, and its size is not itself reported.
- Fix direction: log a `dropped_owner` event when a slot is refused, and put a maximum observation lifetime on an owner that never ends.

### 5. nitpick: stale-result and log-size edge cases

- A pre-existing `result.json` in the worker root (resume or reused root) ends the owner on the first tick, so the live worker would be reported as a survivor immediately.
- `logger.size` starts at 0 for the current process's file. That is fine for `CreateTemp`, but it is not reconciled if the file is externally appended to or rotated by another writer.
- Every event reopens and closes the log file. That is acceptable for debug use, but costly under the 128-owner ceiling.

## Not found

- Flag-off creates no sampler, goroutine, log or subprocess (`start` returns nil before any work). Hooks are nil-safe.
- The code never kills, retries cleanup or gates a result. Write and enumeration failures only increment drops or log to stderr.
- Identity is PID plus birth, anchored at launch before `Wait`, which is correct. No arguments, environment or prompt contents are emitted; `safeID` restricts IDs.

## Outcome

material findings remain
