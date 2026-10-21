# Adversarial review: issue 439 proposal, finished application state, round 05

Reviewer: Claude Fable 5.1, same session as rounds 01 to 04, 2026-10-09.

## Review target

The proposal tree rooted at `ephemeral/research/issue-439/proposal.md` at
commit `dd4d21a0` (`docs: trim placement paragraph whitespace`, on top of
`7fc938b3 docs: separate shared provider provisioning from run admission`,
clean working tree), assessed as the finished application it describes.
Reviewed against the same authoritative sources as the earlier rounds:

- `ephemeral/research/issue-439/requirements.md`
- `ephemeral/research/issue-439/issue.json`
- `AGENTS.md`, `docs/definition-of-done.md`, `go doc -all .`
- the current source on this branch, including the merged PR #440 worker
  (`plugins/codex-desktop/`, still at `1c3eb84c`)

Operating constraints from the caller: read-only except this artifact; write
to the caller-supplied path under `ephemeral/reviews/`. Rounds 01 to 04 are
left untouched. No caller narrowing of subject matter or predicted verdict
was present.

## Evidence inspected

Change since round 04: `git diff 18c3fc90..dd4d21a0` touches only
`ephemeral/`: `topics/ownership.md` ("Gimbal-controlled" narrowed to
"hosted run-owned"; new provisioning paragraph opening the shared-providers
section), `topics/backend-contract.md` (provision outside any run; attach
only; attachment identity is not exclusive ownership),
`topics/delivery-and-review.md` (pi wedge is runner wedge; subprocess
harness wedge on agy; first hosted launch with no daemon must fail
explicitly, then provision, retry, cancel one run, shared infrastructure
survives), `topics/execution-placement.md` (provisioning records the
installed Gimbal executable's absolute path; Gimbal install is a
prerequisite of the future lock mode only), and the worklog (one decision,
one correction). No Go, Node, `justfile`, or `plugins/` change:
`git diff 18c3fc90 --stat -- . ':!ephemeral'` is empty.

Re-read: the diff above in full, `ownership.md:36-52`,
`execution-placement.md:57-68`, `delivery-and-review.md:8-14`. Pages
unchanged since round 01 stand as reviewed.

Implementation read for the new claims: `codex/codex.go:96-98` (`conn`
calls `connection(ctx, true)`: the turn path starts the daemon),
`codex/codex.go:401,475` (only interrupt/unregister and archive-on-close
use `startDaemon=false`), `codex/rpc.go:79-103` (`errDaemonNotRunning`;
`daemonStatus` discovers the socket; `codex app-server daemon start`),
`opencode/client.go:107` (`Connect` calls `ensureServer`: the adapter's
session path lazily starts the shared server), `opencode/adapter.go:68-69`
("The first session starts or discovers the shared OpenCode server; closing
sessions never stops it"), `opencode/server.go:64-123` (`StartServer`,
`StopServer`: reads the state file, checks the PID birth token, then
SIGTERM and SIGKILL to the process group), `opencode/server.go:53-62`
(default state dir `~/.gimbal/opencode`), `cmd/gimbal/main.go:157-205`
(`gimbal opencode start|stop`, state dir from `GIMBAL_OPENCODE_DIR` or
`$HOME`), `opencode/process_unix.go:44` (`Setpgid`). External evidence is
unchanged from round 01.

## Disposition of round 04 findings

1. **Runner starts the shared daemons: resolved as a decision.**
   `ownership.md:49` and `backend-contract.md:15` provision shared providers
   outside any run, make hosted adapters attach-only with an explicit
   absent-provider failure, forbid lazy daemon spawn in the runner, and
   `delivery-and-review.md:12` demonstrates the first-launch-without-daemon
   case. The claim that Codex has an internal attach-only path is accurate
   (`codex/codex.go:401,475`); the claim that OpenCode lacks one is
   accurate (`opencode/client.go:107`).
2. **Gimbal executable discovery without `PATH`: resolved.**
   `execution-placement.md:69`.
3. **pi runner wedge equals harness wedge: resolved.**
   `delivery-and-review.md:11`.

## Findings

### 1. Issue (incorrect premise): Gimbal does own the OpenCode daemon, and its stop command kills every attached run's work without consulting ownership

