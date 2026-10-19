# Adversarial review: issue 611, round 02

Date: 2026-10-18. Reviewer: Claude Fable 5.1 (claude-fable-5-1), read-only.

## Review target

The complete current work on branch `codex/issue-611-executors`: commit
`8b6e84bd feat: run commands and agent turns in independent executors` plus
the uncommitted working tree (23 modified files, new `agy/executor.go`,
`agy/executor_test.go`, `pi/executor_test.go`), reviewed against
`ephemeral/issues/611.md` and `AGENTS.md`. The tree changed while the review
ran: `internal/agentexecutor/agent.go` (steer control gained a caller
deadline) and `internal/executor/files.go` (`Call` now closes the connection
on context cancellation and carries a `Deadline`). Findings cite the files as
they stood at the end of the review.

The caller's instructions contained no scope narrowing; "do not edit any other
files" is an operating constraint and was honored. Nothing outside this file
was written.

## Evidence inspected

- Issue text including the conversation clarifications (agy migrated,
  OpenCode excluded), worklog decisions, `AGENTS.md`, round 01 review.
- Full current text of `internal/executor/files.go` (+tests),
  `internal/commandexecutor/command.go` (+tests),
  `internal/agentexecutor/agent.go`, `cli.go` (+tests), `agy/executor.go`
  (+test), `pi/executor.go` (+tests), `claude/executor.go`,
  `codex/executor.go`.
- Diffs of `README.md`, `backend/*`, `internal/binding/binding.go`,
  `internal/effects/connection.go`, `internal/taskdaemon/*`,
  `remote_execution.go`.
- Surrounding code: `pi/adapter.go` (`Close`, `sessionState`), `codex/codex.go`
  (`Close`, `DetachSession`, shared app-server comments), `agy/agy.go` and
  `claude/claude.go` `Close`, `command.go` RunCommand executor path,
  `scope.go:255-285` and `run.go:495-512` adapter `Close` call sites,
  `cmd/gimbal/main.go` dispatch, `cmd/gimbal/run_prompt.go`,
  `internal/taskdaemon/process.go` root selection, `events.go` `NativeRef`
  tag.
- Ran: `go build ./...` (only the pre-existing `plugins/*` main-less errors),
  `go vet` on the executor packages, the four adapters, `taskdaemon` and the
  root package (clean), and `go test -count=1` for `internal/executor`,
  `internal/commandexecutor`, `internal/agentexecutor`, `pi`, `agy`,
  `internal/taskdaemon`, `internal/binding`. All passed.
- Ran one scratch test through `go test -overlay` (no file added to the
  tree) to reproduce finding 1; output quoted there.
- Not verifiable here: real Pi, Claude, Codex and Gemini turns through the
  executor, session continuation and close across executor processes with a
  real harness, the remote-route demonstration, executor memory. Nothing in
  the tree reports them, which `AGENTS.md` expects (proof goes in the PR), so
  the PR must carry that report.

## Round 01 findings, re-checked

1. Native sessions never released: addressed. `Adapter.Close`
   (`agent.go:544-584`) now submits a close turn; the executor loads the
   checkpoint in closing mode and calls the native `Close`, which archives the
   Codex thread and removes the Pi root. Pi forks get their own root in
   `SaveSession` so a parent close cannot delete a child's history
   (`pi/executor.go:28-68`, covered by `TestExecutorForkHistorySurvivesParentClose`).
2. Daemon stop regressed to SIGKILL: addressed. `Request.Grace` sends SIGINT
   first and reports `Forced` after the grace expires
   (`command.go:155-182`, `204-206`); the daemon passes ten seconds and
   surfaces the forced message (`process.go:81`, `96-101`). Covered by
   `TestGracefulStopAndForcedStopAreDistinguished`.
3. Post-acceptance I/O errors became panics: addressed. Everything between
   `Accept` and the result write now lands in `Result.Error` or
   `ObservationError`; the only remaining panic is the result write itself
   (`agent.go:358`, `command.go:210`), which is the one failure that cannot
   be recorded.
4. Acceptance failures surfaced as `executor startup: EOF`: addressed.
   `RejectStartup` writes the reason to the startup pipe and `Launch` returns
   it verbatim (`files.go:213-225`, `445-456`); the conflict test now
   asserts the word "conflicts".
5. Library `init()` hooks: unchanged (see finding 2 below).

## Findings

### 1. issue — a session whose last turn was vacuumed can no longer be closed or steered

The session directory keeps `active.json` pointing at the last turn
(`agent.go:263`). `Vacuum` removes that turn's directory after seven days
(`files.go:359-392`) and nothing updates the pointer. Afterwards:

