# Adversarial review: issue 611, round 04

Date: 2026-10-18. Reviewer: Claude Fable 5.1 (claude-fable-5-1), read-only.

## Review target

The complete current work on branch `codex/issue-611-executors`: commits
`8b6e84bd`, `662f4f7c`, `18493175`, `5896ee69` plus the uncommitted working
tree (37 modified files, mostly the round 03 follow-ups: deadline inheritance,
daemon executor root, per-operation diagnostics, submission-record vacuum),
reviewed against `ephemeral/issues/611.md` with its conversation
clarifications, the PR description in `/tmp/issue611-pr.md`, and `AGENTS.md`.
The caller also asked for an adjudication of `/tmp/issue611-adjudication.md`,
a source-based challenge to round 03 finding 3; that is in its own section.

The caller's instructions contained no scope narrowing. "Do not edit any other
files" is an operating constraint and was honored: nothing outside this file
was written into the tree. The reproduction below ran against a disposable
root in the session scratchpad.

## Evidence inspected

- Issue text, PR description, adjudication, worklog, `AGENTS.md`, round 03.
- Full current text of `internal/executor/files.go`,
  `internal/commandexecutor/command.go`, `internal/agentexecutor/agent.go`,
  `internal/agentexecutor/cli.go`, `internal/taskdaemon/process.go`; the
  diffs against `main` of `internal/taskdaemon/daemon.go`, `client.go`,
  `command.go`, `remote_execution.go`, `backend/backend.go`, `README.md`,
  `AGENTS.md`, `cmd/gimbal/*`, `internal/invocation/invocation.go`,
  `internal/binding/*`; the test files of the three executor packages and
  the `taskdaemon` tests' constructor calls.
- Surrounding code: `codex/codex.go:318-365` (unconfirmed stop) and
  `485-525` (`Close`), `session.go:385-430` (`DeliveryUncertain` handling),
  `internal/effects/client.go:179-221` (remote `Wait` producing
  `DeliveryUncertain`), `internal/taskdaemon/daemon.go:195-250`.
- Ran `go build ./...` (only the pre-existing `plugins/*` main-less errors),
  `go vet` on the executor packages, `taskdaemon`, `binding`, `cmd/...` and
  the root package (clean), and `go test -count=1 -race` for
  `internal/executor`, `internal/commandexecutor`, `internal/agentexecutor`,
  `internal/taskdaemon`, `internal/binding`, `internal/effects`, `pi`,
  `agy`: all passed. `~/.gimbal/executors` had the same 461 entries before
  and after the run; nothing new landed in the home directory.
- Reproduced finding 1 with a built `gimbal` binary against a scratch
  `--state-dir` (transcript in the finding).
- Not verifiable here: the live harness runs, caller-loss survival,
  steering, memory figures and the Bob remote route. The PR description
  reports them in the form `AGENTS.md` asks for, naming `luna`, `haiku`,
  Pi `glm-flash` and Gemini `flash`. Taken at face value, not re-run.

## Round 03 findings, re-checked

1. Twenty-minute default on every Gimbal-originated command and turn:
   addressed. `command.go:278-280` and `agent.go:499-501` pass the context
   deadline only; `process.go:86` forwards `req.Timeout`; `remote_execution.go`
   and `taskdaemon/client.go` carry the remaining deadline through
   `backend.Process.Timeout` / `backend.Turn.Timeout`; `daemon.go:213-216`
   applies it. The default stays on the two CLIs (`cli.go:35`,
   `command.go:256`). Covered by `TestNoImplicitDeadlineForProgrammaticSubmission`
   and `TestProgrammaticAgentOperationHasNoImplicitDeadline`. README updated.
2. Bridge and tests writing into the real home directory: addressed.
   `taskdaemon.New` takes the executor root (`daemon.go:66-75`) and every test
   passes `t.TempDir()`; `Launch` now writes `diagnostics.log` into the
   operation directory (`files.go:206`, `agent.go:160`, `command.go:88`), so
   it is vacuumed with the operation; `Vacuum` removes the root-level and
   per-session submission records alongside (`files.go:415-418`).
