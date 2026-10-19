# Issue 611 release follow-up: adversarial review, round 01

## Target

Uncommitted worktree on `codex/issue-611-release-fix` (HEAD `955b7e02`,
`chore: release v0.16.0`), compared with `/tmp/issue611-release-fix.md`, the
qualification log in `/tmp/issue611-release-qualification.log`, the merged PR
text in `/tmp/issue611-pr.md`, AGENTS.md and `docs/definition-of-done.md`.

Diff: 10 modified files, one new test file (`internal/effects/connection_test.go`),
26 insertions, 28 deletions. All Go plus one worklog line under `ephemeral/`.

`origin/main` is not resolvable in this worktree (`git fetch` was not run to
avoid touching shared state), so "diff from origin/main" was taken as the
worktree diff against HEAD, which is the v0.16.0 release commit.

## What the gate reported

`just lint-go` (golangci-lint v2.14.0, pinned in `release.yml`) failed with
23 issues: 21 `errcheck`, 1 `ineffassign`, 1 `unused`. The remaining gate steps
(`go vet`, `just test`, `just fmt-check`, `just check-web`) never ran.

## Evidence inspected

- Full diff, every hunk, with the surrounding code for each silenced return.
- `golangci-lint run ./...` with the pinned v2.14.0 (via `go run ...@v2.14.0`):
  **0 issues**. This is the exact gate that failed.
- `golangci-lint run ./...` with the locally installed v2.15.0: 3 `errcheck`
  issues, all pre-existing channel receives in test files not touched by this
  diff (`cmd/gimbal/release_plugin_archive_test.go:139`,
  `internal/environments/connect_watch_test.go:194`,
  `internal/pi/agent/agent_test.go:221`). See finding 4.
- `go vet ./...`: clean.
- `go test -count=1` on the touched packages (`effects`, `executor`,
  `agentexecutor`, `commandexecutor`, `generate/...`, `devgenerate`, `pi`): pass.
- `go test -count=1 ./...`: every package passes except `web`, whose four
  failures all read `skgo: reading skgo.manifest.json from the build: file does
  not exist`. `web/build` holds only `.gitkeep` here; CI builds the app before
  testing. Environmental, not caused by this diff.
- `gofmt -l` on the touched files: clean.
- Call sites of the `state` operation and of `writeWorkflowPages`
  (`grep`), the history of the `value = Reply{}` line (`git log -S`), and the
  bodies of each adapter's `DetachSession`/`Close`.

## Findings

### 1. The `ineffassign` was a real behavior bug, and the fix is correct (confirmed)

`git show 484857be -- internal/effects/connection.go` shows that removing the
`ack` case deleted its `case` and `err =` lines but left its `value = Reply{}`
line, which fell into the `state` case directly below. Since v0.16.0 every local
`state` call returned an empty reply, so `backend.State.Pending` was always 0
and `Error` always empty.

Consequence, not just lint noise: `internal/runner/task_cleanup.go:45` asks
`state` on a local connection and, when `Pending == 0`, records "remote
resources are released" and **skips `Cleanup`**. `orphanCall` at line 70 likewise
reported nothing pending. So an orphaned run with pending native cleanup on a
local task connection was marked clean and its cleanup never requested. That
is exactly the issue's "must return the actual pending-cleanup state instead of
an empty acknowledgement", and the one-line deletion restores it.

`TestLocalStatePreservesPendingCleanup` drives `effects.Call` with an empty
endpoint and a stub `Execution` returning `Pending: 3`, and asserts both fields
survive the JSON round trip. It fails against HEAD (the assignment would zero
them) and passes now. It covers the dispatch regression the worklog names.

Nit, not blocking: the consequence in `task_cleanup.go` is not covered. The
existing `task_cleanup_test.go` exercises the HTTP (remote) path through a
handler that encodes `effects.State`, so the local-connection branch that
skips cleanup on `Pending == 0` has no test. The issue asked for the dispatch
regression to be covered; it is. Mentioning this so nobody mistakes the new
test for coverage of the skipped-cleanup decision.

### 2. Cleanup-only versus durable returns: classification is right (confirmed)

Each of the 21 `errcheck` sites was checked against the rule "cleanup-only
return values handled explicitly; durable write errors remain reported".

Durable, now reported:

- `internal/executor/files.go` `Write`: previously `defer dir.Close(); return
  dir.Sync()`, which silently dropped the directory close error. Now
  `errors.Join(dir.Sync(), dir.Close())`. This is the one site where the
  previous code could lose a durable-write error, and it is now reported.
  Good.

Cleanup-only, correctly made explicit with `_ =`:

