# Adversarial review: issue 611, round 03

Date: 2026-10-18. Reviewer: Claude Fable 5.1 (claude-fable-5-1), read-only.

## Review target

The complete current work on branch `codex/issue-611-executors`: commits
`8b6e84bd feat: run commands and agent turns in independent executors`,
`662f4f7c fix: preserve executor cleanup and include Gemini turns`,
`18493175 fix: retain session controls after executor vacuum`, plus the
uncommitted working tree, reviewed against `ephemeral/issues/611.md` (with
its conversation clarifications), the PR description in
`/tmp/issue611-pr.md`, and `AGENTS.md`. The tree changed while the review
ran: `internal/agentexecutor/agent.go` gained `finishInterruption` and the
`backend.DeliveryUncertain` branch in `Run` (uncommitted), with a new test and
a worklog decision. Findings cite the files as they stood at the end of the
review.

The caller's instructions contained no scope narrowing; "do not edit any other
files" is an operating constraint and was honored. Nothing outside this file
was written. The review itself wrote nothing into the tree, but running the
`internal/taskdaemon` tests did write into `~/.gimbal/executors` (finding 2).

## Evidence inspected

- Issue text, PR description, worklog, `AGENTS.md`, rounds 01 and 02.
- Full current text of `internal/executor/files.go` (+tests),
  `internal/commandexecutor/command.go` (+tests),
  `internal/agentexecutor/agent.go`, `cli.go` (+tests), `pi/executor.go`
  (+tests), `claude/executor.go`, `codex/executor.go`, `agy/executor.go`
  (+test), `cmd/gimbal-command/main.go`.
- Diffs against `main` of `command.go`, `cmd/gimbal/main.go`, `README.md`,
  `AGENTS.md`, `Justfile`, `.goreleaser.yaml`,
  `scripts/sign-notarize-macos-cli.sh`, `backend/*`, `internal/binding/*`,
  `internal/effects/connection.go`, `internal/taskdaemon/*`,
  `remote_execution.go`.
- Surrounding code: `service.go` (local services), `session.go:380-440`
  (turn ending and `DeliveryUncertain`), `codex/codex.go:340-365`, the four
  native `Close` implementations, `internal/observation/artifacts.go`
  (artifact path prefix check), `scripts/release-cli.sh` (target rendering),
  `events.go` `NativeRef` tag.
- Ran: `go build ./...` (only the pre-existing `plugins/*` main-less
  errors), `go vet` on the executor packages, the four adapters, `taskdaemon`,
  `binding`, `cmd/...` and the root package (clean), and
  `go test -count=1 -race` for `internal/executor`,
  `internal/commandexecutor`, `internal/agentexecutor`, `pi`, `agy`,
  `internal/taskdaemon`, `internal/binding`. All passed, including the
  uncommitted `TestUnconfirmedNativeStopRemainsOwnedUntilCleanupConfirms`.
- Inspected `~/.gimbal/executors` after the test run to confirm finding 2.
- Not verifiable here: the live harness runs, caller-loss survival, steering,
  memory figures and the Bob remote-route demonstration. The PR description
  reports them in the form `AGENTS.md` asks for, with model aliases named
  (`luna`, `haiku`, Pi `glm-flash`, Gemini `flash`). I take that report at
  face value and did not re-run it.

## Round 02 findings, re-checked

1. Session controls after turn vacuum: addressed. `Run` removes
   `active.json` after publishing the result (`agent.go:361`), `AssertOwner`
   opens `owner.lock` without `O_CREATE` so it never recreates a directory
   (`files.go:144`), and `Steer`/`Close` treat a missing turn directory as
   "no active turn" (`agent.go:518-520`, `567-578`). Covered by
   `TestIdleSessionCanSteerAndCloseAfterTurnVacuum`.
2. Library `init()` hooks: unchanged (finding 5 below).
3. `GIMBAL_EXECUTABLE`: removed. `Submit` uses `os.Executable()` only
   (`agent.go:158`).
4. Fork turn identity not recorded: addressed. `Fork` writes `fork.json`
   before submitting (`agent.go:542`).
5. Session lease hand-off by timing: unchanged (finding 6 below).

## Findings

### 1. issue — every bridge command, every bridge service and every deadline-less workflow command or turn now dies after twenty minutes

Before this branch neither the local `RunCommand` path nor the task daemon
imposed a timeout; a command ran until it exited or its context ended. Now:

