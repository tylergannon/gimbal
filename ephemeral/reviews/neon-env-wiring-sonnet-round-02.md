# Adversarial review round 02: Neon DSNEnv wiring, completed run, caller scope adjudication

Reviewer: Sonnet. Date: 2026-11-03. Target: worktree
`/Users/tyler/.codex/worktrees/1430/gimbal` on `codex/distributed-execution-slice`,
uncommitted changes on top of `4bcc3c92`. Round 01 is preserved unchanged in
`neon-env-wiring-sonnet.md`; the caller's adjudication is preserved at the end of
`ephemeral/worklog/distributed-execution-slice.md`. This round supersedes neither.

The launch prompt and follow-up asked me to assess the caller's scope adjudication
rather than assume it. They did not predict a verdict or exclude subject matter, so
no narrowing was ignored. Read-only, artifact-path and no-rerun limits were applied
as operating constraints. No credential value was printed. I read variable names,
file modes, byte counts and pattern-match counts only.

## Evidence inspected

- Round 01, the caller adjudication, the plan (R1-R10, D1-D10, C1 rows) and the
  source diff `plugins/messaging-river/{main.go,main_test.go}`, `.gitignore`,
  `package.json`, `neon.ts`, `examples/config/river-neon.json`.
- Binary timing: `main.go` 14:56:30 and `main_test.go` 14:56:43 precede
  `bin/gimbal` 14:57:44 and `bin/plugins/river.so` 14:57:46. The run started at
  15:05. The run used code at least as new as the source. Round 01's hash match
  between local and amrita binaries stands. I did not rerun the build or the tests.
- Authoritative run `01M6HKPKG8R0GXRWJ41TS5MTGZ` (`run.json`, `run.jsonl`,
  `sessions.json`, `task-route`, `interactions.json`, `model_calls.json`,
  `sessions/agentic-dialogues.1.jsonl`).
- Credential posture: `.env.local` (names and mode), the amrita worker env file
  (names and mode), both `retired-credentials-20261103` archives, `.gitignore`
  results, and the `os.Environ()` call sites in `internal/taskdaemon/process.go:81`
  and `internal/pi/shell/shell.go:94,206`.
- Live processes: local app, local worker A and the amrita worker are running. No
  stray `sleep` or agent shell is left on amrita.

## The run, as completed (corrects and extends round 01)

- Status is `completed`, with no `error` or `recording_error`. The chain is
  `session_closed` (error empty), then `scope_ended`, `run_ended` and `complete`
  at 21:12:12-13. Round 01 saw the run before End; this is the End it lacked.
- `TaskRoute` stores `DSNEnv: DATABASE_URL_UNPOOLED`, the two environment-to-worker
  bindings and queue `gimbal_neon_01a19a10`. The route stores the variable name,
  not the credential.
- All 13 `task_placement` events name `task-remote` and `worker-b`. `sessions.json`
  records `environment task-remote`, `worker_id worker-b`, a workdir on amrita and
  model `gpt-6-luna`. A real native session therefore ran on the remote worker.
- Turn 1 reported `Amritanandamayi.local` and the amrita workdir, with one
  `session.tool.success`. Turn 2 returned the first-turn token with no tool call.
  That is recall across a successive human turn in the same session.
- Turn 3 was a 30-second sleep request. The model replied while the tool was still
  running, and no later tool completion was recorded.
- Turn 4: the Stop request and its interrupt were both consumed. The
  turn ended with `interrupted: true` and `context canceled: human requested
  interruption`.
- **Correction to round 01:** that review said Stop was handled "while a command was
  in progress". The turn 4 session log shows the interrupt arrived while the model
  was still emitting text. No tool had been called in turn 4. What is proven is
  that a Stop control, issued from the page, reached the admitted owner and
  cancelled an active native turn. Cancelling a running remote child process is not
  shown. The plan's C1 row does not require it.
- End (21:12:10) was consumed, the session closed, and the run completed. No
  leftover `sleep` on amrita.
- The scan of run records, the app instance dir and both worker state dirs (local
  and amrita) for a `postgres://user:password@` pattern returned nothing. The
  credential is not in events, prompts, run records or session logs for this run.
  This verifies the caller's "never place into workflow context, prompts, events,
  source or committed files" for the exercised path.

