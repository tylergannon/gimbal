# Adversarial review: issue 439 proposal, finished application state, round 03

Reviewer: Claude Fable 5.1, same session as rounds 01 and 02, 2026-10-09.

## Review target

The proposal tree rooted at `ephemeral/research/issue-439/proposal.md` at
commit `cfb66dc9` (`docs: gate native admission and verified run
containment`, clean working tree), assessed as the finished application it
describes. Reviewed against the same authoritative sources as the earlier
rounds:

- `ephemeral/research/issue-439/requirements.md`
- `ephemeral/research/issue-439/issue.json`
- `AGENTS.md`, `docs/definition-of-done.md`, `go doc -all .`
- the current source on this branch, including the merged PR #440 worker
  (`plugins/codex-desktop/`, still at `1c3eb84c`)

Operating constraints from the caller: read-only except this artifact; write
to the caller-supplied path under `ephemeral/reviews/`. Rounds 01 and 02 are
left untouched. No caller narrowing of subject matter or predicted verdict
was present.

## Evidence inspected

Change since round 02: `git diff 8afd79bc..cfb66dc9` touches only
`ephemeral/`: `proposal.md` (established-state paragraph; two risk rows),
`topics/execution-placement.md` (new section "Bind native identity before
the first work starts"; single-source placement sentence; Go lock bootstrap
and held-lock behavior in the handover section), `topics/ownership.md`
(verified-profile rule and real-adapter decisive check),
`topics/backend-contract.md` (native allocation-before-turn; verified
profiles; real-harness validation bullet), `topics/delivery-and-review.md`
(real-adapter demonstration bullet; lost-reply dependency),
`computer-use/session-control/implemented-worker.md` (lost-reply row and
obligation), and `ephemeral/worklog/20261009-issue-439-design.md` (three
entries). No Go, Node, `justfile`, or `plugins/` change:
`git diff 8afd79bc --stat -- . ':!ephemeral'` is empty.

Re-read in full: `proposal.md`, `execution-placement.md`,
`backend-contract.md`, `delivery-and-review.md`, the diffs of
`ownership.md` and `implemented-worker.md`, and the worklog. Pages unchanged
since round 01 (`interchange.md`, `lean-runners.md`,
`workflow-discovery.md`, `storage-and-remote.md`, `computer-use/findings.md`,
`assessment.md`, `live-spike.md`, `application-review*.md`) stand as
reviewed.

Implementation re-read for the new claims: `plugins/codex-desktop/lib.mjs:32-37`
(fixed 30 s `desktopCall` timeout), `lib.mjs:137-151` (save `starting`,
`create_thread`, save `threadId`; no request ID in the provider input),
`server.mjs:47-53` (socket bind as the only exclusivity),
`scripts/launch` (shell `exec` of the desktop-supplied Node);
`codex/rpc.go:26-27,83-85` (shared `codex app-server` daemon keeps every
thread), `codex/codex.go:531-534` (`turn/interrupt` RPC),
`opencode/server.go:203` (`opencode serve` shared server),
`opencode/adapter.go:212-213,243` (HTTP `Abort`),
`internal/pi/shell/process_unix.go:11-19` (Go port uses `Setpgid`; upstream
pi uses `detached: true`), `agy/process_unix.go:12-24`. The linked trace
`~/Documents/Codex/2026-10-09/gimbal-session-control-research/validation/desktop-create-thread-trace.md`
exists (10,498 bytes). Other external evidence is unchanged from round 01.

## Disposition of round 02 findings

1. **Node worker cannot hold an OS lock: resolved as a design.**
   `execution-placement.md:61-63,67` specifies a Go launch bootstrap that
   takes the nonblocking exclusive lock, exec-replaces itself with the
   desktop-supplied Node so bridge ancestry is unchanged, retains the
   descriptor, never forwards it, lets only the lock holder unlink a stale
   socket, reports a held lock as unavailable, and requires re-running the
   ordinary-MCP binding and native acceptance scenario afterwards.
2. **Force-stop prohibition on third-party binaries: addressed, with a
   consequence carried into finding 1 below.** `ownership.md:43-45` and
   `backend-contract.md:15,35` now require verifying each real harness on
   long-running tool work, reject placements whose descendants cannot be
   controlled, and accept no unmanaged residual.
3. **Two placement sources: resolved.** `execution-placement.md:41` names
   the backend deployment configuration as the single source and requires
   worker registration to check its adapter binding against it.
4. **Handover blocks on a live old worker: resolved.**
   `execution-placement.md:63`.

The revision also adds a gap the authors found themselves: the desktop
`create_thread` starts work before returning an ID, so a lost reply can
leave a running native task with no recorded identity
(`execution-placement.md:29-35`, `proposal.md:15,22`). The description
matches `lib.mjs:137-151`; marking it an unresolved provider gate rather
than an implemented fix is the correct treatment.

## Findings

### 1. Issue (internal contradiction in the force-stop rule): the daemon-hosted adapters cannot satisfy the rule as written, and the proposal does not say whether they are rejected

