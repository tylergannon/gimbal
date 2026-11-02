# Resource-lifetime acceptance plan review — round 1

Target: `ephemeral/plans/resource-lifetime-acceptance.md` (untracked draft) as the
task definition and acceptance plan for finishing PR #665 at
`cd4ed425c0258d3ba21870e895b1c644f6e80a7a` on `codex/codex-resource-lifetime`.
Reviewed against `AGENTS.md` (repository and `~/.codex`), `docs/definition-of-done.md`,
`ephemeral/plans/codex-resource-lifetime-proof.md`,
`ephemeral/worklog/codex-resource-lifetime.md`, review rounds 1–3, the PR body,
and the implementation in the range `4b828028..cd4ed425`. The caller restated
Tyler's accepted boundary (demonstrate repaired stabilization; investigate the
historical roughly 30-process growth only if it recurs). No caller instruction
narrowed subject matter; read-only operation and the artifact path were honored.
No runtime experiment, service lifecycle action, or build was performed.

## Evidence inspected

- Adapter: `codex/codex.go` (package contract lines 1–25; `adapter.use`
  97–121; `RunTurn` cancellation/stop confirmation 380–430; `DetachSession`
  466–471; `Close`/`releaseSession` 520–571; `session.registerThread` 611–618;
  `stopConfirmed` 620–657), `codex/rpc.go` (`connect` 84–157, `daemonStatus`/
  `runCodex` 162–190, `registerThread` 194–213, `finishRead` 237–255),
  `codex/debug.go` 11–21, `codex/rlimit_unix.go`.
- Executor: `internal/agentexecutor/agent.go` (`native` 90–105, `Submit`
  164–184 launching one `--gimbal-agent-worker` per turn, worker release
  366–378, `Wait` 458–507, outer `RunTurn` cancellation 564–585);
  `internal/executor/files.go` `Launch` 194–246 (`go cmd.Wait()` reap, 2-minute
  startup pipe) and `platform_unix.go` 26–29 (`Setsid`).
- Daemon: `internal/taskdaemon/daemon.go` `closeSession` diff (session index
  released even when `Close` fails).
- Live harness: `codex/daemon_live_test.go` — `TestCloseArchivesThreadsWithoutTouchingTheDaemon`
  (223–327; `thread/loaded/list` probe, managed daemon PID unchanged),
  `TestDetachedConversationResumesTheSameNativeThread` (406–470),
  `managedDaemonPID` (607–619, `pgrep -fl "app-server --listen unix"`).
  Focused doubles: `codex/lifetime_test.go`, `internal/agentexecutor/lifetime_test.go`.
- CLI paths: `cmd/gimbal/cancel.go` (`gimbal cancel RUN --work-dir`),
  `cmd/gimbal/run_prompt.go` (in-process `web.NewInstance`, exits after the turn),
  `cmd/gimbal/daemon.go`, `internal/binding/binding.go` (all `codex` roles go
  through `agentexecutor.New`; the only production `codex.New()` is inside the
  worker, `agent.go:99`).
- Contract text changed by the PR: `README.md`, `docs/definition-of-done.md`,
  `harness.go`, `skills/gimbal-runs/SKILL.md`, `skills/gimbal-runs/references/task-environments.md`,
  `skills/gimbal-workflows/SKILL.md`.
- Checked and not a finding: the web page's "Retry cleanup" control
  (`web/src/lib/conversation/ChatActions.svelte:34`) is driven by
  `internal/conversation/remote.go` environment-preparation cleanup, not the
  removed Codex Close-retry lifecycle. `git worktree list` shows the separate
  `gimbal-cpu-diagnosis` and `native-control-feasibility` worktrees the plan
  commits to preserving.

The plan's statements about the repair match the code: route admission shares
the session closing lock; terminal transports retire the socket, reader and
debug recorder; the last operation with no owned sessions drops the shared
connection; `Close`/`DetachSession` release local scope on failure and return
the remote error; no `cleanup.json` or retry worker remains in Go code; the
worker joins release errors into the ordinary final result and `Wait`
preserves context identity with that text.

## Findings

### 1. Issue — The plan never names the owner topology or the concrete harness, so its "owner lifetime" and "persistent owner" claims are not yet checkable

The plan says "Use supported harnesses", "sampling the same persistent owners
at each boundary", and asks for observation of "local routes/readers/debug
descriptors", without stating what those owners are on this codebase:

- Every turn is its own process. `agentexecutor.Adapter.RunTurn` →
  `Submit` → `executor.Launch(binary, ["--gimbal-agent-worker", root], …)`
  (`internal/agentexecutor/agent.go:164–184`, `internal/executor/files.go:216–234`),
  started with `Setsid` (`platform_unix.go:28`) and reaped by the launching
  instance's `go cmd.Wait()`. The worker constructs its own `codex.New()`
  (`agent.go:99`), dials its own WebSocket client, runs one turn, then
  `Close` (archive) only when the scope ends or `DetachSession` (unsubscribe)
  between turns (`agent.go:366–378`), and exits. No long-lived production
  process holds the Codex adapter (`internal/binding/binding.go:48–57`).
  Consequently the in-process socket/reader/debug-descriptor release that the
  repair and the focused tests cover is bounded by a per-turn process
  lifetime; the integration observation that actually distinguishes a leak
  is (a) prompt worker exit and reap under the launching instance, (b) the
  daemon-side client socket/FD count and `thread/loaded/list`, and
  (c) identifiable tool descendants of the shared daemon. The plan's DoD #2
  wording lets a reader satisfy "local routes/readers" with package doubles
  while the per-turn process model goes unmentioned.
