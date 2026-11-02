# Adversarial review: native-control spec, plan, audit (round 05)

## Target

Current `ephemeral/native-control/{spec.md,plan.md,socket-audit.md,spec-review.md}`
and the worklog, against `AGENTS.md` and the restated Tyler decisions (audit
Desktop control surfaces first, favor an existing interface, stop early,
Gimbal-initiated native work while Locked Use stays on, build only what was
asked). Full re-review.

The launch prompt did not narrow scope, predict findings, or request a verdict.
Nothing was ignored.

## Round-04 disposition

- Reload evidence (#1): resolved. The audit row and worklog now say the only
  traced `config/mcpServer/reload` is internal-marketplace-only, and the spec
  defines "normal refresh" narrowly.
- Undefined candidate 1 (#2): resolved by removing it. The gate now tests the
  existing worker only; direct routes are follow-ons. The spec says to stop and
  propose a smaller route if an adequate direct endpoint appears.
- Unreplayable snapshot, ambiguous budget, plan/spec route mismatch (#3–#5):
  resolved. The audit labels the snapshot as not independently replayed, the
  three 20-minute caps are stated as the whole gate, and `plan.md` points to the
  single worker candidate.

## Evidence inspected

- All documents in full, including the new `spec-review.md`. `plugins/` and
  `internal/` have no changes against HEAD.
- Cache files in `/tmp/gimbal-native-control-audit-20261101/` (re-read
  `config/mcpServer/reload` context).
- `plugins/codex-desktop/lib.mjs` `appReadiness` and `start`.
- Read-only SSH to amrita: `Group Containers` listing and the metadata of the
  Computer Use approvals file; one attempt to read that file's content
  (denied; see finding 2). Nothing was started, stopped or called.
- Not re-verified: the inspector-chat snapshot (remote-control state, outside
  this review), offsets, and the native addon strings.

## Findings

### 1. [issue] The single candidate is predicted to stop at discovery, and the allowed remedy has been removed

The spec now says "static source currently establishes only an
internal-marketplace reload path, so failure to discover the scoped worker on
resume is a credible early inconclusive result", the audit says "CLI
registration may require a fresh owner task", and the spec bars a fresh
owner, a Desktop restart, an app-server replacement, native Settings
automation and any invented reload. Taken together, the approval asks Tyler
to authorize a 60-minute gate with an install and a follow-up to an existing
chat whose first step the documents themselves expect to fail, with no
sanctioned way past it.

That is honest, and it protects the "don't build around" rule, but it leaves
the decision Tyler most needs to make implicit. The approval text should state
the choice directly: either (a) accept that the likely result is an
inconclusive stop at discovery and treat it as information, or (b) pre-authorize
one specific activation remedy (for example one fresh owner task, with its
bounded scope) so that the downstream questions (peer admission, stale turn,
renderer readiness, locked native action) can be tested at all. As written, the
most informative parts of the gate may never run, and a stop at step 1 will say
nothing about Locked Use.

### 2. [issue] The worker's readiness check reads a protected container file that may not be readable from the worker's context

`appReadiness` (`lib.mjs`) is invoked by every `start` that carries
`requiredApps` and by `status`. It reads
`~/Library/Group Containers/2DC432GLL2.com.openai.sky.CUAService/Library/Application Support/Software/ComputerUseAppApprovals.json`.
Any error other than `ENOENT` becomes `readiness_unavailable` and the
submission fails. On amrita that file exists (140 bytes, Oct 20), and an SSH
session got `Operation not permitted` when reading it (the file mode is world
readable, so this is OS-level data protection, not permissions). The worker
runs as a child of Desktop's app-server, whose permission context may or may
not differ; nobody has checked. Neither the audit nor the spec lists this
among admission checks, though the spec's native tasks will rely on it
(`requiredApps` is how the worker enforces "saved native app approval").

Consequence: after clearing discovery, binding, peer admission and the stale
turn, the harmless admission task could fail on an unrelated filesystem
permission, and the failure would be reported as `readiness_unavailable`. The
spec forbids changing code in the gate, so the workaround of omitting
`requiredApps` would defeat the consent check. Add this read to the admission
step (read the file from the worker's own process context) and say what
happens if it is denied.

### 3. [nitpick] Leftover multi-route wording

After the move to a single candidate, three sentences still assume a choice:
"The first route must be chosen from the reviewed audit with a reason …"; "Installing
only the scoped existing worker after approval is allowed if the reviewed route
needs it"; and the pseudocode line "select smallest evidenced candidate route".
They are harmless but contradict "One candidate route: the existing Desktop
worker". Reword them to name the worker.

### 4. [nitpick] Lock indicators are named but not specified

"Adjacent read-only macOS session and display indicators" are the only evidence
that the locked check was met, and "unknown lock state cannot pass". The spec
does not say which indicators, so two runs could disagree about whether the
check was satisfied. Name them in the approval so the validator and the
implementer use the same ones.

## Outcome

material findings remain
