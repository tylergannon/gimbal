# Adversarial review: native-control spec, plan, socket audit (round 02)

## Target

Current `ephemeral/native-control/{spec.md,plan.md,socket-audit.md}` and
`ephemeral/worklog/20261101-native-control-feasibility.md`, against `AGENTS.md`
and the restated Tyler decisions: audit Desktop's control surfaces first, prefer
an existing interface, stop early on an unavailable prerequisite, deliver
Gimbal-initiated native work while Locked Use stays on, and do not build beyond
what was asked. This is a full re-review, not a check of round-01 fixes only.

The launch prompt did not narrow scope, predict findings, or request a verdict.
Nothing was ignored.

## Round-01 disposition (re-verified against sources)

- Ledger finding: resolved. The spec now sets an isolated `GIMBAL_DESKTOP_DIR`
  and preserves the historical ledger. On amrita the historical
  `~/.gimbal/codex-desktop/sessions.json` is unchanged, and
  `Gimbal Qualification/desktop-worker` does not exist yet, so the isolated
  path is clean. The audit's baseline row is corrected.
- Tool-surface and peer-authorizer finding: partly resolved. The audit has the
  two new rows and the spec has a pre-native verification step. See finding 2
  for what remains.
- Owner origin and "fresh task": addressed (owner loaded across both trials;
  "fresh" means a new child under the same owner). The new text opens finding 1.
- `listen` overclaim: fixed. The 500-line budget now says a breach means stop.

## Evidence inspected

- All four documents in full. `plugins/codex-desktop/{server,lib}.mjs` and
  README (unchanged since #440; `git diff` against HEAD is empty).
- Cache files in `/tmp/gimbal-native-control-audit-20261101/`, including the
  newly added renderer modules. I re-read `tue`/`rue` (app-tools server),
  `vd` (peer authorizer), `callDynamicAppTool` (~4608523), `Ize`/`Lze`
  (~2075341), `buildMcpCodexConfig` and the `nl` constant.
- Read-only SSH to amrita: process list (Desktop child 37320 runs
  `app-server --analytics-default-enabled` with only
  `plugins.codex-app-tools@openai-bundled…codex_app.enabled=true`;
  `ipc.sock` is mode 0600), the `Gimbal Qualification` directory listing,
  and the historical ledger. Nothing was started, stopped, installed, or called.
- Not re-verified: the audit's character offsets, FD inventory, launchctl and
  preference observations, and the native addon's binary strings.

## Findings

### 1. [issue] The spec has no way to start the owner task, and "Gimbal-originated" is left undefined for the gate

`spec.md` ("Feasibility and resource limits") says provisioning "starts or
reuses one genuine local Desktop owner task through normal Desktop
mechanisms". It does not say who or what starts it. The only interfaces in
play are the Desktop UI and the same app-tools `create_thread` route whose
availability is the thing under test. The machine is locked. Starting the
owner through the UI is a human action, and starting it through the route
under test makes the admission check depend on its own result. The 20-minute
admission budget and the "no manual unlock" rule cannot both be met unless
this is settled. "Reuse" is also unspecified: no existing loaded owner is
identified, and the worker binds only from a `bind_worker` call made by a
task with genuine thread and turn metadata (`server.mjs`).

Separately, the gate requires "Gimbal initiates" (C5), while the same section
allows a "temporary invocation" kept outside git, and the Gimbal caller cell
is only built after the gate (plan step 5). A shell run of `client.mjs`
satisfies "temporary invocation" but is not a Gimbal workflow run. The spec
should say which of the two C5 and C6 accept at the gate. As written, the
gate could pass on a shell client and leave "Gimbal-originated" unproven,
which is the goal Tyler stated. C3 (caller handling) is also listed as a
definition-of-done claim yet cannot be observed before the post-gate
amendment.

### 2. [issue] The admission check does not cover two observed gates: the renderer window and the owner's turn

`callDynamicAppTool` for `codex_app` calls does not execute in the main
process. It goes through `Ize`/`Lze`, which iterate ready app windows and
require `dynamicAppTools.canCallTool(request)` to accept the call. If no
ready window supports it, the call fails with "No ready app window supports
app tools". `canCallTool` receives the request's `threadId`, `turnId` and
`callId`.

Two consequences the spec does not test:
- `Worker` stores the owner's first `thread_id` and `turn_id` from `bind` and
  sends that same pair on every later `desktopCall`, for the life of the
  process. The spec requires only that the owner stay "loaded". Whether
  Desktop accepts calls against a turn that has finished is not established
  anywhere in the audit. If it does not, the worker works for the first
  minutes and then fails, and a stale-turn failure would be read as a
  Locked Use failure.
- Whether a renderer window counts as "ready" while the display is locked or
  asleep is unaudited, though that is the exact state C5 targets.

The new pre-native step ("verify peer admission and the effective
task-create/read/stop interfaces … in the genuine Desktop context") does
not name either. Add both to the pre-native checks, with the stale-turn case
tested after the owner's turn ends and again after lock.

### 3. [issue] The audit still reports the model's cleanup tools as verifiable only by experiment, but the cancel path depends on a tool the shipped build offers elsewhere

This is narrower than round-01 finding 2, which is now mostly met. The worker's
`cancel` calls `set_thread_archived` on the `codex_app` namespace and treats
`notLoaded` plus no `inProgress` turn as confirmation. In this build,
`set_thread_archived` is defined as the automations constant `nl` and added to
automation tool names. The audit's new row says "renderer definitions include
task-read and archive names/schemas", which is true of the schema files but
leaves the `enabled_tools` list (create, send, handoff, fork only) and the
read gate (`Ev`) as the live default. The spec correctly requires a live check
and a stop if cleanup is unavailable. It then still lets C4 stand as "owned
task termination is confirmed or explicitly retained as unconfirmed". That
wording lets a run claim C4 with termination unconfirmed. The pre-native step
requires reconcile-and-stop to work on a harmless task first, so C4's
"unconfirmed" branch should apply only to failures after that point. State
that, so a missing cleanup tool is a stop rather than a C4 pass.

### 4. [nitpick] Hard-coded model id contradicts repository policy

`AGENTS.md` requires unversioned aliases and says never to hard-code model
versions. `lib.mjs:126` defaults to `gpt-5.6-luna`, and README lines 83 and 95
repeat it. The spec and plan say "Luna" while the worker passes the versioned
id. The bundle does contain `gpt-5.6-luna` (five occurrences), so it
currently works, but the id will rot and the spec does not say whether the
gate passes `luna` or the versioned name. Settle that before the run.

### 5. [nitpick] Budget arithmetic leaves no room for the extra checks

The 60-minute cap splits 20/20/20. Admission now includes owner provisioning,
scoped worker install, peer admission, tool verification, and a harmless
create/read/stop cycle. That is a lot for 20 minutes, and "a tool wait
counts". Either say overruns of the admission phase borrow from nothing and
end the candidate (consistent with the stop rule), or rebalance. Leaving it
implicit invites an argument at the first overrun.

## Outcome

material findings remain
