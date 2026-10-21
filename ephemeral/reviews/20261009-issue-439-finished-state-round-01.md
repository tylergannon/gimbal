# Adversarial review: issue 439 proposal, finished application state, round 01

Reviewer: Claude Fable 5.1, fresh session, 2026-10-09.

## Review target

The proposal tree rooted at `ephemeral/research/issue-439/proposal.md` at
commit `0dc3a689` (clean working tree), assessed as the finished application
it describes: a central server that durably owns runs, small project runners
reached over HTTP on Unix sockets, compiled distributed backends, and a macOS
Codex desktop worker for native computer use. Reviewed against:

- `ephemeral/research/issue-439/requirements.md` (authoritative request and
  every later clarification)
- `ephemeral/research/issue-439/issue.json` (issue 439 body)
- `AGENTS.md`, `docs/definition-of-done.md`, `go doc -all .`
- the current source on this branch, which includes the merged PR #440
  worker at `1c3eb84c`

Operating constraints from the caller: read-only except this artifact; write
to the caller-supplied path under `ephemeral/reviews/`. No caller narrowing of
subject matter or predicted verdict was present.

## Evidence inspected

Proposal: `proposal.md` and all eight topic pages (`ownership.md`,
`interchange.md`, `lean-runners.md`, `workflow-discovery.md`,
`execution-placement.md`, `backend-contract.md`, `storage-and-remote.md`,
`delivery-and-review.md`); `computer-use/findings.md`,
`computer-use/session-control/{assessment,live-spike,implemented-worker}.md`;
`application-review-request.md` and `application-review.md`; prior round 09
under `ephemeral/reviews/`.

Implementation: `plugins/codex-desktop/{README.md,lib.mjs,server.mjs,
client.mjs,lib.test.mjs,.mcp.json,scripts/launch,skills/desktop-worker/SKILL.md}`,
`ephemeral/worklog/codex-desktop-worker.md`; `internal/host/host.go:74-135`
(`owner.lock` via `flock`), `internal/host/compiled.go:54-73`,
`run.go:503-545` (`CancelHostedRun`), `run.go:41-69` (graph registration);
`agy/process_unix.go:12`, `opencode/process_unix.go:44`,
`service_process_unix.go:12`, `internal/pi/shell/process_unix.go:19`;
`examples/temporal/delivery_temporal_gen.go:24-33` (per-run `TaskQueue`);
`justfile` (`test` recipe); public Godoc for `HarnessAdapter`, `ModelBinding`,
`OpenRun`, `Run`, `LifecycleRecord`.

Linked external evidence, all present and read: the two fixture records under
`~/Documents/Codex/2026-10-09/gimbal-session-control-research/build/`, the
three independent-evidence JSON files under `codex/approval-review/`, and
`codex/approval-contract-review.md`. The rollout excerpts confirm two distinct
new threads (`01a12342-1904…`, `01a12343-d9cf…`) on `gpt-5.6-luna`, native
`cua.getApp("com.google.Chrome")` with `codex/toolSurface.kind = computerUse`,
native setValue/typeText/click, fixture-recorded values at 00:42:02Z and
00:43:52Z, and empty elicitation/approval searches over both intervals. The
cancellation evidence confirms `turn_aborted reason=interrupted`, PID 12805
absent two seconds after cancel, heartbeat frozen at 16 lines. The proposal's
factual claims about the implemented slice match that evidence.

Round 09's three nitpicks are resolved (`ownership.md:25` close-on-exec;
`storage-and-remote.md:12` lists `process.lock`; `interchange.md:29` defines
publish-after-sync). The application review's three issues are addressed as
design decisions (`ownership.md:51` foreign-owned descriptors;
`interchange.md:27` one authoritative `events.jsonl`; `ownership.md:9` stable
per-user instance directory), with one exception described in finding 2.

## Findings

### 1. Issue (incomplete requirement): task-level execution requirements have no representation anywhere a workflow author can write them

**Requirement.** `requirements.md:21`: "The design must represent task-level
execution requirements and remote session ownership, even when scheduling
and integration are implemented by consumer-owned backends … This is a
required target capability, not only whole-workflow remote placement."