3. `finishInterruption`: adjudicated below. Downgraded to nitpick 2.
4. Supervisor SIGKILLs the group after normal exit: unchanged, still not in
   the `RunCommand` doc comment (README covers it under "Independent
   executors"). Remains below the reporting threshold.
5. Library `init()` hooks: unchanged (nitpick 3).
6. Session lease hand-off by timing: unchanged (nitpick 4).

## Adjudication of round 03 finding 3

The adjudication is correct on the substance and round 03 was wrong on its
example.

- The "stopped app-server" example was inaccurate. `codex.Close` treats
  `errDaemonNotRunning` as a successful close (`codex/codex.go:499-501`,
  then falls through to `delete(a.sessions, ...)` and returns nil at
  `codex.go:525`). With the server gone, the first retry confirms, the
  worker publishes a cancellation, and nothing spins. Retracted.
- The retained-owner design matches the issue better than the bounded
  "cleanup unconfirmed" result round 03 suggested. The issue forbids an
  "ordinary uncertain-outcome branch" and says caller loss stops observation,
  not execution. Codex raises `DeliveryUncertain` only when `ctx.Err() != nil`
  and the native turn's cessation is unconfirmed (`codex.go:332-360`), so a
  published "cancelled" would be exactly the uncertain-outcome result the
  issue forbids. The loop terminates when the server either answers the
  archive call or stops, both of which an operator can bring about, and
  `cleanup.json` records every attempt. Accepted.
- Round 03's second bullet (the session is closed with no explanation) is
  addressed: `agent.go:357-366` writes `closed.json` with the reason, and a
  later turn fails with "session is closed: native session had to close to
  confirm an interrupted turn stopped" (`agent.go:258-266`).
- Round 03's third bullet (the `Unresolved` path in `session.go:419` is dead)
  was wrong. `internal/effects.Wait` returns `backend.DeliveryUncertain` for a
  remote turn whose stop or result cannot be confirmed (`client.go:184`,
  `209`, `220`), independent of harness. That path stays live. Retracted.

What remains is a design residue rather than a defect: the retry has no
ceiling and the accepted deadline does not bound it (`finishInterruption`
uses `context.WithoutCancel`, `agent.go:631`). The README documents this.
Recorded as nitpick 2.

## Findings

### 1. issue — `agent-executor --close` on an already-closed session prints a descriptor for an operation that no longer exists, or fails with a file error

`cli.go:54-63`: `--close` calls `Adapter.Close`, which returns nil at once
when `closed.json` exists (`agent.go:593-595`), then reads `close.json` and
prints `executor.Describe(turn dir)`. `Describe` (`files.go:179-193`) only
checks whether `result.json` exists; it never checks `request.json`. Two
cases go wrong:

- The close turn has been vacuumed (seven days after an explicit close).
  `--close` prints `state: running` with a socket path nothing listens on.
  Reproduced with a scratch root: create `sessions/<id>/session.json`, run
  `--close` (prints `completed`), remove the `turn_*` directory as `Vacuum`
  would, run `--close` again:

  ```
  {"state":"running","socket":"/var/folders/.../gimbal-executor-501-6036503e....sock",
   "events":".../turn_01M58PGJ0D0NA8JK2WHHTQ3V8T/events.jsonl",
   "result":".../turn_01M58PGJ0D0NA8JK2WHHTQ3V8T/result.json"}
  ```

  `--lookup` of the same turn correctly says "turn not found". A caller
  that waits on the printed descriptor never returns: `executor.Wait` fails
  to dial, `AssertOwner` sees no `request.json` and returns silently
  (`files.go:138`), and the loop sleeps and retries until the caller's own
  context ends.
- The session was closed by the interruption path (`agent.go:357-366`).
  `close.json` is never written, so `--close` returns "open ...: no such
  file or directory" instead of reporting the closed session.

The issue's protocol says a completed lookup returns `completed` with
retained file locations. Fix: have `--close` answer from `closed.json` when
it exists (a `completed` descriptor with no files, or an explicit "session
already closed" line), and let `Describe` refuse a directory without
`request.json`.

### 2. nitpick — unconfirmed-stop retry has no ceiling and is invisible to readers

`agent.go:627-659`. Accepted per the adjudication above. Two residues:
`Describe` reports a turn in this state as `running` with no hint that it
is stopping (the `cleanup.json` status is not in the descriptor or in
`--lookup` output), and the loop ignores the accepted deadline. Surfacing
`cleanup.json` in `--lookup` would be enough.

### 3. nitpick — library `init()` hooks still take over any binary that links Gimbal

Unchanged from rounds 01 to 03: `commandexecutor/command.go:46-60` and
`agentexecutor/agent.go:27-41` inspect `os.Args` in package `init` and call
`os.Exit`. Every workflow binary, example and test binary carries two hidden
worker modes that run before `main`.

### 4. nitpick — consecutive turns on one session hand off the session lease by timing

Unchanged from rounds 02 and 03. `Run` rejects with "another turn owns this
session" (`agent.go:187-193`) while the previous worker is still in its
deferred unlock after `Notify`. The window is the previous worker's defer
chain against a reader that must see the result, return, and fork a new
process, so it is very small, but a lost race surfaces as a failed
`RunTurn` with no retained operation (the new worker returns before
`Accept`). A short bounded wait on the session lock would remove it.

### 5. nitpick — the local paths compute the forwarded deadline differently from the remote paths

`command.go:279` and `agent.go:500` use `time.Until(deadline)` unclamped;
`remote_execution.go:214` and `taskdaemon/client.go:179` clamp to
`max(time.Nanosecond, ...)`. A deadline that expires between the `ctx.Err()`
check and the submission becomes a negative `Timeout`, which the worker
reads as "no deadline" (`command.go:116`, `agent.go:209`). The caller's
`Wait` returns at once and sends `cancel`, so the outcome is right; the
request file just records no deadline for an operation that had one.

## Observations below the reporting threshold

- `Run` emits the descriptor before native session setup (`agent.go:277`
  precedes `CreateSession`/`LoadSession`), while the issue says "after
  acceptance and setup". The files the descriptor names exist at that point
  and setup failures land in `result.json`, so the contract holds in
  effect.
- `runCapturedCommand` still leaves an empty `commands/<graph-id>/`
  directory holding only the `.executor.json` manifest beside the
  `command_<ulid>/` directory with the output (round 03 observation).
- `Launch` keeps a two-minute read deadline on the startup pipe
  (`files.go:233`); a duplicate submission whose first worker is slow to
  `Accept` is covered because `Claim` spins only until `request.json`
  appears.
- The `released` fallback `return context.Canceled` when `ctx.Err() == nil`
  (`agent.go:648-651`) is unreachable for Codex, the only adapter that
  raises `DeliveryUncertain` (`codex.go:332`). Harmless.
- OpenCode stays on its direct path (`binding.go:50-53`), as Tyler asked.
  Gemini (`agy`) goes through the executor (`agent.go:85-86`), as asked.

## Outcome

material findings remain