- The persistent owners to sample are therefore: the disposable Gimbal
  instance process (`gimbal --instance-dir … --port …`, hosting the task
  daemon and workflow daemon per `cmd/gimbal/daemon.go`), the shared `codex
  app-server --listen unix` daemon (`managedDaemonPID`), and transiently each
  worker. `gimbal run-prompt` is unsuitable as the owner because its instance
  exits with the command (`cmd/gimbal/run_prompt.go:95–100`), which the plan
  itself says is not evidence of release.
- The plan does not say which command and workflow produce the normal,
  cancelled (`gimbal cancel RUN --work-dir`, `cmd/gimbal/cancel.go`) and
  failed cases, nor which existing live checks already bear on claims when
  run on the candidate: `TestDetachedConversationResumesTheSameNativeThread`
  (DoD #3, adapter level) and `TestCloseArchivesThreadsWithoutTouchingTheDaemon`
  (loaded-thread and daemon-PID observation technique).

Impact: as written, approval would authorize an experiment whose sampled
owners and claim-to-check mapping are left to the implementer, and a run could
report "observed" internals that were only exercised by doubles. This is
definitional, not "test-case mechanics". Name the topology and the
claim-to-harness mapping in the plan before approval.

### 2. Issue — The sequential-only design cannot support the headline stabilization claim for the way Gimbal actually runs, and skips the concurrent mechanisms most likely behind the historical growth

The plan prescribes "three sequential repetitions of each normal/cancel/error
case" and "Keep concurrency limited to the sharing behavior being
demonstrated", while claiming to "demonstrate that repaired Gimbal does not
accumulate resources". Two concurrency paths in the candidate are never
exercised:

- Cold-daemon start race. Each worker independently probes the daemon and,
  if it is not running, runs `codex app-server daemon start`
  (`codex/rpc.go:90–120`). A `Group` of N first turns after a daemon death
  issues N concurrent starts; whether the losers exit or linger is vendor
  behavior that the plan's "process ancestry" inventory would catch only if
  the case is run. The earlier proof plan recorded the incident as "roughly
  30 app-server processes"; this is the one Gimbal-attributable path that
  spawns app-server processes at all.
- Concurrent sessions sharing one worker connection (`adapter.use`,
  `users`, two-session routing). The only material defect found in round 1
  was exactly this shape; the plan verifies it with the package double only.

Gimbal's real workflows are Group-shaped (bake-offs, critique rounds), so a
pass under strictly sequential load does not establish the claim Tyler
accepted. The exclusion of a 30-replica stress test does not exclude one
bounded concurrent case. Either add one real lifetime with a small `Group`
(two or three sessions in one run, same owners sampled, same two-minute
settle rule) and, if a disposable private daemon is available (finding 3),
one concurrent cold-start, or restate the headline claim as "sequential owner
lifetimes stabilize" and carry the concurrent gap explicitly into the PR body
as a material gap.

### 3. Issue — DoD #4's real failure case has no identified safe owner, so it is on track to end as a declared blocker rather than evidence

The plan requires "a bounded real failure on a disposable owned connection or
private service" and forbids damaging the shared daemon, but names no
candidate. The code's environment model offers one: workers inherit the
instance's environment unchanged (`exec.Command` without `cmd.Env`,
`internal/executor/files.go:216`), and daemon discovery is entirely
`codex app-server daemon version` in that environment (`rpc.go:165–190`). A
disposable instance started with its own `CODEX_HOME` (and the daemon it
starts under `raiseFileDescriptorLimit`) would be a private service that can
be stopped, have its socket closed mid-turn (as
`TestActiveTurnReconnectPreservesToolEffectLive` already does against the
shared daemon), or be fed a malformed frame, without touching Desktop's
daemon. Whether Codex keys the daemon socket by `CODEX_HOME` must be confirmed
in the read-only inspection the plan already schedules. The plan should name
this (or another) disposable owner as the intended failure vehicle and state
the fallback ordering explicitly, so "If no safe real failure is possible" is
reached by elimination rather than by default.

### 4. Nitpick — The "debug file descriptor" is opt-in and sensitive

`openRawRecorder` opens a file only when `GIMBAL_CODEX_DEBUG_DIR` is set
(`codex/debug.go:20–21`); captures "can contain sensitive data and must not be
committed" (`codex/codex.go:12–15`). A production integration run without the
variable has no such descriptor to observe. State that this descriptor is
proved by the focused package test on the candidate, or name a disposable,
uncommitted capture directory if the experiment enables it.

### 5. Nitpick — Loaded-thread expectations during the settle window should be stated

Between turns, workflow sessions are unsubscribed, not archived
(`agent.go:372–378`), and `DetachSession` documents an "inactivity grace
period" of unknown length before unload (`codex/codex.go:466–468`). Within a
two-minute window, a detached human thread or a live run's between-turn thread
appearing in `thread/loaded/list` is expected baseline; the unload assertion
belongs at scope-end archive, as the existing live test places it. Saying so
prevents a false "unmet claim".

## Not findings, recorded for the implementer

- `releaseSession` returns "already closing" for a concurrent second `Close`
  on a session not yet deleted (`codex/codex.go:533–536`), while `harness.go:60`
  promises idempotent `Close`. Production callers serialize (`taskdaemon`
  `closeMu`; one `Close` per worker), so this does not affect the plan.
- Model names for the implementing/validating agents ("Terra 6.1", "Sonnet")
  and the parent-chat handback are coordination choices the plan marks as
  authorized; this review cannot verify them and does not treat them as defects.

Outcome: material findings remain.