## Assessment of the caller's scope adjudication

Question: does `DSNEnv` violate an accepted requirement or create a material new
privilege boundary compared with `DSNFile` and bootstrap secrets?

**Requirement check.** I found no accepted requirement it violates.

- The plan says "private credential references are derivable implementation
  details", and that only a materially new public contract returns for approval.
  `DSNEnv` is a plugin configuration field, not a new public Gimbal API name.
- The user selected Neon, asked for env-var wiring, and ran `neon-env`. `neon-env`
  delivers the credential through the environment by design. The environment is the
  requested mechanism.
- No plan item, repository rule or user instruction requires isolating credentials
  from native tasks. The repo rule "no secrets in committed files or events" is met
  by the scan above and by `.gitignore` coverage of `/.env`, `/.env.local`, `/.neon`
  and `/node_modules/` (`git check-ignore` confirms all three).

**Privilege boundary check.** It does not create a new privilege boundary.

- Native task children run as the user's own OS account with `os.Environ()`
  inherited. They could already read any same-user `0600` file, including a
  `DSNFile`, the project `.env.local`, and the existing bootstrap secrets. The
  capability to obtain the DSN is unchanged. Round 01's finding 1 treats this as a
  new exposure; as a capability it is not.
- `DSNFile` had already been in use with a Neon DSN on amrita (`neon.dsn`, dated
  14:55, now archived) before this change, so the production credential sat
  readable on that host in either design.

**What is real, and what the adjudication understates.** I concede less than round
01 asked for and more than the adjudication says.

1. **Ambient versus deliberate access.** A file must be read on purpose. An
   environment variable reaches every child automatically, so a plain `env`,
   `printenv`, a crash dump, a tool that logs its environment, or a model asked to
   "show your environment" discloses it with no file path to guess. This is a
   higher accidental-disclosure likelihood. It is not a different privilege. The
   adjudication's statement that env and files are equivalent is true for
   capability and not for accident-proneness.
2. **Breadth on worker A.** Worker A is launched with `--env-file=.env.local`, so it
   and its children also hold `DATABASE_URL` (pooled) and `NEON_BRANCH`. The amrita
   worker's env file holds only `DATABASE_URL_UNPOOLED`. The plugin needs only the
   unpooled one. This is a tidiness gap, not a boundary.
3. **Blast radius.** The credential is for the project's `production` Neon branch
   and carries full read/write on the app database. Same-user trust means a
   prompt-injected agent could damage it. This is accepted by the adjudication's
   trust statement. It is a design limit, and the integration should not be
   described as protecting the database from agents.
4. **Evidence of non-exposure is limited to this run.** The observed commands
   (`hostname`, `pwd`, `sleep`) did not read the environment. Nothing here shows what
   an agent would see; I traced inheritance in source only (`process.go:81`,
   `shell.go:94,206`). Round 01 made the same admission.

**Smallest justified correction.** None is required for an accepted requirement.
Round 01's three options were: a plugin-level scrub, an explicit acceptance, or
keep `DSNFile` on agent hosts. I assess:

- The scrub is not justified. Removing the variable from task environments would be new behavior in
  the task launch path that nobody asked for (the plugin does not own that path). It would also not stop a same-user file read, so it would add
  name-level machinery without a boundary.
- The explicit acceptance already exists in the adjudication. Keep it, and add the
  items under "Residual trust limits" below to it.
- Switching amrita to `DSNFile` is a legitimate user choice but not required by the
  plan. It buys deliberate-read semantics, nothing more.

I therefore agree with the caller on scope and disagree only with the claim that
the two mechanisms are equivalent in every respect.

## Credential file state (round 01 finding 2)

- Obsolete DSN copies are no longer in the original task directories. They are in
  owner-only (`0700`) `retired-credentials-20261103` directories, files `0600`.
  That is relocation. Both archives still hold full Neon DSN copies (`neon.dsn`,
  142 bytes) plus the retired LAN database DSNs and route file, in `/tmp`.
  Retirement did not eliminate them, and the adjudication says "recoverably", so
  this is consistent. If the user wants no extra copies of the live credential,
  deleting those files is a later user-approved step. Rotating the Neon password is
  the only action that removes the exposure if there is doubt about any copy.
