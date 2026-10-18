# Adversarial review: issue 611, round 01

Date: 2026-10-18. Reviewer: Claude Fable 5.1 (claude-fable-5-1), read-only.

## Review target

The uncommitted working tree on branch `codex/issue-611-executors`, reviewed
against `ephemeral/issues/611.md` ("Move native execution ownership into
small, independently running Go executors") and `AGENTS.md`. The tree changed
while the review ran: `internal/agentexecutor/agent.go`, `pi/executor.go` and
`README.md` were edited concurrently, and the earlier `web/` and
`web/build/.gitkeep` changes disappeared. Findings cite the files as they
stood at the end of the review.

The caller's instructions contained no scope narrowing; "do not edit any other
files" is an operating constraint and was honored.

## Evidence inspected

- Issue text, worklog decisions, `AGENTS.md`, `go doc . HarnessAdapter`.
- New packages in full: `internal/executor/files.go` (+tests),
  `internal/commandexecutor/command.go` (+tests), `internal/agentexecutor/agent.go`,
  `cli.go` (+tests), `cmd/gimbal-command/main.go`, `claude/executor.go`,
  `codex/executor.go`, `pi/executor.go`.
- Diffs of `command.go`, `internal/taskdaemon/process.go`,
  `internal/binding/*`, `cmd/gimbal/main.go`, `.goreleaser.yaml`, `Justfile`,
  `scripts/sign-notarize-macos-cli.sh`, `scripts/release-cli.sh`, `README.md`.
- Surrounding adapter code: `codex/codex.go` (ResumeSession, DetachSession,
  Close), `pi/adapter.go` (CreateSession, Fork, Close), `claude/claude.go`
  (CreateSession, Fork, Close), `session.go` event wrapping,
  `internal/observation/http.go` stdout route, `web/command_output_test.go`.
- Ran: `go build ./...` (only the pre-existing `plugins/*` main-less build
  errors, unchanged by this work), `go test` for `internal/executor`,
  `internal/commandexecutor`, `internal/agentexecutor`, `internal/binding`,
  the root package, `internal/taskdaemon`, and `./web -run CommandOutput`.
  All passed.
- Live check with a built `gimbal-command` against a scratch state dir:
  one descriptor line on submit; the command finished and its output was
  retrievable after the caller exited; a conflicting resubmission failed; an
  accepted operation with no owner and no result panicked on `--lookup`.
- Not verifiable here: real Pi, Claude and Codex turns, session continuation
  across executor processes, the remote-route demonstration, and executor
  memory. Nothing in the tree reports them, which is expected under AGENTS.md
  (proof goes in the PR), so the PR must carry that report.

## Findings

### 1. issue — the caller-side adapter never releases native sessions

`internal/agentexecutor/agent.go:491-516`: `Adapter.Close` only cancels an
active turn and returns. The runtime calls `Close` when the owning scope ends,
and `HarnessAdapter.Close` is documented as "releases whatever the adapter
holds for the session". Before this change that meant:

- Codex archived the thread (`codex/codex.go:489-526`, `thread/archive`,
  `unregisterThread`, `onArchive`).
- Pi disposed the in-process agent and removed its owned temp root
  (`pi/adapter.go:420-440`, `os.RemoveAll(s.root)`).

Now neither happens anywhere: the executor process only calls
`DetachSession` (`agent.go:296`), which by design keeps the thread and the
Pi directory. The issue asks to "preserve adapter behavior", and the worklog
decision "session continuation state is not vacuumed" is about not deleting
state still needed for continuation, not about never ending a session whose
scope has ended. Impact: every workflow session leaves an unarchived Codex
thread in app-server and a `gimbal-pi-*` directory under the OS temp dir,
with no path that ever reclaims them. Workflows that fork many sessions (bake
offs) will accumulate them without bound.

### 2. issue — daemon service stop regressed from SIGINT with grace to immediate SIGKILL

`internal/taskdaemon/process.go` at HEAD (lines 115-137) stopped a task by
sending SIGINT to the process group, waiting up to ten seconds, and only then
SIGKILL, reporting "external resource cleanup unconfirmed" when the grace
expired. The new `stop` (`process.go:95-99`) forwards to
`commandexecutor.Cancel`, and the supervisor's only cancellation is
`cmd.Cancel` = `SIGKILL` to the group (`internal/commandexecutor/command.go:139-145`).

The issue did not ask for daemon stop semantics to change, and the supervisor
is the only place that could preserve them. Impact: a required service (dev
server, database) stopped by the daemon never receives an interrupt and cannot
clean up ports, lock files, or child resources; the former "cleanup
unconfirmed" signal to the workflow is gone. `RunCommand` cancellation also
moved from killing one process to killing the group, which is an improvement,
but the daemon case is a regression.

### 3. issue — ordinary I/O errors after acceptance are converted into permanent invariant violations with no recovery

After `Accept`, the owner panics instead of recording a terminal failure on:
`agent.go:236-241` (deadline/active.json write), `agent.go:284` (event
append), `agent.go:292-294` (post-turn `SaveSession`), `agent.go:307-312`
(`events.Sync`, result write); `command.go:129-131`, `164-166`.

Any of these leaves `request.json` without `result.json` and no lease. The
consequences compound:

- `Vacuum` only removes directories that have a `result.json`
  (`internal/executor/files.go:359-361`), so the directory is retained
  forever.
- Every later `--lookup`, `Wait`, `Cancel`, `Steer`, `Close`, and `Claim` of
  that ID panics (`files.go:135-147`, `files.go:407-411`). Confirmed live:
  a `--lookup` of such a directory prints the panic. The same panic fires
  inside the workflow process through `executor.Wait` (`files.go:318-332`)
  and `Adapter.Close` (`agent.go:507-510`).
- Nothing documents how an operator clears the state; the README section says
  only that active operations are never vacuumed.

The issue wants a panic when an accepted operation has lost its live owner and
has no saved result. A `SaveSession` or `Sync` failure is not owner loss; the
owner is alive and can persist a terminal failure, which is what "persist
terminal results before announcing completion" asks for. Reserve the panic for
real loss and record everything else.

### 4. issue — acceptance failures reach the caller as "executor startup: EOF"

`internal/executor/files.go:200-216`: the parent reads one descriptor from the
startup pipe; when the worker fails before `Startup` it exits 1 and the parent
returns `executor startup: EOF (diagnostics: …/diagnostics.log)`. The worker's
actual reason goes to a shared, append-only `diagnostics.log` under the root
(`command.go:41-54`, `agent.go:25-38`). Confirmed live: a conflicting
resubmission printed exactly that EOF message, with "operation identity
conflicts with accepted inputs" only in the log file.

Reasons lost this way include: conflicting inputs, "another turn owns this
session", missing or conflicting `session.json`, invalid identity, and the
invariant panic in `Claim`. The issue says "conflicting inputs fail", and
`TestConcurrentSubmitExecutesOnceAndRetainsCompletedOutput`
(`command_test.go:43-46`) only checks that an error exists, so the test does not
catch it. Impact: a workflow or operator cannot tell a conflict from a crash
without opening a log that every operation under `~/.gimbal/executors` appends
to and nothing ever truncates. The startup pipe already carries JSON; a
`{"state":"rejected","error":…}` line, or the worker writing its error to the
pipe, would fix it.

### 5. nitpick — library `init()` hooks that take over the process

`internal/commandexecutor/command.go:41-54` and
`internal/agentexecutor/agent.go:25-38` inspect `os.Args` in package `init`
and call `os.Exit`. Because the root `gimbal` package imports
`commandexecutor`, every workflow binary, every example, and every test binary
that links Gimbal silently gains a `--gimbal-command-worker` mode that runs
before `main` and before the importing packages' own `init`. The re-exec design
is deliberate and documented in the comment, but an explicit entry point
called from `main` (as `cmd/gimbal-command/main.go` already does) would keep
the control flow visible and match "as simple as possible".

## Observations below the reporting threshold

- `executor.Wait`'s server-side comparison uses file size against the
  reader's consumed offset (`files.go:282-292`, `agent.go:391`). While a
  partial trailing line exists the wait returns immediately and the reader
  spins until the line completes. Brief in practice; worth a glance.
- `Result.Output []byte` is base64 in `result.json`
  (`agent.go:57`), so the retained terminal result is not readable as JSON
  text by hand.
- `request.Timeout = time.Until(deadline)` (`command.go:279`,
  `agent.go:420`) makes the recorded request differ on every submission, so
  `Match` can never deduplicate a Gimbal-originated resubmission. Harmless
  today because Gimbal mints a fresh ID per call, but it means the
  "records IDs before submission" manifests are never read by anything.
- Socket paths land in the OS temp dir at roughly 98 characters on macOS,
  near the 104-byte Unix socket limit.

## Outcome

material findings remain
