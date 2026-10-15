# Adversarial review: issue 439 proposal, finished application state, round 07

Reviewer: Claude Fable 5.1, same session as rounds 01 to 06, 2026-10-09.

## Review target

The proposal tree rooted at `ephemeral/research/issue-439/proposal.md` at
commit `284805bf` (`docs: specify recovery of runtime backend
environments`, clean working tree), assessed as the finished application it
describes. Reviewed against the same authoritative sources as the earlier
rounds:

- `ephemeral/research/issue-439/requirements.md`
- `ephemeral/research/issue-439/issue.json`
- `AGENTS.md`, `docs/definition-of-done.md`, `go doc -all .`
- the current source on this branch, including the merged PR #440 worker
  (`plugins/codex-desktop/`, still at `1c3eb84c`)
- the unmerged runtime backend on `codex/temporal-container-backend` at
  `013559e85701cb424696b41fd396391a05a223ec`, which the proposal adopts

Operating constraints from the caller: read-only except this artifact; write
to the caller-supplied path under `ephemeral/reviews/`. Rounds 01 to 06 are
left untouched. No caller narrowing of subject matter or predicted verdict
was present.

## Evidence inspected

Change since round 06: `git diff ff26f254..284805bf` touches only
`ephemeral/`: `topics/ownership.md` (new section "Recover remote
environments without the runner"), `topics/backend-contract.md` (recovery
through a control path independent of the runner), `topics/
delivery-and-review.md` (two new demonstration bullets: kill the runner
during a remote command and an authenticated remote turn; reject unsupported
declared bindings before effects and show sibling termination on
environment removal), `topics/execution-placement.md` (permalinked sources;
pinned worklog evidence stated with the credentials caveat; "reject before
effects" sentence; new "dedicated environment is also a cleanup boundary"
paragraph), and the worklog (two entries). No Go, Node, `justfile`, or
`plugins/` change: `git diff ff26f254 --stat -- . ':!ephemeral'` is empty.

Pinned commit availability: `013559e8` is on `origin/codex/temporal-container-backend`
and resolves through the GitHub API, so the new permalinks are valid.

Pinned backend, read at that commit: `internal/execution/temporal.go:68-100`
(`Config`: Docker image, worker binary, Temporal address, Postgres DSN,
mounts, secret files, Docker executable), `:154-245` (`New`; per-instance
`owner: uuid.NewString()`), `:270-279` (`roleBindings` rejects any bound
role that is not Codex), `:281-333` (`Resolve`), `:438-475` (`Close`,
`removeWorker`), `:477-551` (`Environment.remove`, `activityUnconfirmed`),
`:553-558` (`Harness`), `:561-600` (`Start` calls `awaitStarted` and
`Stop`s on failure), `:680-712` (`awaitStarted`), `:871-913` (`Stop`);
`web/runtime.go:93-107,150-165` (`WithExecutionBackend` holds the backend
`Config` in the server process; `SetExecutionBackendFactory` calls
`execution.New` per hosted run inside the web process);
`cmd/gimbal/main.go:245`; `internal/host/host.go:274`
(`gimbal.WithExecution(backend)`); `execution.go` (constructor comment:
"Provider configuration belongs to the implementation's constructor").
Proposal pages re-read in full: `ownership.md`, `execution-placement.md`,
`delivery-and-review.md:8-16`, `backend-contract.md:8-16`,
`lean-runners.md`, `storage-and-remote.md:17`.

## Disposition of round 06 findings

1. **No recovery path for a dead runner's environments: resolved as a
   design.** `ownership.md:47-51` records the backend reference and
   incarnation in outer ownership before environments are created, treats
   each resolved environment as a registered unit with native identities
   persisted before dispatch, and requires a backend-specific recovery
   controller that fences, verifies, and stops without calling `Resolve`
   or constructing a fresh incarnation. `delivery-and-review.md:13`
   demonstrates it. See finding 2 for what the section still leaves open.
2. **Overstated evidence: resolved, with one inaccuracy carried in.**
   `execution-placement.md:41` now states the no-model command evidence,
   the credentials blocker, and Codex-only harness binding. The sentence
   "unsupported bindings currently failing at first `Generate`" repeats a
   claim from my round-06 review that the pinned code does not support; see
   finding 1.
3. **Whole-environment removal blast radius: resolved.**
   `execution-placement.md:55`, `delivery-and-review.md:14`.
4. **Worktree-path citations: resolved.** `execution-placement.md:41`.

## Findings

### 1. Issue (incorrect evidence, conflicting requirement): the pinned backend rejects every non-Codex role at construction, not at first `Generate`, which also forbids the mixed role sets the proposal promises