- Active files that remain, as the adjudication states: local `.env.local` (`0600`,
  ignored, not tracked), `.neon` (`0600`, ignored) and the amrita worker env file
  (`0600`, one variable). These are needed for restart. No finding.

## Findings

No new material finding. Round 01 findings 3 and 4 stand unchanged as nitpicks:

1. **Nitpick: `examples/config/river-neon.json` is unrequested.** It is plain
   config with hard-coded `workflow-local`, `worker-a`, `task-remote` and `worker-b`.
   The root `npm start` passes no plugin config, so the route comes from elsewhere.
   Either drop it or say in the PR how it is used. Nothing here is a proof program.
2. **Nitpick: coverage of the new path is thin.** There is no unit test for a
   successful `open` through `DSNEnv`; the live run covers it. `parse` also now
   rejects surrounding whitespace in `DSNFile`, a small behavior change for any
   existing config. `DSNEnv` keys differing only in case fail closed (the field
   lookup is case-sensitive) with the fixed error message, which is safe.
3. **Nitpick: `npm start` depends on an ignored local `.neon` link and on `neon-env`
   installed in the ignored `node_modules`.** This works on this checkout. It is
   not reproducible from a clean clone without `neon` setup, which is acceptable
   for a pre-release local convenience but should not be described as a general
   build/start path.

## Accepted requirements: met or unmet (C1 and Neon setup only)

Scope is the C1 row and the Neon-setup claims. D6-D10 are not assessed.

| C1 clause | Status | Evidence |
| --- | --- | --- |
| Declared, validated logical-environment input reaches the registered workflow from the actual conversation UI | Met on the face of the record; the click itself is the caller's report | `graph.json` declares `environment` input; `sessions.json` records `task-remote`. I did not view pixels. |
| Real remote native session receives successive human turns at its admitted owner | Met | Four turns, all placements `worker-b`, recall on turn 2. |
| Controls reach the admitted owner | Met | Stop consumed and completed (`interrupted:true`); End consumed and `session_closed`. See the correction above on what Stop interrupted. |
| Truthful placement | Met | All 13 placements name `task-remote`/`worker-b`, and the amrita process is the worker. |
| Preparation by that boundary or an explicitly supplied existing owned directory | Met on the face | The workdir on amrita pre-dates the run (created 2026-11-01) and is user-owned. `PrepareCheckout` was not recorded in run values, so I cannot tell from the record how it was set. |
| Invalid selection rejects before effects | Not exercised in this run | Covered by the source stage accepted at `1006a81d` and by `TestNamespaceValidationIsPureAndStrict`, which covers an unknown environment. The plugin diff touches `parse`, which that path uses. No new live rejection was shown. |
| Prerequisite remains unmerged | Still true | The `DSNEnv` change, `package.json`, `neon.ts` and the example are uncommitted in the worktree. This evidence is of uncommitted code. |

Neon setup claims: the Neon database is in use over the unpooled endpoint (round 01
counted `gimbal_message_events` rows, which I did not repeat); the worker advisory
lock design is preserved; secret files are ignored and owner-only. All hold.

**Concrete accepted requirement that remains unmet: none.** The only open C1 items
are the two noted above, neither a failure: live invalid-selection rejection was not
re-exercised on Neon, and the change is uncommitted and unmerged.

## Residual trust limits (to carry with the adjudication)

- Native tasks run as the user, inherit the full process environment and can read
  that user's `0600` files. The Neon production credential is readable by them.
  Nothing here isolates a prompt-injected or careless agent from the database.
- Env delivery makes accidental disclosure easier than file delivery (`env` prints
  it). Worker A also inherits two extra Neon variables.
- Retired credential copies, including the live Neon DSN, still exist under `/tmp`
  in owner-only archives. Rotation is the only way to remove doubt about them.
- Stop evidence covers interruption of an active native turn, not termination of a
  running remote child process.
- This is not a security certification and not completion of D6-D10 or of the
  distributed goal. Restart/reconnect, the A-to-B context handoff, attestation and
  the pixel inspection of mixed placement remain future work.

## Outcome

`only nitpicks remain`
