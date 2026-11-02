# Resource-lifetime acceptance plan review — round 2

Target: the amended `ephemeral/plans/resource-lifetime-acceptance.md` (untracked
draft), together with the unchanged implementation at
`cd4ed425c0258d3ba21870e895b1c644f6e80a7a` on `codex/codex-resource-lifetime`
(PR #665, still draft at that head), the worklog additions in
`ephemeral/worklog/codex-resource-lifetime.md`, and
`ephemeral/reviews/resource-lifetime-acceptance-self-review.md`. Authoritative
sources are the same as round 1: repository and `~/.codex` `AGENTS.md`,
`docs/definition-of-done.md`, `ephemeral/plans/codex-resource-lifetime-proof.md`,
the worklog, review rounds 1–3 of the repair, the PR body, and Tyler's accepted
boundary as restated by the caller. No caller instruction narrowed subject
matter. Read-only; no runtime experiment, build, or service action was performed.

## Evidence inspected

- Round-1 findings against the amended plan text, section by section.
- Plan-cited harness surface, verified in code: `gimbal run review` with
  `--goal` (field flag, `internal/workflows/review/review.go:33`),
  `--code-review` (role flag, `cmd/gimbal/workflows.go:76–80`), `--instance-dir`,
  `--project`, `--follow` (`workflows.go:88–101`); `gimbal cancel RUN --work-dir`
  (`cmd/gimbal/cancel.go`). The review workflow is one session and one
  `AgentCall` (`workflow_gen.go:18–19`), so the sequential cases are sequential.
- Topology statements, verified in code: per-turn `--gimbal-agent-worker`
  (`internal/agentexecutor/agent.go:164–184`), `codex.New()` only inside the
  worker (`agent.go:99`; `internal/binding/binding.go:48–57`), `go cmd.Wait()`
  reap (`internal/executor/files.go:234`), between-turn `SaveSession` +
  `DetachSession` versus scope-end `Close` (`agent.go:366–378`), executor
  `LoadSession` → `ResumeSession` on every later turn (`codex/executor.go:21–32`,
  `agent.go:298, 327–329`), so production conversations exercise detach/resume
  with the same native identity and expose it as `native_session`.
- Alias resolution: `luna` is a family with versions 5.6 and 6
  (`internal/modelalias/aliases_gen.go:18–19`); resolution happens at the
  invocation layer (`internal/invocation/invocation.go:98`), so the live tests
  that pass a model straight to the adapter would resolve the alias themselves
  via `modelalias.Resolve`, as the plan requires; the newest resolution is
  `gpt-6-luna`, which the plan's "report the resolved model" covers.
- Failure vehicles against the adapter's recovery paths: `connect`/`daemonStatus`
  in the worker's inherited environment (`codex/rpc.go:84–190`,
  `internal/executor/files.go:216` sets no `cmd.Env`), `recoverTurn` on transport
  error while the context lives (`codex/codex.go:402–404`), `releaseSession`
  redialing through `a.connection(ctx,false)` (`codex.go:551–565`), and the
  test-only `adapter.dial` hook (`codex.go:59`).
- Conversation end path (`internal/conversation/remote.go:238`, kind `finish`)
  and the environment-cleanup `cleanup_pending` flag, confirmed again as
  unrelated to the removed Codex Close-retry lifecycle.

## Disposition of round-1 findings

1. Owner topology and claim-to-harness mapping: resolved. The plan now states
   the per-turn worker model, names the persistent owners to sample, maps
   claims 1/2/5 to `gimbal run review` plus `gimbal cancel`, claim 3 to
   production conversation detach/resume plus the existing adapter live test,
   and rules out `run-prompt` as the owner. Statements match the code.
2. Concurrency: resolved. Claim 2 now requires one two-session `Group`
   lifetime and one two-worker cold start against a verified private daemon,
   states that sequential-only observation cannot satisfy the full claim, and
   requires disclosure if the cold-start case is unavailable.
3. Failure owner: resolved. First choice is a disposable instance and daemon
   under its own `CODEX_HOME` with isolation verified by distinct socket/PID
   before any stop; second choice is the test-owned connection; otherwise
   claim 4 is left unmet rather than waived.
4. Debug descriptor: resolved (focused package test; live recording disabled).
5. Loaded-thread grace period: resolved (explicit baseline rule; unload
   asserted at scope-end archive).

No new material defect was found. The remaining points are nitpicks.

## Findings

### 1. Nitpick — The identifiable tool child in claim 1 is left to model behavior

Claim 1 requires "an authorized harmless tool action whose child process can be
identified", and the plan supplies "a harmless fixture file for an observable
read tool action" under the review workflow's fixed prompt
(`review.go:54`: "Read the code… Make no changes."). A file read may be served
inside the daemon with no child process, and a review goal does not oblige the
model to shell out. Say in the plan that the `--goal` text asks for one specific
harmless shell command carrying a unique marker (the technique already used at
`codex/daemon_live_test.go:96`), and that a turn producing no such child is
"not observed" for that sub-claim and is re-run, not counted as a failure. This
does not modify the workflow.

### 2. Nitpick — The second-choice failure vehicle does not say how a terminal failure arises

With the daemon alive, `ws.CloseNow()` on the test-owned connection exercises
recovery, not terminal failure: an active turn redials via `recoverTurn`, and a
`Close` after socket loss redials via `a.connection` and archives (the existing
`TestCloseArchivesThroughARedialedConnection` proves exactly that). The plan
already says recovered success does not satisfy claim 4, but names no mechanism
for the failure. The only means that touches neither the daemon nor other
clients is the test-only `adapter.dial` hook (`codex.go:59`) returning an error
for the redial of that one adapter after the kill, which is a local
connection-refused on a real session, not a provider double. State that, or
state that the second choice can only establish the reconnect/once-only
observation and claim 4 is then unmet.

### 3. Nitpick — The private `CODEX_HOME` tests need an explicit opt-in and an in-test isolation guard

The committed cold-start and terminal-failure tests will stop a daemon. The
plan's verification principle must live inside those tests: skip unless an
explicit opt-in variable names a test-owned home (alongside `GIMBAL_LIVE`), and
fail closed when the resolved socket from `codex app-server daemon version`
under that home equals the default home's socket. Also say that the tests never
copy `auth.json` or other credentials by default; if Codex offers no supported
way to authenticate a separate home, the private vehicle is unavailable and
the plan's disclosure clause applies.

### 4. Nitpick — Bound the cancel-race re-attempts

A luna review of a small fixture can complete before `gimbal cancel` lands; the
plan correctly treats a completed result as an ordinary outcome and forbids
artificially prolonging the workflow, but "do not repeat unchanged failures"
could be read as forbidding another attempt. Say that a won race is recorded
as a normal lifetime, that the cancel case is re-attempted with a larger
read-only fixture, and cap the attempts (for example three) before reporting
the cancellation observation as not obtained.

## Not findings, recorded for the implementer

- `releaseSession` returns "already closing" for a concurrent second `Close`
  on a not-yet-deleted session (`codex/codex.go:533–536`) while `harness.go:60`
  promises idempotency; production callers serialize, so this does not affect
  the plan. Noted in round 1; unchanged.
- The implementing/validating agent model names and the parent-chat handback
  remain coordination choices this review cannot verify.

Outcome: only nitpicks remain.