**Where the proposal stands.** `execution-placement.md:31`: "A backend-owned
profile or queue can express macOS plus Codex … No new public placement API
or generic scheduler is specified here. This is future backend metadata work:
current `ModelBinding`, lifecycle events, and validated `NativeRef` fields do
not already encode arbitrary worker placement." `execution-placement.md:51`
then promises "Compiler checks can establish that placement and ownership
metadata survive lowering."

**Why this is a gap, not a deferral.** Remote session *ownership* is
represented (worker, request ID, native thread recorded in observations and
the ledger). Task *requirements* are not: nothing in the proposal says how an
authored workflow states that one `Generate` call needs macOS, Codex, and
saved approval for `com.google.Chrome`. The only compiled backend in the tree
places by a per-run queue (`examples/temporal/delivery_temporal_gen.go:33`
`TaskQueue: environment.Queue`), and `ModelBinding` is `{Adapter, Model,
Effort}` per role (`go doc`). A compiler check that "placement metadata
survives lowering" has nothing to check until the metadata's source is
named. The TL;DR's central claim, "A workflow can mix Linux tasks and macOS
desktop tasks" (`proposal.md:9`), therefore describes a backend capability
with no authoring surface.

**Smallest response (a decision, no new exported name).** Say that the role
is the placement unit: a `WorkflowRole` bound to the desktop adapter runs
where that adapter runs, and the adapter's requirements (`requiredApps`,
target desktop) are ordinary constructor arguments in the project's `main`,
exactly as a model name is today. The compiled lowering then maps role to
queue/worker, and the compiler check becomes "every role with a desktop
adapter lowers to a desktop queue." If Tyler wants per-call rather than
per-role placement, that needs a name he has not asked for; the proposal
should say so and pick the role.

### 2. Issue (incorrect implementation of the kill requirement): the local backend's force-stop has no containment mechanism, and the one the proposal gestures at contradicts the tree

**Requirement.** `requirements.md:13`: the server must be able to "kill one
run or all its runs … Independent processes must not become unmanaged."

**Where the proposal stands.** `ownership.md:47`: "Force-stop addresses the
backend's entire recorded execution … The backend must provide a way to
terminate owned execution units when their workflow process is
unresponsive." `backend-contract.md:15`: "process groups with enforced
no-escape behavior can provide containment." `backend-contract.md:35`
requires a test that inspects "real descendants." `delivery-and-review.md:37`
records the application review's force-stop finding as "resolve[d] … as
explicit design decisions."

**Why it is not resolved for the local backend.** Every agent and service
child in the tree is started in its *own* process group
(`agy/process_unix.go:12`, `opencode/process_unix.go:44`,
`service_process_unix.go:12`, `internal/pi/shell/process_unix.go:19`, all
`Setpgid: true`). A local runner's group contains the runner alone, so
"process groups with enforced no-escape behavior" is not available without
changing those four sites, and the proposal does not say to change them.
The local runner is the first backend to be built (`delivery-and-review.md:5`),
yet it is the one backend the proposal leaves with an obligation and no
mechanism. Failure: a runner wedged inside an adapter call holds
`process.lock`, answers no HTTP, and keeps its stop intent pending forever;
force-stop signals the runner's PID, the `claude`/`agy`/service children
keep running, and the server has no record of them to name as unresolved.
That is the unmanaged state the requirement forbids, and cancel-all's
"cessation confirmed" cannot be truthful.

**Smallest response.** Pick one for the local backend and write it down:
(a) have adapters record each child's PID and start token in the journal
(the `CommandStarted` record already exists for services; opencode already
verifies identity with `ps lstart`) so force-stop can enumerate, verify, and
signal them and name any it cannot; or (b) start the runner with `Setsid`
and drop `Setpgid` from the four spawn sites so the runner's session is the
containment boundary, accepting that a service's own group semantics change.
Either is a paragraph in `ownership.md` and one test. Without one of them the
local backend fails its own validation list.

