# Adversarial review: issue 439 proposal, finished application state, round 02

Reviewer: Claude Fable 5.1, same session as round 01, 2026-10-09.

## Review target

The proposal tree rooted at `ephemeral/research/issue-439/proposal.md` at
commit `8afd79bc` (`docs: specify role placement and recoverable execution
ownership`, clean working tree), assessed as the finished application it
describes. Reviewed against the same authoritative sources as round 01:

- `ephemeral/research/issue-439/requirements.md`
- `ephemeral/research/issue-439/issue.json`
- `AGENTS.md`, `docs/definition-of-done.md`, `go doc -all .`
- the current source on this branch, including the merged PR #440 worker
  (`plugins/codex-desktop/`, last changed at `1c3eb84c`)

Operating constraints from the caller: read-only except this artifact; write
to the caller-supplied path under `ephemeral/reviews/`. Round 01 is left
untouched. No caller narrowing of subject matter or predicted verdict was
present.

## Evidence inspected

Change since round 01: `git diff 0dc3a689..8afd79bc` touches only
`ephemeral/`: `proposal.md` (TL;DR placement sentence, three risk rows),
`topics/ownership.md` (new section "Local force-stop has a concrete execution
boundary"), `topics/execution-placement.md` (rewritten "Task requirements,
backend placement", new "Recover the desktop owner without losing its
tasks", revised "What remains to demonstrate"), `topics/backend-contract.md`
(role lowering; containment paragraph), `topics/delivery-and-review.md`
(wedge/force-stop demonstration bullet; Chrome and `npm test` gate
paragraphs), `topics/storage-and-remote.md` (unit registrations and
admission lock in the durable-material table),
`computer-use/session-control/implemented-worker.md` (Chrome marker, fenced
handover pointer, separate test gate), and
`ephemeral/worklog/20261009-issue-439-design.md` (three new entries). No Go,
Node, `justfile`, or `plugins/` change: `git diff 0dc3a689 --stat -- .
':!ephemeral'` is empty.

Re-read in full: `proposal.md`, `ownership.md`, `execution-placement.md`,
the diffs of the other four topic pages, `implemented-worker.md`, and the
worklog. Unchanged pages (`interchange.md`, `lean-runners.md`,
`workflow-discovery.md`, `computer-use/findings.md`, `assessment.md`,
`live-spike.md`, `application-review*.md`) stand as reviewed in round 01.

Implementation re-read: `plugins/codex-desktop/{lib.mjs,server.mjs,
client.mjs,lib.test.mjs,README.md,package.json}`. Go anchors verified for
the revised claims: `go doc` for `NewSession(ctx, role WorkflowRole,
workdir)`, `ModelBinding{Adapter, Model, Effort}`, `RoleDevOpsTasks`,
`RoleQAOrchestration`, and the `WorkflowRole` catalog rule; the four
`Setpgid: true` sites (`internal/pi/shell/process_unix.go:19`,
`agy/process_unix.go:12`, `service_process_unix.go:12`,
`opencode/process_unix.go:44`); `command.go:112,273` (`RunCommand` spawns
without a group); `codex/rpc.go:178`; `opencode/process_unix.go:47-48`
(`ps lstart` birth identity); `examples/temporal/delivery_temporal_gen.go:33,71,77`
(one per-run `TaskQueue`, `OpenSession` with a constant role, one activity
per `Generate` site). External evidence is unchanged from round 01 and was
not re-opened.

## Disposition of round 01 findings

1. **Task-level placement representation: resolved as a decision.** The role
   is now the placement unit (`execution-placement.md:31-35`,
   `backend-contract.md:9`, `proposal.md:9`). The claims it rests on are true
   in the tree: `NewSession` takes a `WorkflowRole`, `ModelBinding.Adapter`
   selects the harness, and the Temporal example passes a constant role at
   its `OpenSession` site so a compiler can read it. The limits are stated
   (per-role not per-call; `RunCommand`/`Check` do not move; Temporal is
   single-queue today). See nitpick 3 for the one loose end.
2. **Local force-stop mechanism: resolved as a design, with one defect
   carried into finding 2.** `ownership.md:37-45` now specifies a gated
   bootstrap with durable unit nonce/PID/PGID/birth identity, a runner-held
   admission lock, close-on-exec gate descriptors, intent-before-kill, and
   cleanup-pending for unverifiable groups. It is consistent with the four
   `Setpgid` sites, requires migrating `RunCommand`, and the decisive check in
   `delivery-and-review.md:11` is the right shape.
3. **Owner loss: addressed in design, with one defect carried into
   finding 1.** `execution-placement.md:49-57` adds the fenced handover and
   correctly names `Worker.load`'s owner-equality rule as the thing to change.
4. **Chrome-only proof: resolved.** `proposal.md:26`,
   `execution-placement.md:27`, `implemented-worker.md:13,23`.
5. **Worker tests outside `just test`: resolved as a recorded separate gate**
   (`delivery-and-review.md:34`). The reviewer does not require a build-file
   change; the decision is explicit.

## Findings

### 1. Issue (incorrect mechanism): the fenced handover requires an exclusive OS lock that the Node worker cannot hold, and the worker's only exclusivity today is file presence

**Where the proposal stands.** `execution-placement.md:53`: a replacement
owner "must first hold an exclusive OS lock for the worker/ledger directory,
fencing the old worker from concurrent control; every worker holds this lock
throughout its active lifetime. File presence alone is not the lock."
`proposal.md:27` repeats "under an exclusive worker lock."

**Why it does not hold for the worker as built.** The worker is a Node 22
package with no dependencies (`plugins/codex-desktop/package.json`); Node's
`fs` has no `flock`/`fcntl` lock, and nothing in `lib.mjs`, `server.mjs`, or
`scripts/launch` takes one. Its sole exclusivity is the `worker.sock` bind
(`server.mjs:47-53`): `listen` fails if the path exists, and the error text
instructs the operator to "verify its owner before clearing a stale entry."
That is exactly the file-presence sentinel the proposal rejects in
`ownership.md:11` for the server. After a worker crash or desktop restart the
socket file remains, so even the *same* owner's rebind fails until a human
removes it (`README.md:51-52` "inspect any stale socket before removing
it"), and a replacement owner has nothing kernel-held to fence against. The
handover design therefore names a lock the chosen runtime cannot provide and
leaves the stale-socket recovery step, the one most likely to happen in
practice, undefined.

**Smallest response.** Say how the worker will hold a kernel lock, or change
the lock holder. Two options that add no framework: (a) the Node worker is
launched by a tiny Go bootstrap that takes `flock` on the ledger directory
and passes the held descriptor to Node, mirroring the runner's inherited
`process.lock` (`ownership.md:25`); or (b) the MCP process stays Node and
the handover is arbitrated by the desktop itself, i.e. a new owner may adopt
only after `read_thread` shows the old owner thread archived or unloaded,
accepting that this is provider evidence rather than a kernel lock and saying
so. In either case, define stale-socket recovery: the holder of the lock may
unlink and rebind; nobody else may.

### 2. Issue (unenforceable requirement in the finished state): the force-stop boundary prohibits process-group escape by binaries Gimbal does not author, and the decisive check does not exercise them

**Requirement.** `requirements.md:13`: "Independent processes must not
become unmanaged."

**Where the proposal stands.** `ownership.md:43`: a registered unit's
"workload may fork/exec ordinary grandchildren within the registered group;
unregistered new groups/sessions, detachment, and daemonization are
prohibited." `ownership.md:41` forbids process scanning to discover
ownership. The decisive check (`ownership.md:45`,
`delivery-and-review.md:11`) "force-stops recorded managed child groups
including grandchildren."

**Why this leaves unmanaged work by design.** The workloads behind the
registered groups are `claude`, `codex` (`codex/rpc.go:178`), `agy`
(`agy/agy.go:262`), and OpenCode: third-party harnesses that spawn their own
tool commands and are not bound by a Gimbal contract. Putting each tool
command in its own process group is routine harness practice; Gimbal's own
in-tree agent does exactly that for every shell command
(`internal/pi/shell/process_unix.go:19`). When a harness does this, the
registered PGID kill stops the harness and leaves its running tool command
(a build, a test suite, a server it started) alive with no record, and the
no-scan rule means the server cannot even name it as cleanup-pending. The
decisive check as written can pass against a cooperative fixture while the
real harnesses escape it, so it would not establish the claim the risk row
makes (`proposal.md:24` "Cancel … reaches owned work on every site").

**Smallest response.** Replace "prohibited" with a verified fact per
supported harness: run the decisive check against each real adapter binary
while it executes a long-running tool command, record which harnesses keep
tool commands in the registered group, and for those that do not, either
register the harness's own group-kill path (every one of them has a
cancel/interrupt that stops its tool command) as part of the unit's stop
sequence before the PGID kill, or state that class as the accepted unmanaged
residual in `ownership.md`. The design already says this is "managed
execution, not kernel containment"; it needs to say what is managed.

### 3. Nitpick: role placement has two configuration sources with no stated single owner

`execution-placement.md:31` places a role by its `ModelBinding.Adapter` in
"the project's ordinary Go `main`"; `backend-contract.md:9` and
`execution-placement.md:33` have a compiler map the same role to a
queue/worker at build time. In compiled execution the adapter binding lives
in the macOS worker's `main`, the queue mapping in the compiler's
configuration, and the Linux runner's `main` may bind the role too. Nothing
says which one is authoritative or who checks that they agree, so "reject a
desktop role without a compatible mapping" can pass at compile time while
the worker that receives the activity binds the role to a headless adapter.
One sentence naming the compile-time role-to-queue mapping as the source,
with the worker's binding checked against it at registration, closes it.

### 4. Nitpick: handover blocks while a live old worker holds the lock, and the proposal does not say so

`execution-placement.md:53` fences the old worker with the lock, but
`execution-placement.md:51-53` only covers "the old owner cannot be
resumed." If the old owner *thread* is archived while its MCP process is
still alive (the process exits only on stdin close or signal,
`server.mjs:84-96`), the replacement waits on a lock whose holder will never
hand over. Say that the operator stops the old worker process first, and
that the replacement reports `lock held` rather than hanging.

## What the revision gets right

- Role-as-placement reuses `NewSession`/`ModelBinding` exactly as they exist
  and refuses to invent OS/provider roles, matching the `WorkflowRole`
  catalog rule in Godoc.
- The gated bootstrap keeps the existing per-command groups, uses a birth
  identity the tree already computes (`opencode/process_unix.go:47`), and
  makes intent, registration, grant, and kill ordering explicit. The
  crash-at-gate test is the right check.
- Owner recovery now distinguishes same-owner rebind from replacement
  adoption and names the current `Worker.load` rule as the change site.
- Chrome-specific proof, the separate `npm test` gate, and "not current
  worker behavior" markers keep the evidence honest.

## Outcome

material findings remain