- `internal/taskdaemon/process.go:84` submits every bridge process with
  `Timeout: 20 * time.Minute`, including `req.Service` processes. The
  supervisor's `exec.CommandContext` kills the group at the deadline
  (`command.go:117`, `148`). A remote service that has run for twenty
  minutes is killed, reported `TimedOut`, and because
  `result.Interrupted && !saved.Forced` clears the error
  (`process.go:116-118`) the bridge reports a quiet interruption instead of
  "required service exited unexpectedly". Runs longer than twenty minutes
  on a remote host lose their services silently.
- `command.go:279-282` defaults a workflow `RunCommand` to twenty minutes
  whenever the context carries no deadline. Gimbal run contexts carry none
  (no `WithTimeout`/`WithDeadline` on the run path in `run.go`/`scope.go`
  apart from cleanup). A long build or test suite now fails with
  `context deadline exceeded` and `Interrupted: true`.
- `agent.go:470-473` does the same for every `Generate` turn through Pi,
  Claude, Codex and Gemini. None of those adapters had a turn deadline
  before (`agy` has a 24-hour backstop only). A Codex turn that works for
  more than twenty minutes is now cancelled.

The issue says "timeout [has] defaults" for the executor CLI contract and
separately says "Preserve adapter behavior". A default for a human typing
`gimbal-command` is reasonable; Gimbal-originated submissions should pass the
context's deadline when there is one and otherwise no deadline (or one the
workflow set), not a silent twenty minutes. The PR description does not
mention this change. Fix: let `Timeout <= 0` mean "no deadline" for
Gimbal-originated submissions (the worker already uses
`context.WithTimeout` only because a value is always present), or pass a
very large explicit value from `command.go`, `agent.go` and `process.go`
while keeping the CLI default.

### 2. issue — the bridge and its tests write executor state into the real home directory, and two things under that root are never vacuumed

`internal/taskdaemon/process.go:84` uses `executor.Root("")`, which is
`~/.gimbal/executors`, and `Daemon` has no way to choose another root. The
package tests therefore run real commands against the user's home directory.
Observed after `go test -count=1 -race ./internal/taskdaemon/`:

```
~/.gimbal/executors: 453 entries
~/.gimbal/executors/submissions: 443 files
command_01M58NKHYQT1EZHWTWEB7VQDW4/request.json:
  {"ID":"command_01M58NKHYQT1EZHWTWEB7VQDW4","Command":"zsh","Args":["-c","touch \"$ENV_FILE.tmp\"; ... exec sleep 60"],
   "Workdir":"/var/folders/.../TestSuccessfulAncestorCleanupClearsFailedChildOwnership.../002", ...
```

Every `go test ./...` on a developer machine, and every CI run, now leaves
`command_*` directories and a `sleep 60` supervisor behind in the home
directory. The command directories do vacuum after seven days once another
worker runs, but two things never do:

- `submissions/<id>.json` (`process.go:88-92`): one file per bridge command,
  forever. `Vacuum` only looks at directories containing `result.json`
  (`files.go:395`), so this directory grows without bound.
- `diagnostics.log` (`files.go:206`): every worker's stdout and stderr are
  appended to one file per root, never rotated or truncated.

Give the daemon a root (a field set by `New`, which the tests point at
`t.TempDir()`), and either stop writing per-command submission records that
nothing reads, or vacuum them alongside their operation. The README's
retention text ("removes terminal operations after seven days") should say
what is not covered.

### 3. issue — the uncommitted `finishInterruption` retries native cleanup forever and permanently closes the session after one cancelled turn

`agent.go:329-334` and `finishInterruption` (`agent.go:604-621`): when the
native adapter returns `*backend.DeliveryUncertain` (today only Codex,
`codex/codex.go:360`, after an interrupt it could not confirm), the worker
calls `a.Close` every second until it returns nil. Three consequences:

