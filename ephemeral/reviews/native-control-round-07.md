# Adversarial review: native-control spec, plan, approval brief, audit (round 07)

## Target

Current `ephemeral/native-control/{spec.md,plan.md,approval.md,socket-audit.md,spec-review.md}`
and the worklog, against `AGENTS.md` and the restated Tyler decisions (audit
Desktop control surfaces first, favor an existing interface, stop early,
Gimbal-initiated native work while Locked Use stays on, build only what was
asked). Full re-review.

The launch prompt did not narrow scope, predict findings, or request a verdict.
Its operating constraints (no raw process environments, credentials or private
transcripts; no state changes) are valid and were followed. For the record: in
round 06 I counted occurrences of a variable name in a process environment dump
via `ps eww`. I did not repeat that this round, and nothing here depends on it.

## Round-06 disposition

- Isolated directory not settable (#1): resolved. The staged
  `plugins/gimbal-codex-desktop/.mcp.json` on amrita has an `env` map setting
  `GIMBAL_DESKTOP_DIR` to the qualification path, alongside the `env_vars`
  whitelist. The spec requires that staged copy and an installed-effective-value
  check before any bind, and treats an unset or different value as an admission
  failure. I read the staged file and confirmed it.
- Native target undefined (#2): resolved. Finder is the target, with a defined
  reversible action (Go to Folder dialog, type a marker without submitting,
  screenshot, Cancel) and a confirmed-before-use approval check.
- Spec accretion (#3): partly addressed by `approval.md`, which gives Tyler a
  short statement of what is being authorized.

## Evidence inspected

- All five documents in full; `plugins/` and `internal/` have no changes
  against HEAD.
- Read-only SSH: the staged marketplace tree under
  `/private/tmp/gimbal-native-qualification-_h1f4m_4/desktop-integration`
  (`marketplace.json`, `.codex-plugin/plugin.json`, `.mcp.json`, file listing and
  checksums), and absence of the `desktop-integration` and `desktop-worker`
  directories under `Gimbal Qualification`. Checksums of the staged
  `server.mjs`, `lib.mjs`, `client.mjs`, `package.json` and `scripts/launch`
  match the repository files at HEAD (`2f67be83`).
- Not re-verified: the inspector-chat snapshot, native addon strings, trace
  offsets, and the consent state of Finder (unreadable from SSH).

## Findings

No critical issues or issues remain that I can substantiate from the sources.
The one risk the documents already name stays open and is not a defect in them:
discovery of the scoped worker on the resumed or fresh owner is expected to be
the likeliest stop, and a stop there will say nothing about Locked Use.

### 1. [nitpick] The staged marketplace is not pinned to a revision

The spec says to use the staged tree in `/private/tmp/...` and copy it to the
owned directory. The staged runtime files currently equal the repository at
HEAD, but nothing in the spec requires re-checking that before the copy, and
`/private/tmp` content can change or be purged. The operating recipe has to
state "actual component versions/revision". Add one line: compare the staged
runtime files with the revision under test before copying, and record the
revision.

### 2. [nitpick] The registration delta lives only in the temporary tree

The `env` map exists in the staged `.mcp.json` and not in
`plugins/codex-desktop/.mcp.json` in the repository. Anyone who later installs
from the repository copy gets the whitelist only, and the worker falls back to
the historical ledger directory (the round-06 hazard). That is outside this
gate, but the post-gate amendment and recipe should say that the repository
registration needs the delta, or that the isolated path must always come from a
scoped registration. Also note `GIMBAL_DESKTOP_DIR` appears in both `env` and
`env_vars` in the staged file; the effective-value check already covers any
precedence surprise, so no change is needed beyond that check staying mandatory.

### 3. [nitpick] The approval brief omits two stop conditions the spec relies on

`approval.md` lists the 20/20/20 phases and the fresh-owner allowance, but not
the locked-state re-admission, the stale-owner-turn stop, or the model-alias
comparison, all of which can end phase 1. It does say an early failure can be
inconclusive. One more sentence naming those stops (and that discovery failure
is the expected most likely stop) would keep the approval from reading as easier
to pass than the spec is.

## Outcome

only nitpicks remain
