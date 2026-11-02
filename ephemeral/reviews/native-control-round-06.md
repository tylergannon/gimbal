# Adversarial review: native-control spec, plan, audit (round 06)

## Target

Current `ephemeral/native-control/{spec.md,plan.md,socket-audit.md,spec-review.md}`
and the worklog, against `AGENTS.md` and the restated Tyler decisions (audit
Desktop control surfaces first, favor an existing interface, stop early,
Gimbal-initiated native work while Locked Use stays on, build only what was
asked). Full re-review.

The launch prompt did not narrow scope, predict findings, or request a verdict.
Nothing was ignored.

## Round-05 disposition

- Discovery dead end (#1): resolved. The spec now pre-authorizes one fresh
  local execution-owner chat, created only after genuine Desktop context is
  verified, and says a second discovery failure stops the gate.
- Readiness-file access (#2): resolved. `status` with the required bundle id
  is an admission check, denial is a precise stop, and the audit has a row.
- Leftover multi-route wording (#3): resolved (pseudocode and routing text now
  name the single worker).
- Lock indicators (#4): resolved. The spec names `CGSSessionScreenIsLocked`
  (via `CGSessionCopyCurrentDictionary`), `IOConsoleLocked`,
  `CGGetOnlineDisplayList` and `IODisplayWrangler`. I confirmed read-only over
  SSH that the session dictionary is obtainable and `IOConsoleLocked` is
  readable from a non-GUI session, but the Python on amrita has no `Quartz`
  module, so the implementer needs `ctypes` or `swift` (both are present).

## Evidence inspected

- All documents in full. `plugins/` and `internal/` have no changes against HEAD.
- `plugins/codex-desktop/{.mcp.json,README.md,scripts/launch,client.mjs,lib.mjs}`
  and the cached config reference entries for `mcp_servers.<id>.env` and
  `env_vars`.
- Read-only SSH to amrita: session-lock and console-lock state reads, and a
  check of whether the Desktop app-server child carries `GIMBAL_DESKTOP_DIR`
  (inconclusive: `ps eww` output did not list it; I treat it as unset).
- Not re-verified: the inspector-chat snapshot, trace-anchor offsets, native
  addon strings. Nothing was started, stopped, installed or called.

## Findings

### 1. [issue] The required isolated worker directory cannot be set by the shipped registration; the fallback is the preserved historical ledger

The spec says to set `GIMBAL_DESKTOP_DIR` to
`…/Gimbal Qualification/desktop-worker` "in the scoped MCP registration and
every caller", and forbids touching `~/.gimbal/codex-desktop`. But the shipped
registration, `plugins/codex-desktop/.mcp.json` (and the README's alternative
`config.toml` block), lists the name only under `env_vars`. Per the cached
reference, `env_vars` is a whitelist of variables inherited from the host
process (`mcp_servers.<id>.env_vars`), while `mcp_servers.<id>.env` is the map
that sets values. The registration has no `env` map, and the inherited
environment of Desktop's app-server child does not carry the variable. A Desktop
restart to change that environment is excluded.

So, as installed, the worker process starts with `GIMBAL_DESKTOP_DIR` unset and
`defaultDirectory()` returns `~/.gimbal/codex-desktop`. That is the historical
ledger owned by thread `01a1233f-…`. `Worker.load()` then throws, and `bind`
fails with the owner-conflict message (the round-01 scenario), or, worse for
the stated protection, a future change of that thread's state would make the
worker write into the preserved directory. The spec's own safety rule
("preserve, do not re-own") is therefore not enforced by anything it
authorizes. Callers can use `--state-dir`, but the MCP process cannot.

The spec needs to say how the value reaches the MCP process: either an `env`
entry in a registration the gate is allowed to create (and which one: the
plugin's `.mcp.json`, which would be a change to the shipped worker the gate
bars, or the README's project `config.toml` route), or a different approach.
It should also say which of those the "normal refresh" covers. The README
itself says a local install or update "may require a fresh desktop session or
restart" and offers the project-config route as the no-restart check. Neither
the spec nor the audit mentions that route.

### 2. [issue] The native target for C5 and C6 is not chosen

C5 requires reading accessibility, capturing a screenshot, typing and
clicking "a harmless reversible target". The only app named is Finder, in the
readiness check (`status` with "the required Finder bundle ID") and in the
history sentence about the earlier failure. Typing into Finder is not an
obviously reversible action, the README example uses TextEdit, and the saved
approval file is 140 bytes, so only a few bundle ids are approved. If the
approved set does not include the chosen target, the gate stops on consent
before any native action, and a human action is required that the spec lists
only as a general blocker.

Name the target app, its bundle id, the reversible action, and whether its
saved approval has been confirmed (the check described in finding 2 of the
previous round can do that). Otherwise the validator and implementer can pick
different targets and both claim C5.

### 3. [nitpick] The spec has grown beyond what the gate needs

The spec is now roughly three times its first size, and much of the added text
(lock-indicator names, log correlation, model-alias handling, activation
remedies) is operating recipe or implementation detail rather than contract.
That is defensible for a single gated experiment, but it makes the approval
harder to read and increases the chance of contradictions like those removed in
earlier rounds. Consider moving indicator and log specifics into the recipe the
spec already promises, and keeping the spec to the contract and stop rules.

## Outcome

material findings remain