- There is no bound. If the shared Codex app-server is gone for good, `Close`
  returns the same error forever (`codex.go:499-501`), the worker never
  writes `result.json`, and every reader (`RunTurn`'s post-cancel `Wait`,
  `Adapter.Close`, `--lookup`) spins for as long as its own context allows.
  `AssertOwner` sees a live owner, so nothing ever panics or reports. The
  issue forbids an uncertain-outcome branch; it does not ask for an
  unbounded one. The worklog decision ("retries native Close ... until
  cleanup confirms cessation") describes this, but a retained owner that
  never terminates is a hang, not ownership. Bound the retries by the
  accepted deadline (or a documented ceiling) and then publish a result that
  says cleanup is unconfirmed, as the command supervisor already does for a
  forced stop (`command.go:204-206`).
- `released` forces `closed.json` (`agent.go:347-348`). One cancelled Codex
  turn, for example the loser of a `Group` race, closes the session for every
  later `RunTurn` ("session is closed", `agent.go:253-255`). Before this
  branch the runtime recorded the turn as `Unresolved` and left the session
  usable (`session.go:419-426`). The native thread is archived by `Close`,
  so closing is defensible, but the workflow is not told why its next turn
  on that session fails.
- `session.go:394` and `419` can no longer match `*backend.DeliveryUncertain`
  for these harnesses because the error crosses the file boundary as a
  string (`agent.go:432-433`). That is the intended replacement, but the
  `Unresolved` `TurnEnded` path is now dead for Pi, Claude, Codex and
  Gemini and only OpenCode can reach it. Worth a comment at `session.go:419`
  or deletion if OpenCode cannot raise it either.

This change is not in any commit yet and is not described in the PR.

### 4. nitpick — the supervisor SIGKILLs the process group after every normal exit

`command.go:185-189`: after `cmd.Run` returns, the supervisor kills the whole
group unconditionally. Previously the local `RunCommand` path did not set
`Setpgid` and a backgrounded child of a workflow command survived the
command's exit. The comment explains the intent and the issue's "owned
process group" wording supports it, so this is a deliberate change, but it is
a behavior change for existing workflows that is not in the PR description or
the `RunCommand` doc comment (`command.go:97-112`).

### 5. nitpick — library `init()` hooks still take over any binary that links Gimbal

Unchanged from rounds 01 and 02: `commandexecutor/command.go:46-60` and
`agentexecutor/agent.go:27-41` inspect `os.Args` in package `init` and call
`os.Exit`. The root package imports `commandexecutor` and `binding` imports
`agentexecutor`, so every workflow binary, example and test binary carries two
hidden worker modes that run before `main`. The sibling `gimbal-command`
binary already shows the explicit shape.

### 6. nitpick — consecutive turns on one session still hand off the session lease by timing

Unchanged from round 02: `Run` rejects immediately with "another turn owns
this session" (`agent.go:193-198`) while the previous worker is still in its
deferred unlock after writing `result.json`. I could not make it fail; a
short bounded wait on the session lock would remove the dependence on
timing.

## Observations below the reporting threshold

- `Vacuum` and `Claim` can interleave at the seven-day boundary: `Claim` can
  observe `request.json` under the vacuum's lease, return "retained matching
  operation", and the caller then waits on files that are removed a moment
  later; `AssertOwner` finds no `request.json` and returns silently, so the
  caller waits until its own context ends. Needs a resubmission of a
  seven-day-old ID within the same millisecond as the hourly vacuum.
- `.goreleaser.yaml` lists `darwin_arm64` for `gimbal-command`;
  `scripts/release-cli.sh:36` rewrites every such line, so Linux archives
  also carry it. `scripts/release-macos.sh` does not bundle `gimbal-command`
  into the app; the app's `gimbal` falls back to re-executing itself
  (`command.go:87-90`), which works.
- `runCapturedCommand` writes `commands/<graph-id>/stdout.log.executor.json`
  beside the capture files it then deletes, so a run directory has an empty
  per-graph-id directory holding only a manifest and a separate
  `commands/command_<ulid>/` holding the output. The artifact route still
  works because both are under `artifacts/commands/`
  (`internal/observation/artifacts.go:40`).
- Local `Service` (`service.go:92-104`) still execs directly rather than
  through the supervisor. Consistent with services being scope-bound; the
  issue named commands and agent turns only.
- `cmd/examples/notebook/main.go:46` constructs `codex.New()` directly and
  bypasses the executor. Pre-existing example.
- `Result.Output []byte` is still base64 in `result.json` (`agent.go:60`).
- Agent executor state for workflow runs lives in `~/.gimbal/executors`,
  not under the run directory, while command state lives under
  `<run>/artifacts/commands`. Sessions must outlive runs, so this is
  reasonable, but `gimbal runs` cleanup will not reclaim agent event files.

## Outcome

material findings remain