### 3. Issue (critical antipattern in the finished state): losing the desktop owner task leaves native sessions permanently outside Gimbal's control

**Requirement.** `requirements.md:13` (no unmanaged processes);
`proposal.md:27` risk row: "Reserve that desktop for one controlling task;
reconnect by session identity. No second controller or blind replay."

**Where the proposal stands.** `execution-placement.md:45`: "Losing the
shared owner means control is unavailable, not that all its created tasks
stopped. Keep uncertain runs visible and cancellable." `implemented-worker.md:28`:
"retain control responsibility through worker unavailability."

**Why "visible and cancellable" is not achievable as designed.** The worker
binds its ledger to one owner thread and refuses any other
(`plugins/codex-desktop/lib.mjs:90`: "Worker belongs to desktop task …;
resume that task to recover its sessions"); the README's only recovery path
is "Rebind from the same owner after recovery" (`README.md:51-52`). If that
owner task is archived, deleted, or lost in a desktop restart while a run's
native task is mid-turn, the native task continues operating the user's
Chrome under Codex, the server marks the run unreachable, cancel has nothing
to address, and no documented operation lets a new owner adopt the ledger.
The requirement's failure mode (an independent process nobody can stop
through Gimbal) is exactly this case, and it is the one most likely to occur
in practice because the owner is a UI task a human can close. The backend
contract's "Cancel/force-stop reaches work on two execution sites"
(`backend-contract.md:35`) has no implementation route here.

**Smallest response.** Specify an explicit adoption step in
`execution-placement.md`: a new owner task may bind with an existing ledger
when it first verifies, via `read_thread`, that each recorded thread still
exists, records the owner change in the ledger and the server's run
observations, and inherits the desktop reservation. That is the "reconnect by
session identity" the TL;DR already promises, applied to the owner rather
than only to the created task. Note the ledger check that forbids it today so
the implementer knows what to change.

### 4. Nitpick: the unattended proof is Chrome-only, and the desktop flags Chrome as a distinct surface

`proposal.md:13-15` correctly bounds the result to "the real persistently
approved Chrome bundle." The evidence adds a detail worth stating: every
native call in both sessions carries `_meta.codex/computerUseChrome: true`
alongside `toolSurface.kind: computerUse`, and the only non-Chrome native
attempt in this research (the Cocoa fixture
`dev.gimbal.disposable-native-probe`, `approval-contract-review.md` "Original
gate") waited 263 s on an elicitation. No generic native app has yet passed
unattended with a saved approval. The risk row "Fresh permitted tasks succeed
unattended" (`proposal.md:26`) should say the claim is established for one
app the desktop special-cases, so the next acceptance scenario is run against
a non-Chrome app before any workflow depends on one.

### 5. Nitpick: the worker's checks are outside the repository's test gate

`backend-contract.md:31` asks for automated checks "beside the backend they
check," and `execution-placement.md:27` asks to "recheck this small scenario
after desktop changes." The twenty worker tests run with `npm test` in
`plugins/codex-desktop/`, but the `justfile` `test` recipe runs only `go
test`, the Temporal example, and `web` tests; nothing in the repository runs
the worker's tests. Add the directory to `just test` (or say in the proposal
why the Node package is gated separately) so the implemented slice the
proposal now leans on is at least protocol-checked on every change.

## What the design gets right

- Durable ownership before spawn, `process.lock` inheritance with
  close-on-exec, and foreign-owned descriptor handling close the launch and
  restart windows correctly on Linux and macOS.
- Server-dials-runner over HTTP on Unix sockets, with the journal as the
  backpressure queue and positions assigned at durable append, fits the
  stated concurrency and is how this tree already works.
- One authoritative `events.jsonl` with derived views follows "delete what
  is replaced."
- The plugin question is answered with the right reason: a process boundary
  couples on the wire, a plugin couples on toolchain and every dependency.
- The implemented worker's contracts (persist-before-create, no blind replay,
  ownership-scoped read/message/cancel, acceptance distinct from cessation)
  match the evidence and the repository's execution-and-observation rules.

## Outcome

material findings remain
