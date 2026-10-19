# Adversarial review: issue 611, round 05

Date: 2026-10-18. Reviewer: Claude Fable 5.1 (claude-fable-5-1), read-only.

## Review target

The complete current work on branch `codex/issue-611-executors`, now five
commits on top of `main` with a clean working tree: `8b6e84bd`, `662f4f7c`,
`18493175`, `5896ee69` and the new `7d6b1120` ("fix: preserve executor
deadlines and retained close receipts"), 61 files changed. Reviewed against
`ephemeral/issues/611.md` with its conversation clarifications, the PR
description in `/tmp/issue611-pr.md`, `AGENTS.md`, and rounds 03 and 04.

"Do not edit any other files" was honored: nothing outside this file was
written into the tree. The reproduction below used a scratch root and a
binary built into the session scratchpad.

## Evidence inspected

- Issue text, PR description, worklog (`ephemeral/worklog/20261018-issue-611-executors.md`),
  `AGENTS.md`, rounds 03 and 04.
- Full current text of `internal/executor/files.go`,
  `internal/agentexecutor/agent.go`, `internal/agentexecutor/cli.go`,
  `internal/commandexecutor/command.go`, `internal/taskdaemon/process.go`,
  the executor section of `README.md` (lines 272-316), the test files of the
  three executor packages; the `7d6b1120` diff of those files; the branch
  diffs of `internal/taskdaemon/daemon.go`, `client.go`, `remote_execution.go`,
  `backend/backend.go`, `internal/invocation/invocation.go`; `command.go:258-330`
  (`runCapturedCommand`); `cmd/gimbal/main.go` dispatch and `daemon.go` root
  flag; `internal/binding/binding.go:50-53`; `codex/codex.go:318-365, 405-525`
  and `codex/executor.go` for the close and checkpoint paths.
- Ran `go build ./...` (only the pre-existing `plugins/*` main-less link
  errors), `go vet` on the executor, task daemon and `cmd` packages and the
  root package (clean), and `go test -count=1 -race` for `internal/executor`,
  `internal/commandexecutor`, `internal/agentexecutor`, `internal/taskdaemon`,
  `internal/binding`, `cmd/gimbal`: all passed. `~/.gimbal/executors` had 463
  entries before and after; nothing new landed in the home directory.
- Reproduced the round 04 fix with the built binary against a scratch root
  (transcript below).
- Not verifiable here: the live harness runs, caller-loss survival,
  steering, memory figures and the Bob remote route. The PR description
  reports them in the form `AGENTS.md` asks for, naming `luna`, `haiku`,
  Pi `glm-flash` and Gemini `flash`. Taken at face value, not re-run.

## Round 04 findings, re-checked

1. `--close` on an already-closed session (issue): addressed. `cli.go:54-61`
   no longer reads `close.json` or describes the close turn; it calls
   `Adapter.Close` and prints a `completed` descriptor whose `events` and
   `result` are the session's own `closed-events.jsonl` and `closed.json`,
   which the worker writes together (`agent.go:365-379`) on both the explicit
   close and the interruption-release paths. `Describe` now refuses a
   directory without `request.json` (`files.go:179-182`); every caller is
   guarded by an `Exists(request.json)` or a successful `Match`. Covered by
   `TestRepeatedCLICloseUsesSessionReceiptAfterOperationVacuum`. Observed with
   the built binary: a scratch session, `--close`, remove the `turn_*`
   directory and `close.json` as the round 04 scenario did, `--close` again:

   ```
   {"state":"completed","socket":null,"events":".../sessions/session_01K7X…A1/closed-events.jsonl","result":".../sessions/session_01K7X…A1/closed.json"}
   exit=0
   ```

   Both invocations printed that line. `--lookup` of the removed turn says
   "turn not found", and a new turn on the closed session is rejected with
   `session is closed: explicit close`.
2. Unconfirmed-stop retry invisible to readers (nitpick): partly addressed.
   `finishInterruption` now writes `cleanup.json` with state, attempt count,
   time and last error after every attempt (`agent.go:648-659`), and README
   documents it. `--lookup` still prints `state: running` with no hint of the
   file. Remains a nitpick (below, 2).
3. Library `init()` hooks: unchanged (nitpick 3).
4. Session lease hand-off by timing: unchanged (nitpick 4).
5. Local and remote deadline computation differ: addressed. `RunTurn`,
   `Fork` and `runCapturedCommand` now carry the absolute context deadline in
   `Request.Deadline`; the worker applies the earlier of `Timeout` and
   `Deadline` (`agent.go:210-223`, `command.go:116-129`). An already-expired
   deadline yields an immediately-expired worker context and a `timed_out`
   result, which is the right record. The default stays on the two CLI
   `--timeout` flags.

## Findings

No issue-level findings. Four nitpicks, one new.

### 1. nitpick — cancel-then-close on a Codex session can race the interruption receipt and submit a close turn against an already-closed session

`agent.go:596-636`. `Adapter.Close` checks `closed.json` once at entry
(`:606`), then, if a turn is active, sends `cancel`, waits for its result, and
submits a close turn. The interruption path in the worker writes `closed.json`
(`:376`) and then `result.json` (`:394`). So when a Codex turn's stop is
unconfirmed and settles through `finishInterruption`, a `Close` that began
while the turn was running sees `closed.json` absent, waits for `result.json`,
and still submits a close turn. The worker accepts it (`:267` only refuses when
`!req.Close`), `LoadSession(…, closing=true)` re-registers the thread
(`codex/executor.go:26-31`), and `codex.Close` calls `thread/archive` again
on a thread the previous worker already archived. If app-server answers with
anything other than success or "no rollout found", the close turn fails and
`Adapter.Close` returns an error for a session that is in fact closed; the
`closed.json` receipt is left as written, so the CLI and later turns are
unaffected. Re-checking `closed.json` after the wait at `:618-622` removes the
window. This is the cancellation edge Tyler asked not to spend more time on;
recorded for completeness.

### 2. nitpick — `--lookup` does not surface `cleanup.json`

`cli.go:86` prints `Describe(dir, true)`, which only knows `running` and
`completed`. A turn held in `finishInterruption` shows as `running` with a
live socket; the stopping/attempt detail lives in `cleanup.json`, which the
README names but the descriptor does not. Residue of round 04 nitpick 2.

### 3. nitpick — library `init()` hooks still take over any binary that links Gimbal

Unchanged from rounds 01 to 04: `commandexecutor/command.go:47-61` and
`agentexecutor/agent.go:27-41` inspect `os.Args` in package `init` and call
`os.Exit`. Every workflow binary, example and test binary carries two hidden
worker modes that run before `main`.

### 4. nitpick — consecutive turns on one session hand off the session lease by timing

Unchanged from rounds 02 to 04. `Run` rejects with "another turn owns this
session" (`agent.go:189-195`) while the previous worker is still in its
deferred unlock after writing `result.json` and calling `Notify`. The same
window applies to `Adapter.Close` submitting its close turn right after the
active turn's result appears. A lost race surfaces as a failed `RunTurn` or
`Close` with no retained operation. A short bounded wait on the session lock
would remove it.

## Observations below the reporting threshold

- A turn rejected before acceptance (closed session, missing session,
  busy session) leaves `turn_<id>/` holding `owner.lock` and an empty
  `events.jsonl` with no `request.json`. `Vacuum` removes such directories by
  their own mtime after seven days (`files.go:398-401`) and README says so.
  Seen in the reproduction (`turn_…A2` remained after the rejection).
- `fork.json` and `close.json` are still written as per-session pointers
  (`agent.go:586`, `:627`) but nothing reads them any more. They satisfy
  "Gimbal records IDs before submission" and are harmless; they could go.
- `executor.Wait` passes the reader's byte offset as `After`; a partial
  trailing line makes the server return at once and the reader spin through
  socket round trips until the line completes. `json.Encoder.Encode` writes
  each event in one `write`, so the window is a single syscall.
- The supervisor still SIGKILLs the process group after a normal exit
  (`command.go:197-201`). A freed pgid reused by an unrelated group between
  reap and kill is theoretical. Round 03 nitpick 4, still below threshold.
- Agent `Run` emits the descriptor before native session setup
  (`agent.go:284` precedes `CreateSession`/`LoadSession`); the files named
  exist and setup failures land in `result.json`, so the contract holds.
- `runCapturedCommand` leaves the empty `commands/<graph-id>/` directory with
  only the `.executor.json` manifest beside `command_<ulid>/` (round 03).
- OpenCode stays on its direct path (`binding.go:52-53`); Gemini goes through
  the executor (`binding.go:50-51`, `agent.go:87-88`), as Tyler asked.
- `taskdaemon.New` takes the executor root; `cmd/gimbal/daemon.go:59` exposes
  `--executor-state-dir`, `messaging.go:62` and `invocation.go:54` pass `""`
  (home default), and all tests pass `t.TempDir()`.
- `Acknowledge` is gone from `backend.Execution` and the `ack` call from
  `remote_execution.go`; retention is the executor vacuum's alone, as the
  worklog decision says.

## Outcome

no material findings remain