- `Adapter.Close` (`agent.go:557-565`) sees no `result.json`, dials the
  socket, fails, calls `AssertOwner` (which creates an empty directory with
  only `owner.lock` through `Lock`'s `MkdirAll`), and returns the dial error.
  The runtime records a close failure and parks the session in
  `pendingClose` (`scope.go:270-280`); the retry in `run.go:504` fails the
  same way, so the native Codex thread or Pi root is never released.
- `Adapter.Steer` (`agent.go:512-517`) returns the dial error instead of
  `false, nil`.
- The next `RunTurn` succeeds, because `Run`'s `AssertOwner(previous)`
  (`agent.go:202-203`) finds no `request.json` in the recreated directory,
  but it leaves that stub directory behind forever (no `result.json`, so the
  vacuum skips it).

Reproduction (scratch test run via `go test -overlay` against
`internal/agentexecutor`; session created with `Adapter{harness:"claude"}`,
`active.json` pointed at a completed turn whose `result.json` was aged eight
days, then `executor.Vacuum`):

```
Steer after vacuum: landed=false err=dial unix /var/folders/…/gimbal-executor-501-….sock: connect: no such file or directory
Close after vacuum: err=dial unix /var/folders/…/gimbal-executor-501-….sock: connect: no such file or directory
turn dir recreated by AssertOwner: true
```

The issue says session continuation state is never vacuumed and that
"preserve native session identity/history for later turns" must hold without
an idle executor process. The `active.json` pointer is such state, and
`Close` is the only path that releases native resources (round 01 finding 1).
Impact: any session idle past the retention window leaks its native thread
or directory and reports a spurious close failure. Treat a missing turn
directory as "no active turn" in `Close` and `Steer`, or clear `active.json`
when the turn publishes its result.

### 2. nitpick — library `init()` hooks still take over any binary that links Gimbal

Unchanged from round 01: `commandexecutor/command.go:46-60` and
`agentexecutor/agent.go:26-40` inspect `os.Args` in package `init` and call
`os.Exit`. The root package imports `commandexecutor` and `session.go:61`
pulls in `binding`, so every workflow binary, example and test binary gains
two hidden worker modes that run before `main`. An explicit entry point
called from `main`, as `cmd/gimbal-command/main.go` already does, keeps the
control flow visible and matches "as simple as possible".

### 3. nitpick — `GIMBAL_EXECUTABLE` is an unused, undocumented knob

`agent.go:158` reads `GIMBAL_EXECUTABLE` to pick the worker binary. Nothing
in the repository sets it (no test, no script, no README mention). The issue
did not ask for it. Either document it as the way to point a workflow at an
installed `gimbal` binary, or remove it.

### 4. nitpick — fork turn identities are not recorded before submission

The issue says "Gimbal records IDs before submission". `RunTurn` writes
`submissions/<turn>.json` (`agent.go:470-476`) and `Close` writes `close.json`
(`agent.go:575`), but `Fork` (`agent.go:536-537`) mints a turn ID and submits
it with no record. The forked session itself is recorded by `Create`, so
only the fork turn is unrecoverable; worth making the three paths consistent.

### 5. nitpick — consecutive turns on one session hand off the session lease by timing

`Run` returns "another turn owns this session" immediately when the session
lock is held (`agent.go:193-198`). The previous turn's worker releases that
lock in deferred calls after writing `result.json` (`agent.go:358-362`), and
the caller's `Wait` returns as soon as the result file is visible. The caller
then fsyncs its submission record and forks a new process before the new
worker reaches `Lock(sdir)`, so in practice the previous worker has long
since exited; I could not make it fail. A brief bounded wait on the session
lock instead of an immediate rejection would remove the dependence on
timing. Below the threshold on its own; recorded because the failure mode
is a hard error to the workflow with no retry.

## Observations below the reporting threshold

- `Result.Output []byte` is base64 in `result.json` (`agent.go:60`), so the
  retained answer is not readable as text by hand.
- `request.Timeout = time.Until(deadline)` (`command.go:279`,
  `agent.go:467`) makes a Gimbal-originated resubmission never match, so the
  submission manifests are written but never used for deduplication.
- `PeriodicVacuum` runs only inside a live worker (`agent.go:211`,
  `command.go:115`), so a root that receives no new work after its last
  operation, such as a finished run's `artifacts/commands`, is never
  vacuumed. For run directories that is probably the desired behavior (run
  retention governs them); the README's "removes after seven days" is only
  true for roots that keep receiving work.
- The server-side wait compares file size to the reader's offset
  (`files.go:291-300`); while a partial trailing line exists the wait returns
  immediately. Each event is one `Write` call, so this is brief.

## Outcome

material findings remain
