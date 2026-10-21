# Adversarial review: issue 439 proposal, finished application state, round 04

Reviewer: Claude Fable 5.1, same session as rounds 01 to 03, 2026-10-09.

## Review target

The proposal tree rooted at `ephemeral/research/issue-439/proposal.md` at
commit `18c3fc90` (`docs: distinguish provider cleanup from local process
containment`, clean working tree), assessed as the finished application it
describes. Reviewed against the same authoritative sources as the earlier
rounds:

- `ephemeral/research/issue-439/requirements.md`
- `ephemeral/research/issue-439/issue.json`
- `AGENTS.md`, `docs/definition-of-done.md`, `go doc -all .`
- the current source on this branch, including the merged PR #440 worker
  (`plugins/codex-desktop/`, still at `1c3eb84c`)

Operating constraints from the caller: read-only except this artifact; write
to the caller-supplied path under `ephemeral/reviews/`. Rounds 01 to 03 are
left untouched. No caller narrowing of subject matter or predicted verdict
was present.

## Evidence inspected

Change since round 03: `git diff cfb66dc9..18c3fc90` touches only
`ephemeral/`: `proposal.md` (cancellation risk row), `topics/ownership.md`
(new section "Shared providers retain named ownership when control is
unavailable"; force-stop paragraph reworded), `topics/backend-contract.md`
(local managed versus shared provider stop; validation bullet),
`topics/delivery-and-review.md` (pi-first demonstration bullet; shared
Codex/OpenCode admission bullet), `topics/execution-placement.md` (timeout
note; `server.mjs` versus `scripts/launch`; two bootstrap modes), and
`ephemeral/worklog/20261009-issue-439-design.md` (two entries). No Go,
Node, `justfile`, or `plugins/` change: `git diff cfb66dc9 --stat -- .
':!ephemeral'` is empty.

Re-read in full: the diff above, `ownership.md:36-62`, and the four changed
topic pages at the changed sections. Pages unchanged since round 01
(`interchange.md`, `lean-runners.md`, `workflow-discovery.md`,
`storage-and-remote.md`, the computer-use pages, `application-review*.md`)
stand as reviewed.

Implementation read for the new claims: `codex/rpc.go:80-130` (`connect`
starts the shared app-server daemon from inside the adapter when none is
running), `codex/codex.go:219,310` (`thread/start` precedes `turn/start`),
`codex/codex.go:531-534` (`turn/interrupt` result discarded),
`opencode/server.go:30-31,51-52,190-230` (shared state directory under
`~/.gimbal/opencode`, reuse-or-start, `prepareServerProcess`, PID and birth
token recorded), `opencode/adapter.go:236-262` (`Abort` result discarded),
`opencode/process_unix.go:44-63`, `agy/process_unix.go:12-24`,
`pi/adapter.go:1-40` (in-process adapter, no harness subprocess),
`internal/pi/shell/process_unix.go:11-19`, `plugins/codex-desktop/.mcp.json`
(four-variable environment allowlist), `plugins/codex-desktop/scripts/launch`,
`plugins/codex-desktop/README.md:11-13,34-36`. External evidence is
unchanged from round 01.

## Disposition of round 03 findings

1. **Daemon-hosted adapters versus the no-residual rule: resolved as a
   decision.** `ownership.md:43,47-51`, `backend-contract.md:15,17,35`, and
   `delivery-and-review.md:11-12` now separate a local managed unit
   (independent force boundary, wedged harness included) from a shared
   provider session (verified native interrupt-and-observe, named
   cleanup-pending while the provider is unavailable, never kill the shared
   daemon) from unknown escaped execution (rejected). The claim that the
   current Codex and OpenCode cancel paths do not establish descendant
   cessation matches `codex/codex.go:534` and `opencode/adapter.go:259`.
   The first slice now names pi first and claims nothing proven.
2. **Creation timeout manufactures the lost-reply case: resolved.**
   `execution-placement.md:33`.
3. **Wrong file for socket exclusivity: resolved.**
   `execution-placement.md:67`.
4. **Two bootstraps: resolved as a decision.** `execution-placement.md:67`:
   local gate as a project-runner mode, desktop lock mode from the installed
   Gimbal executable, duties kept distinct, no dependency of the desktop
   worker on a project runner binary.

## Findings

### 1. Issue (incomplete requirement): the runner itself starts the shared Codex and OpenCode daemons, and the proposal's own spawn rule has no place for that

**Requirement.** `requirements.md:13`: "Independent processes must not
become unmanaged." `ownership.md:43`: "Every Gimbal-controlled root spawn
uses this managed path … Unknown escaped execution is not an accepted
residual." `ownership.md:49`: "Never kill the shared Codex or OpenCode
daemon."

**What the tree does today.** The Codex adapter's `connect` runs
`codex app-server daemon start` from inside the adapter when no daemon is
running (`codex/rpc.go:83-100`), and its own comment says the result is
machine-shared with Codex Desktop and is never stopped by Gimbal. The
OpenCode integration does the same: it reuses a healthy server recorded in
the shared `~/.gimbal/opencode` state directory or starts `opencode serve`
in its own process group, records PID and birth token, and returns
(`opencode/server.go:30-31,51-52,203-216`). Both are long-lived services
created by whichever Gimbal process first needs them, intended to outlive
that process.

**Why the proposal cannot absorb this as written.** On a fresh machine the
first hosted Codex or OpenCode run spawns the daemon from the runner. The
registration gate then has two options and both break a rule: register the
daemon as a run unit, in which case cancel or force-stop of that run signals
the shared daemon (`ownership.md:49` forbidden) and every later run depends
on a process another run owns; or exempt it, in which case a
Gimbal-controlled root spawn exists outside the managed path, which
`ownership.md:43` says is not an accepted residual. The new shared-provider
section describes how to *use* a daemon but never says who is allowed to
*create* one in the finished application, and the delivery bullet
(`delivery-and-review.md:12`) tests stop and unavailability, not
provisioning.

**Smallest response.** One decision, one sentence in the shared-providers
section: shared daemons are instance-level infrastructure provisioned
outside any run, by the server at project admission or by an explicit user
command, recorded as instance-shared and excluded from run cancel; hosted
runs connect with `startDaemon=false` and fail explicitly with
`errDaemonNotRunning` (`codex/rpc.go:81`) when the daemon is absent. The
existing OpenCode lifecycle commands already have this shape. Then add
"first hosted run on a machine with no daemon" to the demonstration list so
the decision is observed, not assumed.

### 2. Nitpick: the desktop lock mode must find the installed Gimbal executable with no `PATH`

`execution-placement.md:67` runs the lock mode "from the installed Gimbal
executable before execing Node." The plugin's MCP configuration forwards
only `CODEX_APP_TOOLS_PIPE_PATH`, `CODEX_MCP_NODE_PATH`, `HOME`, and
`GIMBAL_DESKTOP_DIR` (`plugins/codex-desktop/.mcp.json`), so the launch
script runs with the default shell `PATH`, and the marketplace host "loads a
cached copy" of the plugin (`README.md:36`), so a binary cannot ride along
in the plugin folder without becoming the "extra installed bootstrap
program" the proposal rejects. Say how the launch resolves the executable
(a `HOME`-relative install path or a path written at provisioning) and add
"Gimbal installed on this Mac" to the README's prerequisites, which today
list only macOS, Node 22+, and a signed-in desktop (`README.md:11-13`).

### 3. Nitpick: for pi, "wedge the harness" and "wedge the runner" are the same process

`delivery-and-review.md:11` validates pi first and asks to "wedge the local
runner/harness." `pi/adapter.go:1-3` is in-process: no harness subprocess
exists, so the unresponsive-harness scenario is the unresponsive-runner
scenario, and passing it for pi says nothing about a harness that wedges
while the runner stays healthy. State that the harness-separate wedge case
is first exercised by agy (`agy/process_unix.go:12`, a real subprocess with
group kill) so the pi pass is not read as covering it.

## What the revision gets right

- The three-way distinction (local managed unit, shared provider session,
  unknown escaped execution) is coherent, matches the tree's adapters, and
  stops advertising a force boundary that shared daemons cannot supply.
- Allocation-before-turn is already available for the Codex daemon
  (`thread/start` then `turn/start`), so the native-admission rule is
  satisfiable there without new provider work.
- The two bootstrap modes are tied to one internal launcher code path with
  distinct deployment duties, consistent with "as simple as possible."
- Every "not current behavior" and "nothing already proven" marker remains
  accurate against `1c3eb84c`.

## Outcome

material findings remain