**Correction to round 06.** I wrote that role mismatch surfaces at the
first turn because `Harness` returns nil for non-Codex bindings. That nil
path is unreachable: `New` calls `roleBindings`, which returns an error for
any bound role whose binding is not `codex` with a model and adapter
(`temporal.go:270-279`), so a backend is never constructed with such a
role. The proposal adopted my statement (`execution-placement.md:41`:
"unsupported bindings currently failing at first `Generate`") and it is
wrong in the same way.

**Why it matters beyond accuracy.** The pinned backend receives the whole
run's model bindings (`web/runtime.go:150-152`: the factory is handed
`models` and passes them to `New`) and rejects the run if *any* role is
bound to pi, agy, or Claude. The proposal's TL;DR and placement page
promise the opposite shape: "roles choose the agent within" an environment
(`proposal.md:9`), "a Linux context for build/check commands and a macOS
context for a native session" (`execution-placement.md:39`), and "This does
not require enumerating every unused catalog role" (`execution-placement.md:45`).
A run that binds one Codex role for a remote environment and one pi role
for local sessions is rejected by the adopted seam today. Neither
`execution-placement.md:41` nor the delivery bullet
(`delivery-and-review.md:14`) names this as a change the integration must
make.

**Smallest response.** Replace the `Generate` sentence with the accurate
statement: the pinned `roleBindings` rejects at `New` and rejects
run-wide, so validation must be narrowed to the roles declared for a given
environment while roles bound to local adapters remain unconstrained. Add
"bind one local pi role and one remote Codex role in the same run" to the
mixed-platform demonstration so the narrowing is observed.

### 2. Issue (incomplete requirement): the recovery section does not say which process holds the backend configuration or hosts the recovery controller, and the two readings the proposal supports lead to different systems

**What the section requires.** `ownership.md:51`: "A replacement server
must be able to open a backend-specific recovery controller from that
reference and identity independently of the dead runner."

**What the proposal says elsewhere.** `lean-runners.md:9`: the project
runner links "Gimbal execution runtime, required adapters"; the server
excludes project code. `execution-placement.md:45`: adapter wiring is
"in project Go". `execution.go` (pinned): "Provider configuration belongs
to the implementation's constructor." In the pinned branch the constructor
runs in the server's web process with server-held configuration
(`web/runtime.go:96,159`, `cmd/gimbal/main.go:245`), which is the
arrangement the proposal is replacing.

**The two readings.** If the project runner's `main` constructs the backend
(as the constructor comment and "wiring in project Go" suggest), the server
has no Postgres DSN, Temporal address, Docker executable, or secret-file
map, and cannot open a recovery controller without executing project code;
the recovery section is then unimplementable as written. If instead the
backend configuration is server-side deployment configuration handed to
the runner at launch (the pattern `storage-and-remote.md:17` uses for store
paths), the server links the Temporal, pgx, and Docker code paths and the
runner must report the incarnation ID through its endpoint or `runner.json`
before its first `Resolve`, because `Resolve` is a workflow-time call. The
proposal does not choose, and "backend kind/configuration reference"
(`ownership.md:49`) is satisfiable under both.

**Smallest response.** One sentence each: backend configuration, including
secrets references, is instance deployment configuration owned by the
server and passed to the hosted runner at launch; the runner publishes the
backend incarnation in `runner.json` before creating any environment; the
server links the backend's recovery code, which is Gimbal code, not project
code. Then `lean-runners.md` should list the backend client among what the
runner links and what the server links.

### 3. Nitpick: the schedule-to-start case is reproducible in the pinned code, and the fix point can be named

`execution-placement.md:55` says a queued dispatch timeout "must not cause
destructive environment cleanup" and that the pinned implementation may not
distinguish it. It does not. With no worker polling the queue, `Start`
schedules the activity with a 30 s `ScheduleToStartTimeout`
(`temporal.go:606`), `awaitStarted` sees status `TIMED_OUT`, `Get` returns a
`TimeoutError`, `Start` calls `p.Stop()` (`temporal.go:597`), `Stop` waits,
`activityUnconfirmed` returns true for every `TimeoutError`
(`temporal.go:544-546`), and `Environment.remove` deletes the container
that never ran the command. The distinguishing field is the timeout type on
`TimeoutError`; naming it in the correction keeps the integration from
re-deriving the trace.

## What the revision gets right

- Recovery is now a stated requirement with a durable identity chain
  (reference, incarnation, environment identities) and an explicit ban on
  recreating an uncertain environment through `Resolve`.
- Evidence is stated at its actual strength: no-model commands observed,
  authenticated turns blocked on credentials, nothing rerun.
- Environment removal is named as a cleanup boundary with sibling impact
  recorded and externally shared providers excluded.
- Sources are pinned by branch and permalink, and the permalinked commit is
  published.

## Outcome

material findings remain