**What the proposal says.** `ownership.md:49`: configuration "does not
acquire exclusive ownership of a daemon that may serve other instances or
the native desktop"; provisioning goes "through the backend's explicit
setup/lifecycle command". `ownership.md:51`: "a shared service cannot
inherit that physical force guarantee." `backend-contract.md:15`: "Known
shared attachment identity is not exclusive service ownership."

**What the tree does.** The only OpenCode lifecycle command is
`gimbal opencode start|stop` (`cmd/gimbal/main.go:157-205`). `StopServer`
(`opencode/server.go:74-123`) reads the PID and birth token Gimbal recorded
when it launched `opencode serve` into its own process group
(`opencode/process_unix.go:44`), then SIGTERMs and, after a timeout,
SIGKILLs that whole group. Gimbal therefore does own this daemon in exactly
the physical sense the proposal reserves for local managed units: it
created the group, holds its identity, and can force-stop it. The daemon
is per-user (`~/.gimbal/opencode`), so every server instance and direct
`Run` of that user attaches to the same process. This is unlike the Codex
daemon, which Gimbal never stops (`codex/rpc.go:86-87`).

**Concrete failure.** Two instances, or one instance and one direct run,
share the daemon. An operator runs `gimbal opencode stop` to "recycle" it,
which the proposal names as the lifecycle path. The group kill terminates
every attached session's tool descendants immediately. The registry learns
only that the provider became unreachable: each session goes
cleanup-pending with its reservation held, although cessation was in fact
established by the kill. Nothing in the proposal lets the stop command see
attached sessions, and nothing records its verified group kill as the
authoritative stop evidence it is. The cleanup-pending entries then depend
on a later restart and a successful `Abort` against OpenCode's persisted
session store to resolve, which the proposal does not describe.

**Smallest response.** Split the shared-provider rule into the two cases
the tree actually has. Externally owned provider (Codex app-server, Codex
Desktop): everything in `ownership.md:49-51` as written. Gimbal-provisioned
provider (OpenCode under `~/.gimbal/opencode`): the lifecycle stop command
first lists sessions attached by live owned runs from the instance registry
and refuses without an explicit force flag; when it does stop, its
PID-token-verified group kill is recorded as cessation evidence for every
session hosted in that group, so no entry is left cleanup-pending for work
that is known dead. Add "stop the provisioned OpenCode daemon while a run
is attached" to the demonstration list.

### 2. Nitpick: the Codex turn path is not attach-only today, and the wording suggests no Codex change is needed

`ownership.md:49`: "Codex already has the internal `startDaemon=false` /
`errDaemonNotRunning` path; hosted OpenCode needs the same attach-only
behavior." The attach-only path is used only by interrupt and
archive-on-close (`codex/codex.go:401,475`). The path every turn takes,
`conn` (`codex/codex.go:96-98`), passes `true` and starts the daemon. Say
that the hosted Codex adapter must call the attach-only variant on its turn
path, or an implementer reads the sentence as "Codex done, OpenCode to do."

### 3. Nitpick: for Codex, neither the provisioning command nor the recorded endpoint has an existing mechanism

`ownership.md:49` says configuration "records the known shared attachment
endpoint and identity" and provisioning uses "the backend's explicit
setup/lifecycle command or existing service manager". For OpenCode both
exist (`gimbal opencode start`, the state file). For Codex, Gimbal has no
setup command; the daemon is started by the external `codex app-server
daemon start`, and its socket is discovered at connect time through
`daemonStatus` (`codex/rpc.go:89`), not read from configuration. Name the
per-provider provisioning step in `delivery-and-review.md:12` and say that
for Codex the recorded identity is whatever `daemonStatus` reports at
admission, so the demonstration is unambiguous.

## What the revision gets right

- Lazy daemon start is removed from hosted runs, with an explicit failure
  and a demonstration that exercises it.
- The narrowing from "Gimbal-controlled" to "hosted run-owned" keeps direct
  library runs self-owned, consistent with `ownership.md:67`.
- The desktop lock mode now has a stated executable-discovery rule that does
  not touch `PATH` or the marketplace cache, and the plugin's current
  Node-only prerequisites are left accurate.
- The pi validation no longer implies a harness-wedge result it cannot
  produce.

## Outcome

material findings remain