**Where the proposal stands.** `ownership.md:43`: registered-group placement
"supports only actual harness/workload profiles verified to keep all tool
descendants in recorded units … Escaping profiles require independently
enforceable backend-native containment … or verified separately addressable
native resources. Otherwise reject that hosted placement before dispatch.
There is no accepted unmanaged residual." Same paragraph: "Shared provider
services such as OpenCode and Codex desktop are attached resources: cancel
the run-owned native sessions, not their shared server process."
`backend-contract.md:15`: "A cooperative stop request alone is insufficient
when the harness is unresponsive."

**Why the two sentences cannot both hold for two of the four adapters.**
The `codex` adapter does not spawn a harness at all: every session and turn
lives in the machine's shared `codex app-server` daemon ("the daemon, not
this process, keeps the threads loaded", `codex/rpc.go:26-27`), and the only
stop is the `turn/interrupt` RPC (`codex/codex.go:531-534`). OpenCode is the
same shape: one shared `opencode serve` (`opencode/server.go:203`) and an
HTTP `Abort` (`opencode/adapter.go:213`). Their tool commands are children
of the shared daemon, so no run-owned registered group can ever contain
them, and the daemon itself is declared off-limits. The only control is the
cooperative interrupt, which the rule says is insufficient, and the rule
accepts no residual. Read literally, the local backend must refuse to host
Codex and OpenCode sessions. Read charitably, "separately addressable native
resources" is meant to admit daemon sessions whose interrupt is verified to
stop their tool children, which is a cooperative stop with a residual (a
wedged daemon) that the same paragraph denies exists. The in-tree evidence
that this is not hypothetical: the Go port of pi keeps tool commands in a
group only because it is in-tree; upstream pi uses `detached: true`, which
is a new session, not a group (`internal/pi/shell/process_unix.go:11-13`).
Codex and OpenCode cannot be edited that way.

**Impact.** The finished local backend's hostable adapter set is
undetermined by the text, and the first deliverable (`delivery-and-review.md:5,36`)
names no verified profile it can ship with. The only profile compliant by
construction today is the in-tree pi agent, whose tool spawns are
Gimbal-controlled; the proposal does not say so.

**Smallest response.** Resolve the contradiction in one paragraph of
`ownership.md`: define "separately addressable native resource" as a
provider session whose interrupt has been observed, in the decisive check,
to stop its tool descendants; state that for such sessions the backend's
stop sequence is interrupt, observe cessation through the provider, and
otherwise hold the unit `cleanup-pending` attributed to the named daemon;
and say plainly that a wedged shared daemon is that named residual, not an
unmanaged run. Then list the profiles the first slice ships with (pi by
construction; codex and OpenCode after the daemon-interrupt check; agy and
claude after the real-harness group check). If instead Tyler wants no
residual at all, the proposal must say the daemon adapters are not hostable
until the daemon runs per run, which is a product decision the TL;DR should
carry.

### 2. Nitpick: the lost-reply window is partly manufactured by the worker's fixed creation timeout

`lib.mjs:32,37` applies the same 30 s `desktopCall` timeout to
`create_thread` as to a read. A desktop that takes longer than 30 s to
create and start the task (first model load, slow machine) turns a creation
that *succeeded* into `outcome_unknown` with the task running, which is the
exact case `execution-placement.md:29-35` describes. The provider gate is
still the right fix, but the proposal should note that the current worker
can reproduce the gap by timing alone and that a creation-specific timeout
(or none, since the caller already bounds the socket exchange,
`client.mjs:62-64`) narrows it without changing semantics.

### 3. Nitpick: "the current launch script uses socket-path existence for exclusivity" is the wrong file

`execution-placement.md:67`. `scripts/launch` only validates
`CODEX_MCP_NODE_PATH` and `exec`s Node; the socket-bind exclusivity and the
stale-socket message are in `server.mjs:47-53`. It matters because the lock
bootstrap replaces the launch script, while the unlink-on-close and
bind-failure logic that must defer to the lock holder lives in `server.mjs`.

### 4. Nitpick: two bootstraps are described without saying whether they are one program

`ownership.md:39` makes the local unit bootstrap "a mode of the runner
executable"; `execution-placement.md:61` describes "a small Go launch
bootstrap" for the desktop worker. Both hold a kernel lock across an exec.
Say whether the worker's bootstrap is the same mode of the same binary, so
the implementer does not build two.

## What the revision gets right

- The lost-reply gap is stated precisely, tied to `lib.mjs`, and kept as a
  provider dependency rather than a prompt-based wait or a local fix.
- The lock bootstrap preserves bridge ancestry by exec-replacement and gives
  stale-socket recovery exactly one authorized actor.
- Placement now has one configuration source and a registration-time
  agreement check.
- The decisive force-stop check now names real adapters, long-running tool
  work, descendants in their own groups, and an unresponsive harness.

## Outcome

material findings remain
