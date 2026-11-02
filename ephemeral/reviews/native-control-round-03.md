# Adversarial review: native-control spec, plan, socket audit (round 03)

## Target

Current `ephemeral/native-control/{spec.md,plan.md,socket-audit.md}` and
`ephemeral/worklog/20261101-native-control-feasibility.md`, against `AGENTS.md`
and the restated Tyler decisions (audit Desktop control surfaces first, prefer
an existing interface, stop early, Gimbal-initiated native work while Locked Use
stays on, build only what was asked). Full re-review.

The launch prompt did not narrow scope, predict findings, or request a verdict.
Nothing was ignored.

## Round-02 disposition

- Owner origin (round-02 #1): addressed by naming a bootstrap chat; the new
  text opens finding 1. The Gimbal-initiated requirement is now fixed: both
  trials must run through the existing Gimbal controller, and a shell-only
  invocation is insufficient. Resolved.
- Renderer readiness and stale owner turn (#2): addressed in the spec and in a
  new audit row. Resolved as a test requirement.
- Cleanup tool and C4 (#3): resolved. Missing cleanup is an admission failure
  and C4 can no longer pass unconfirmed.
- Model id (#4): the spec now forbids the worker's hard-coded default. The
  replacement opens finding 2.
- Phase budgets (#5): resolved ("admission cannot borrow later time").

## Evidence inspected

- All four documents, in full. `plugins/codex-desktop/*.mjs` is unchanged from
  HEAD (`git diff` empty), so the review of its behavior stands.
- Cache files in `/tmp/gimbal-native-control-audit-20261101/`: app-tools
  server (`tue`/`rue`), `callDynamicAppTool`, `Ize`/`Lze`, the `remote-control:`
  host-id construction (~4084588), model-family and slug references in
  `main`, `app-initial` and `app-shared`.
- `lib.mjs`/`client.mjs`/README for `hostId` and `model` handling.
- Not re-verified: the bootstrap chat's identity and "not loaded" snapshot (it
  lives in remote state outside the cache; I made no remote call this round),
  the audit's offsets, and the native addon's binary strings.

## Findings

### 1. [issue] The bootstrap owner is a remote-control-host chat resumed by an unaudited route, and the worker never carries a host id

`spec.md` names an existing chat (`01a193e2-…`, host
`remote-control:env_e_6a4d…`) as the owner and says "the existing host/task
routing can address it without depending on the proposed worker". Three gaps:

- That routing is itself a Desktop task-control interface: it has to resume
  an unloaded thread and deliver a turn. It is not a row in `socket-audit.md`.
  The audit covers SSH and remote-control only as transports that do not
  "expose the existing Desktop-local child". If it works, it may be the
  adequate existing interface Tyler asked to favor, and the worker is
  unnecessary. If it is restricted, the bootstrap fails before any admission
  test. Either way the spec's "smallest evidenced route" claim is made without
  the evidence.
- `remote-control:` host ids are built for environments the controlling client
  reaches over the remote-control websocket (main ~4084588). The spec does not
  say whether the thread is local to amrita's own Desktop or addressed from
  another client. This matters because `lib.mjs` `desktopCall` sends no
  `hostId`, and the app-tools `tools/call` schema defaults it to local
  (`hostId` enum, default `u.h`). The worker also stores
  `created.hostId || 'local'` and reuses it for `read_thread` and
  `set_thread_archived`. A thread whose host id is not local would be called
  under the wrong host, and the failure would look like a stale-context or
  Locked Use failure.
- The chat is not loaded, and its tool set was fixed before the scoped
  plugin existed. The spec says "after normal discovery" but does not say
  whether a resumed thread picks up a newly registered MCP server or a fresh
  thread is needed. A fresh thread is excluded ("no alternative owner/chat is
  created"), so a likely failure there is a forced early no-go that the spec
  could have predicted.

Fix: add an audit row for the resume/follow-up route, state which host the
thread is local to, and decide before approval whether a resumed thread can
discover a newly registered server.

### 2. [issue] The `luna` alias has no resolver in the spec or the worker

The new spec text requires the unversioned `luna` alias, "resolved through the
installed model-selection surface", and forbids the worker's default. But the
worker forwards `request.model` verbatim to `create_thread`
(`lib.mjs` `start`), and no code in the plan resolves a family name to a
slug. In the bundle, `luna` is only a family token used to classify slugs
(`aC=[luna,terra,sol]`, with a regex for `family-version`). Concrete model ids
are exact slugs, such as `gpt-5.6-luna` (in the cached source) and
`gpt-6-luna` (`app-shared`), and one module maps other names to explicit
slugs. So the likely outcomes are: `create_thread` rejects `luna` (an
admission failure that has nothing to do with native control), or the caller
quietly picks a slug, which is the hard-coded version the repository forbids.

Neither the audit nor the spec shows that `create_thread` accepts a bare
family alias. Verify that in the harmless-task step before it is a gate, and
say who resolves the alias if it is not accepted. Because the gate allows no
worker change, the caller workflow is the only place this can happen.

### 3. [nitpick] The only named candidate has a likely, decisive no-go that the spec does not weigh

The spec allows two routes but names one: the worker bound from a finished
owner turn. It tests stale-turn admission and stops that candidate if it
fails. The worker reuses the first turn id for every call (`server.mjs`
`bind`, `lib.mjs` `desktopCall`), and the audit has no source evidence that
Desktop accepts a finished turn. The spec cannot create an alternative owner
under this gate. The audit's own row for the Desktop client coordination IPC
(follower start/steer for loaded tasks) is the other credible route, but the
spec never says it is the second candidate or why it is excluded. State the
second route now, or state that a stale-turn no-go ends the gate with no
second attempt, so the outcome is not argued later.

### 4. [nitpick] "Repeat the admission check while locked" has no procedure

The spec requires an admission check while locked and recording renderer
readiness "in that observed lock/display state". It does not say how the
locked state is reached or confirmed, who locks, or what happens if the
machine is not locked during the window. Locking by hand is an unlock/relock
manual action on the thing under test; waiting for a natural lock is
unbounded inside a 20-minute phase. Define it as an observation (record
whichever state is present, and mark a not-locked run as not satisfying the
check).

### 5. [nitpick] Admission phase is still overloaded

Admission now contains scoped install, resuming the unloaded chat, binding,
letting the owner turn finish, re-admission, a locked re-admission, and a
harmless create/read/stop cycle, in 20 minutes with tool waits counted and
borrowing forbidden. Finding 1 or 2 alone could consume it. This is consistent
with the stop rule, but the approval request should say plainly that an
inconclusive 20-minute no-go is the expected common result.

## Outcome

material findings remain
