# Adversarial review: issue 439 proposal, finished application state, round 08

Reviewer: Claude Fable 5.1, same session as rounds 01 to 07, 2026-10-09.

## Review target

The proposal tree rooted at `ephemeral/research/issue-439/proposal.md` at
commit `1eadeab0` (`docs: define backend recovery plugin deployment
ownership`, clean working tree), assessed as the finished application it
describes. Reviewed against the same authoritative sources as the earlier
rounds:

- `ephemeral/research/issue-439/requirements.md`
- `ephemeral/research/issue-439/issue.json`
- `AGENTS.md`, `docs/definition-of-done.md`, `go doc -all .`
- the current source on this branch, including the merged PR #440 worker
  (`plugins/codex-desktop/`, still at `1c3eb84c`)
- the unmerged runtime backend on `codex/temporal-container-backend` at
  `013559e85701cb424696b41fd396391a05a223ec`, which the proposal adopts
- the parallel draft the proposal now aligns its recovery packaging with,
  `temporal-docker-interface-design-2026-10-09.md` in the gimbal-view
  exploration worktree ("Status: draft design, under independent review")

Operating constraints from the caller: read-only except this artifact; write
to the caller-supplied path under `ephemeral/reviews/`. Rounds 01 to 07 are
left untouched. No caller narrowing of subject matter or predicted verdict
was present.

## Evidence inspected