- `agentexecutor/agent.go:266` `events.Close()`: the events log is
  `Sync`ed at line 389 and that error joins `observationErr`. Close after sync
  is cleanup.
- `agentexecutor/agent.go:307` fork-parent `DetachSession`: for Codex this
  unregisters the thread subscription (app-server unloads it after the
  inactivity grace), for Claude it is a map delete, for Pi it releases memory
  while retaining history. None of these write durable state; failure leaves a
  resource loaded, not data lost. The turn's own session detach at line 360
  still joins into `turnErr`, which is the right asymmetry.
- `agentexecutor/agent.go:414` read-side `file.Close()` in `ReadEvents`.
- `commandexecutor/command.go:146,151` `out`/`stderr` close: both are
  `Sync`ed at line 219 with the error joined before the result is written.
- `devgenerate/commands.go:83` scratch `RemoveAll`.
- `executor/files.go`: temp-file `Remove` after rename (always fails on the
  success path by design), `f.Close()` on the `Flock` failure path, pipe and
  log closes in `Launch`, listener close on the chmod failure path, connection
  closes in `serve`/`Call`, startup-descriptor closes.
- `generate/devgen/main.go:20` generator close, `pi/executor.go:56` read-side
  source close. The Pi write side (`file`) already joined `io.Copy` and
  `file.Close()` before this change.
- `agent_test.go:40`: `f.Close()` now checked. Minor ordering oddity: the
  close error is checked before the earlier write error, so a double failure
  would surface the close. Harmless in a test.

No site was silenced that should have been reported, and no previously
reported error was lost. Execution behavior is unchanged: every `_ =` replaces
a `defer x.Close()` or bare call whose return was already discarded.

### 3. `writeWorkflowPages` removal (confirmed)

`grep` finds no callers in code or tests; `writeWorkflowPagesResult` is the
only remaining entry and is used. Deletion is the right fix under "no shims".

### 4. Forward risk outside this diff: the gate will fail again on golangci-lint v2.15 (observation)

v2.15.0 `errcheck` flags bare channel receives (`<-wait`, `case <-doneB:`) in
three test files this diff does not touch. CI pins v2.14.0 so the current gate
passes, but the first bump of `golangci/golangci-lint-action`'s `version`
reproduces this release failure with different lines. Not a requirement of
this issue; file it as an issue rather than widening the diff.

### 5. Proof is not yet written anywhere I was given (gap, must be closed before merge)

The issue says repository instructions and `docs/definition-of-done.md`
govern acceptance, and AGENTS.md says proof is "running the real thing
yourself and saying what you saw". The only proof-shaped text in the follow-up
is the worklog line ("Run the actual release lint gate before merging"), which
is an instruction, not an observation. No commit, PR body, or chat report was
supplied to me.

What the PR body must state, in the author's own words from their own run:
`just lint-go` (or `golangci-lint run ./...`) at **v2.14.0** reporting 0
issues; `go vet ./...` clean; `go test ./...` passing; and that the new
`effects` test fails on HEAD and passes with the change. My runs above saw all
of that, so the claim is true; it just has to be the author's report. The
worklog entry in `ephemeral/` is fine as a note and is not a substitute.

### 6. Concurrent edits seen in this worktree (observation, not part of the reviewed diff)

During the review, `command_test.go` at the repository root briefly showed an
uncommitted change raising the `RunCommand` cancellation timeout from 300 ms
to 2 s, with the comment "Leave startup time for the independent executor",
then reverted. Three generated web files were also rewritten by the
`internal/generate` tests during my full-suite run and later restored. The
tree now matches the diff described above.

If the `command_test.go` change lands in this follow-up, it is a behavior
signal, not a lint fix: it says a command with a 300 ms deadline can no longer
preserve the bytes it already wrote, because executor startup eats the budget.
That belongs in the PR text as an observed limitation, or in its own issue,
rather than arriving silently as a widened test timeout.

## Repository-rule check

- No new exported names, no wrappers, no compatibility shims. Pass.
- `ephemeral/` receives only a worklog line. Pass.
- Nothing is committed yet. The commit and PR title must be `fix:` so the
  release path cuts the patch that v0.16.0's failed qualification owes.
- Live runs: none needed; this diff changes no execution path, and the
  v0.16.0 native-harness proof in `/tmp/issue611-pr.md` still describes the
  shipped behavior.

## Verdict

**Accept the code as is.** The three lint classes are fixed at their root, the
one durable-write site is now reported, the restored `state` reply fixes a
real skipped-cleanup bug, and the gate that failed passes at its pinned
version. Before merge: write the proof report (finding 5) and decide whether
the stray `command_test.go` timeout change (finding 6) is in or out. File
finding 4 as an issue.
