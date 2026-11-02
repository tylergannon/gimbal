# Adversarial review: native-control spec, plan, socket audit (round 01)

## Target

`ephemeral/native-control/{spec.md,plan.md,socket-audit.md}` and
`ephemeral/worklog/20261101-native-control-feasibility.md`, judged against
`AGENTS.md` (no wrappers, build what was asked, proof is what you saw) and the
Tyler decisions the spec restates: audit Desktop's own control/socket surfaces
first, prefer an existing adequate interface, stop early on a missing
prerequisite, Gimbal-initiated native work while Locked Use stays on.

The launch prompt did not narrow scope, predict findings, or request a verdict.
Nothing was ignored. `/tmp/gimbal-native-control-audit-20261101/reviewer-round-01.json`
is an empty file and was not used.

## Evidence inspected

- The three documents and the worklog, in full.
- `plugins/codex-desktop/{server,lib,client}.mjs`, its README and tests.
  Checked: physical total 381 lines (matches the spec); nonblank 366, so the
  500-line budget leaves little room.
- Cached modules in `/tmp/gimbal-native-control-audit-20261101/`. The three
  SHA-256 values in the audit match the files. I read the app-tools listener
  (`tue`, around offset 338454), its peer authorizer `vd`, the `codex_app`
  tool enablement near `buildMcpCodexConfig`, `set_thread_archived` at
  217710, and `read_thread` gating at 782638.
- The cached `config-file-config-reference.md` and `app-server.md`.
- Read-only SSH to amrita: `ls` of `/tmp/codex-app-tools` and
  `~/.gimbal/codex-desktop`, and a read of the ledger's owner and request
  states. No service was touched and no native-control call was made.
- Not re-verified: the audit's character offsets, the FD inventory, launchctl
  and preference observations, and the "15,513 files" count.

## Findings

### 1. [issue] Audit says the worker state is absent, but a ledger owned by another task exists on amrita, and the worker cannot bind a new owner

`socket-audit.md` (last checklist row) says "owned marketplace and worker
directories absent … Still uninstalled". But `~/.gimbal/codex-desktop/`
exists (created Oct 9, mode 0700). It holds `sessions.json` with version 1,
`ownerThreadId` `01a1233f-…`, and three requests, all `cancelled`.

`Worker.load()` (`lib.mjs`, the `ownerThreadId !== caller.thread_id` check)
throws `Worker belongs to desktop task … resume that task to recover its
sessions` for any other binding thread. `server.mjs` `bind()` calls `load()`
before it listens, so a fresh owner task cannot bind.

Consequences:
- The spec's C1 ("genuine owner binds once") and the "Owner restart/unload
  recovery documented as observed" clause have no viable first step. Either
  the old owner thread must still be resumable (unverified), or the ledger
  must be moved or removed, which is remote state mutation that the spec
  neither authorizes nor describes.
- The audit's baseline of what exists is wrong, so a 20-minute admission
  budget could be spent on a deterministic `owner_conflict` failure that is
  already knowable.
- The worker's own ledger also contradicts the audit's "historical default
  worker socket absent" reading as evidence of cleanliness.

Fix: correct the audit row, check whether `01a1233f-…` is loadable, and add
an explicit, approved decision on ledger handling to the spec before any
experiment.

### 2. [issue] The audit certifies the transport but not the app-tools surface the worker depends on

The worker's `read` and `cancel` call `read_thread` and `set_thread_archived`
through the `codex_app` namespace. In the shipped bundle:
- `buildMcpCodexConfig` sets `enabled_tools` for `codex_app` to
  `create_thread`, `send_message_to_thread`, `handoff_thread`, `fork_thread`.
- `read_thread` is exposed only when `Ev(...)` finds it in dynamic tools or
  config (offset ~782638).
- `set_thread_archived` appears only as an automations constant (`nl`, offset
  217710, added to the allowed names for automation runs) and as an
  automations prompt line. I found no handler on the app-tools socket.

So C3 ("failed observation and pending approval reported") and C4
("termination confirmed or retained as unconfirmed"), and the spec's
reliance on the existing worker, may rest on tools this build does not offer
to an MCP-spawned caller. The audit never traced the tool list or the
dispatch gate. It also records only that "peer authorization" exists. `vd()`
loads a native addon (`authorizeSocketPeer`) on packaged darwin builds. What
it verifies, such as signing identity or ancestry, decides whether a Node
MCP child passes. The earlier Oct 9 success was on an older build, and the
audit itself says the bundle is now 26.1021.

This is the "address native ownership rather than only transport
compatibility" gate that the spec demands of the first route, and it is
unaudited. The checklist's "inspected" label overstates coverage here.

Fix: add two audit rows (authorizer semantics, effective tool list and
dispatch for `codex_app`) before the reviewed route is chosen.

### 3. [issue] "Gimbal-originated" is not defined, because the owner task is not created by Gimbal

`server.mjs` binds only when a Desktop task calls the `bind_worker` MCP tool
with that task's thread and turn metadata. The Gimbal caller cell (spec
table) contract is to "initiate the native task through the selected real
interface". It never says who starts and keeps alive the owner task, or
whether the owner is itself subject to Locked Use.

C5 requires Gimbal to initiate while locked. C6 requires a "fresh task",
defined as a new execution context. With the ledger rule above, a fresh
child under the same owner and a fresh owner are different cases. The first
needs a durable owner whose lifetime across lock and relock is unknown. The
second hits finding 1. Neither is a "Gimbal initiated" start unless some
Desktop task was started first, by hand or otherwise.

Fix: state the owner's origin and lifetime in the contract, and say which
reading of "fresh" C6 means.

### 4. [nitpick] Audit overclaims one negative

The "Official configuration reference" row says "No `listen` match on
fetched reference". The cached reference has 13 `listen` matches (proxy,
OAuth callback and `experimental_network` listeners). None is an app-server
or Desktop selector, so the conclusion stands, but the sentence is false as
written and invites a reviewer to distrust the rest of the table. Reword it
to "no app-server or Desktop listener setting".

### 5. [nitpick] Code budget is nearly consumed before any change

The budget is 500 nonblank lines for the worker; the existing runtime is 366
nonblank. If findings 1 to 3 force worker changes (ledger re-owning, tool
fallbacks), the cap is a likely breach. The spec correctly treats a breach as
a spec defect, but it should say now that exceeding it means an operational
answer or a stop, not a bigger cap.

## Outcome

material findings remain