Change since round 07: `git diff 284805bf..1eadeab0` touches only
`ephemeral/`: `topics/delivery-and-review.md:14` (mixed local pi and remote
Codex roles in one run), `topics/execution-placement.md:41,45,55`
(`roleBindings` rejects at `New` run-wide; validation to be narrowed per
environment; `TimeoutError` timeout type must be inspected),
`topics/lean-runners.md:9,11` (runner links "selected execution-backend
support"; new paragraph: the server "loads that installed backend plugin's
recovery client when needed"), `topics/ownership.md:49,51` (instance
deployment owns the "installed backend plugin/artifact identity"; a
replacement server "loads the configured installed backend plugin's explicit
typed recovery entry"; "Retain the matching backend artifact and
configuration while runs remain active or cleanup-pending"; "makes no
mixed-version compatibility promise"), `topics/storage-and-remote.md:11`
(server-instance row retains the backend plugin/artifact identity), and the
worklog (round-07 correction; "decision: Coordinated with the parallel
backend team on recovery packaging"). No Go, Node, `justfile`, or `plugins/`
change: `git diff 284805bf --stat -- . ':!ephemeral'` is empty.

Facts re-verified for the rewritten sentences, unchanged from round 07:
`internal/execution/temporal.go:270-279` (`roleBindings` called by `New`
over the whole run's bindings), `:540-551` (`activityUnconfirmed` treats
every `TimeoutError` as unconfirmed), `:599-610` (`ScheduleToStartTimeout:
30s`), `web/runtime.go:20,93-107,150-165` and `cmd/gimbal/main.go:19,245`
(the pinned branch statically links `internal/execution` into the web
server and the binary). On this branch `go.mod:31-32` already requires
`go.temporal.io/sdk` and `go.temporal.io/api` through
`internal/experiments/instrumented`.

The parallel draft, read in full for the packaging it proposes: lines
138-152 declare `BackendPlugin{Name, ConfigSchema, Configure}` and
`WorkflowPlugin{Name, InputSchema, Decode}` as shared exported types; line
213: "At startup, `plugin.Open` each configured `.so` path and
`Lookup("GimbalBackend")` ... the CLI/server need not link those clients.
Load once at startup"; lines 283-309 describe a backend-specific
`RunControl` keyed by "the durable server-assigned `(RunID, LaunchID)`" whose
implementation "opens the same Postgres/Temporal/Docker configuration in a
fresh control process or the existing outer backend endpoint". The issue
text (`issue.json`) itself records the Go plugin constraints: host and
plugin toolchains and common dependency sources must match, and plugins
cannot be unloaded. `requirements.md:13`: "the server must durably know
every workflow run it started, including through restart, so it can kill one
run or all its runs". `requirements.md:9`: initial authoring must not be
gated on seamless upgrades. No build file in this repository sets
`CGO_ENABLED` or `-buildmode`.

Proposal pages re-read in full: `ownership.md`, `lean-runners.md`,
`execution-placement.md`, `delivery-and-review.md:8-16`,
`backend-contract.md:11-16`, `storage-and-remote.md`.

## Disposition of round 07 findings

1. **Run-wide Codex rejection at `New`: resolved.**
   `execution-placement.md:41` now states the accurate mechanism;
   `execution-placement.md:45` requires narrowing validation to the roles
   declared for the selected environment; `delivery-and-review.md:14`
   demonstrates a local pi role beside a remote Codex role.
2. **Which process holds backend configuration and hosts recovery: resolved
   as a design, with a new defect.** `ownership.md:49` makes backend
   configuration and secret references instance deployment configuration
   passed to the runner; the runner publishes its incarnation before the
   first `Resolve`. The chosen mechanism for the server side is a loaded
   backend plugin rather than linked Gimbal code. See finding 1.
3. **Schedule-to-start timeout fix point: resolved.**
   `execution-placement.md:55`.

## Findings

### 1. Issue (incomplete requirement, over-engineering): recovery of a dead runner's environments now depends on the replacement server loading a Go plugin that must match the server's toolchain and dependencies, so the restart case the user clarified is served only when the replacement binary is build-identical

**What the proposal now requires.** `ownership.md:51`: "A replacement
server loads the configured installed backend plugin's explicit typed
recovery entry to construct a backend-specific controller". `lean-runners.md:11`:
the server "loads that installed backend plugin's recovery client when
needed; it does not statically link every provider". The word "plugin" is
never defined in the proposal, but the page it aligns with is explicit: Go
`plugin.Open` of a configured `.so` with a `Lookup` of a shared exported
type (draft line 213), and the proposal's own `lean-runners.md:13`
describes that mechanism's toolchain and shared-dependency matching
constraints.

**Why that breaks the requirement it is meant to serve.** The recovery
section exists for the restart case in `requirements.md:13`. A server
restart in practice is most often a redeploy with a new binary. A Go plugin
built against the previous Gimbal module, Go toolchain, or any shared
dependency version fails to open in the new server ("plugin was built with a
different version of package"). The proposal concedes this: "makes no
mixed-version compatibility promise" and "inability to load them is a
visible control limitation" (`ownership.md:51`). That turns the kill-one and
kill-all guarantee for every in-flight remote run into a guarantee that
holds only when the replacement server is byte-compatible with the retained
artifact, and it makes "retain the matching backend artifact ... while runs
remain active" a per-run artifact-retention store the proposal does not
describe (who stores the `.so`, keyed by what, released when). The
demonstration at `delivery-and-review.md:13` restarts the same server
build, so the failure mode would not be observed.

**Why the mechanism is also more than is needed.** There is exactly one
backend, and it is Gimbal code under `internal/execution`, not project
code. The pinned branch already links it statically into the web server
and the binary (`web/runtime.go:20`, `cmd/gimbal/main.go:19`), and this
branch's `go.mod` already carries the Temporal SDK. The import-graph saving
the plugin buys is therefore small, and the proposal's own argument at
`lean-runners.md:13` (a mode flag "does not exclude linked packages or
their initialization") applies equally to a plugin once loaded, since Go
plugins cannot be unloaded. The draft it aligns with offers a second
mechanism that has none of the coupling: "a fresh control process" opening
the same configuration (draft line 309), that is, the backend ships a small
control executable the server invokes over a process boundary with JSON.
Under `AGENTS.md` ("as simple as possible", "add a name only when a
workflow that exists needs it", new exported names only when Tyler asks),
`BackendPlugin`/`WorkflowPlugin` shared types and a server-side `.so`
loader are new infrastructure that no existing workflow needs, and the
proposal introduces them one round after rejecting "the website as a Go
plugin" on the same page.

**Smallest response.** Pick one of the two coupling-free shapes and say so
in `ownership.md:51` and `lean-runners.md:11`: either the server links the
Gimbal-internal backend recovery controller (project code stays out; the
Temporal/pgx/Docker clients are Gimbal dependencies the binary already has
or will have), or the backend provides a control executable the server runs
by path with the recorded identity, which survives server redeploys and
needs no artifact retention. Drop "retain the matching backend artifact"
unless the plugin route is kept, in which case name the retention store and
add "restart into a server built from a different commit and recover the
environment" to `delivery-and-review.md:13`, because that is the case the
mechanism fails.

### 2. Nitpick: the runner-published backend incarnation duplicates an identity the server already assigns

`ownership.md:49` requires the runner to "durably publish its
run/launch-to-backend-incarnation identity in `runner.json` and the
ownership record before the first `Resolve`", which adds a cross-process
ordering rule (runner writes, server observes and records, then the runner
may dispatch) and a second identity beside the server-assigned launch
identity that `ownership.md:17` already persists before spawn. The draft the
proposal aligns with keys recovery resources by "the durable
server-assigned `(RunID, LaunchID)`" (draft line 285), under which the
backend records its container, queue, and volume under identifiers the
server already holds and no incarnation needs to travel back before
dispatch. Say that the backend's resource records are keyed by the launch
identity the server passes at launch, and remove the runner-published
incarnation unless a backend genuinely cannot accept a caller-supplied key.
Also clarify "the ownership record": `owner.json` is server-written
(`ownership.md:15`), so the runner cannot publish into it directly.

### 3. Nitpick: `lean-runners.md:11` states what the server does not do without stating what the runner does for the local backend

"The runner links or loads the selected backend execution client" covers
Temporal/Docker. For the local backend, the execution client is the
runner's own process-group and admission-lock code (`ownership.md:39-45`),
which is linked, never loaded, and whose recovery controller is the server's
registry walk over `runner.json` and unit records (`ownership.md:31`). One
sentence saying the local backend has no plugin and no separate artifact
would keep a reader from building a loader for it.

## What the revision gets right

- The round-07 correction is adopted precisely: `roleBindings` at `New`,
  run-wide, with the narrowing stated as an integration change and a
  demonstration that observes it.
- The schedule-to-start case names the timeout type as the fix point and
  labels it an integration correction, not current behavior.
- Backend configuration and secret references are placed with instance
  deployment, passed to the runner, and recorded in outer ownership, which
  closes the two-readings problem from round 07.
- The worklog records the coordination with the parallel backend task and
  does not duplicate its exported signatures.

## Outcome

material findings remain
