# Author's spec review

Historical record of the abandoned design and implementation. The rewritten
delivery plan and current specs/durable-interactive.md supersede its proposed
mechanisms and dispositions. Do not use this review as a coding checklist.

Historical authoring review at 13ee0140. Independent Opus round 02 subsequently
identified material defects, adjudicated below; its report supersedes the
no-findings disposition of this original pass.

Scope: complete SPEC.md and its SOURCE-MAP.md reference. This is the author's
six-pass review, not an independent review or implementation acceptance.
No software changes or live demonstrations were performed.

## Resolved finding

R1 — Spanning/public contract. Interactive returns normally for both agent
end and human finish, yet a conversation workflow must remain interactive
until archive. The initially approved message-only transcript could not tell
ordinary workflow code which event ended the call.

The human approved EndedBy with agent/human values ("love it"). SPEC.md now
records this in the public result, with an empty value on error, durable
completion provenance, and an ordinary conversation workflow loop. Re-entry
waits for input without spinning; pending input and finish signals preserve
their identity across call occurrences. C6 and L4 cover the resulting behavior.
The amendment was rechecked against all passes below.

## Passes

| Pass | Result |
| --- | --- |
| Consistency | Generic end completes a call with EndedBy agent; human finish returns EndedBy human and lets the conversation workflow return successfully. Re-entry preserves input without an autonomous model loop. No detach/retain/process-count requirement is present. |
| Claim traceability | C1-C12 and L1-L4 route durable ingress, atomic publication, replay, lifetimes, termination provenance, shared hosting, UI and admission. Initial entry alternatives and native resource handling carry no invented acceptance claims. |
| Seam audit | All six proposed cells have distinct reasons to change, boundaries, contracts and counted ownership/budgets. Internal pseudocode is semantic rather than a demand for wrapper APIs. The decomposition and budget values remain proposed for human review. |
| Claim classification | Twelve cell-local claims and four integration claims. Every cell has routed claims; the native adapter seam requires live demonstration. Static admission is covered by C12. Cell-local stand-ins cannot substitute for live integration claims. |
| Derivability | Storage engine, notification mechanism, pending-input batching, file layout and native resource tactics remain internal. Stable identity, atomic publication, accepted-input durability, termination provenance and uncertainty boundaries belong at seams. No further underivable public decision was identified. |
| Pump hygiene | This is initial authoring. Human decisions are recorded in the worklog. R1's proposed amendment entered the public contract only after the human accepted it. No review opinion silently became an approved public API. |

## Disposition

No unresolved material finding remains in the author's pass. The completed
draft is ready for final review of the proposed decomposition, budgets and
derived contracts before implementation. This is not an independent review,
implementation sign-off or live acceptance report. An independent pass via
request-adversarial-review is available.


## Independent review adjudication

The round-02 Opus report is retained at
`ephemeral/reviews/durable-interactive-opus-spec-round-02.md`.

- F1 accepted: stable ingress is run/scope/name, separate from per-call
  operation ordinal; pending input spans calls and is unprocessed at actual
  owner termination. Removed the undefined superseded disposition.
- F2 accepted within the human's delegated easiest-default choice: every
  occurrence is human-first. The example's opening message changes accordingly.
- F3 accepted: admission applies to all workflows/helpers. Existing effects
  move through RunCommand/Check/Service, no new API. The human approved complete
  RunCommand output while observation stays bounded. Maintained workflows and
  examples are owned and must pass admission. Unrecorded concurrency/map
  iteration are rejected in authored execution.
- F4 accepted with the stated same-code boundary: executable plus loaded
  plugin digests gate reconstruction; mismatched old runs stay visibly blocked
  with retained state/input. No multiversion runtime or upgrade migration.
- F5 accepted: corrected generated-header detection, counted omitted existing
  production paths and assigned them to the existing six cells. Larger measured
  baselines reflect those paths; the repeated illustrative file tree is removed.

The authoritative contract is now `specs/durable-interactive.md`; the old
SPEC.md is a pointer. A fresh independent review reads that full contract and
the single delivery plan, without a defect checklist or expected verdict.


## Round-03 disposition (awaiting human decisions)

Claude Opus run 01M5BEMKTNQKNMWWBQ1F9C8J1A.run-prompt completed successfully
and produced `ephemeral/reviews/durable-interactive-opus-spec-round-03.md`.
It confirmed all five original findings were materially addressed, then found
five further contract gaps. It reproduced the corrected six baselines exactly.

- F1 pending HITL: upgrade-blocked conversations need a viable recovery path.
  Asked whether to retain required executable/plugin artifacts and share Go
  execution hosts per build. This preserves same-code replay rather than
  requiring changed-code migration or permanent blocked tabs.
- F2 accepted: reuse the internal optional native ResumeSession capability;
  Codex is the required restart-continuity harness (`luna`). Other harnesses
  retain their existing visible unresolved-recovery limitation. Assigned
  codex.go/recovery.go to H (1,199 existing lines, H baseline 6,528/max 7,500).
  No resource-retention policy or public HarnessAdapter method was added.
- F3/F4 pending HITL: asked together whether to reject authored defer/recover
  and unrecorded timing/cancellation observations. Proposed explicit completion
  paths and existing library commands for waits, with unsupported agent-turn
  timeouts removed rather than silently inventing an API.
- F5 accepted: per-occurrence returned transcripts include only confirmed
  delivered human input plus committed replies. Pending/unresolved/unprocessed
  input remains visible separately; a message consumed in N+1 appears only in
  that call's result. Order follows delivered messages within durable turns.
- The lower-severity remote ingress gap is resolved by routing interaction
  controls through existing ControlWorkflow transport, retaining environment
  placement. No new backend interface method is needed.
- Applied the TypeScript generated-header counting clarification.

Implementation has not started. The two batched questions are necessary
material authoring/upgrade choices, not requests to reapprove internal tactics.


## Human resolution of round-03 choices

The human explicitly rejects both support and non-support policy/enforcement
for version changes across runs: there is NO POLICY. This supersedes the
author's proposed digest gating, mismatch blocking and retained-build options.
Removed them and the version-boundary claim; no replacement policy or policy
tests are added. Same-code replay is the proof scenario, not an enforcement gate.
This resolves the review's version-policy dissent through HITL rather than
turning the reviewer's objection into an implementation requirement.

The human approves the proposed authored defer/recover and unrecorded timing/
cancellation restrictions ("sounds fine but please explain"). Explained Go
unwinding and deterministic replay, including removal of unsupported agent-turn
timeouts. Spec/plan now require library-controlled suspension unwind, rejected
authored constructs, explicit completion cleanup and existing library waits.
F2/F5 and remote ingress resolutions remain as recorded above.


## Round-04 adjudication and implementation handoff

Opus run 01M5BJYVZXYGR9D9FM8XHTVQQ1.run-prompt completed successfully. Its
report is `ephemeral/reviews/durable-interactive-opus-spec-round-04.md`.

- F1 accepted factual correction: the review's prior conversation-only resume
  premise was wrong for ordinary workflows. Verified binding.Adapter uses
  agentexecutor for agy/claude/codex/pi. Spec/plan now reuse existing durable
  executor checkpoint/reattach and deterministic IDs, without a new lifecycle
  interface or a Codex-only restriction. Assigned the existing executor/effects
  modules and root loop/service/supervision/doc files to E. E baseline 7,428,
  max 9,000; H drops the obsolete Codex conversation ownership to 5,329/max 6,500.
- F2: shared-host ordinary panic containment is a derived isolation requirement
  and is now explicit at the per-run activation boundary. Existing Run/Group
  panic reporting remains. Actual host loss is distinct from human cancellation.
  The demand for service restoration/missing-service rechecks is rejected under
  the human's explicit SERVICE exclusion; no such guarantee or check is added.
  The human's no-version-policy decision likewise precludes an update policy.
- F3 accepted clarification through existing Run signature: logical call remains
  pending during dormancy, authored stacks unwind, same-ID re-entry reconstructs;
  direct callers supply body/bindings and need ingress for human input. Original
  caller deadline is recorded once, stays wall-clock during dormancy and is reused
  in accepted operation requests. No new exported suspension result/sentinel.
- F4 accepted: narrow deterministic opaque-function allowlist, source-visible
  helper traversal, rejected unknown calls; sorted map keys/standard JSON map
  ordering are allowed forms. Existing godoc examples must match admission.
- F5: existing dormant cancellation and retry reconciliation are made explicit.
  Unconfirmed external cessation may remain unresolved, as the human's original
  brief already requires. Rejected an invented abandon/success outcome or an
  eventual-settlement guarantee. Cleanup retains existing cancellation behavior;
  suspension itself performs none, with SERVICE restoration still deferred.

The manager accepts this one-unit/two-owner handoff under the human's delegated
plan authority. A fresh spec/plan review uses the committed source snapshot while
implementation proceeds, so its target is stable and broad build checks are not
duplicated. Construction blockers still return to the manager; no implementer
amends the durable contract.


## Round-05 adjudication and construction clarifications

Opus run 01M5BKY6SHM652B4XJH3QVSWD1.run-prompt reviewed pinned afd5958a
while implementation proceeded. Artifact:
`ephemeral/reviews/durable-interactive-opus-spec-round-05.md`.

- F1 accepted and returned to HITL: the human chooses BOTH local durability and
  remote daemon durability, sequentially ("do both, one and then the other").
  Same owners/delivery, local native checkpoint first, remote L5 afterward,
  one final broad validation pass. H gains taskdaemon (1,409 lines) and daemon
  command (65): baseline 6,803/max 8,500. No SERVICE restoration/rechecks.
- F2 accepted preservation of existing Stop behavior: durable STOP requests
  turn interruption and returns the still-pending Interactive to wait after
  confirmed cessation; distinct from FINISH, no fabricated partial chat,
  EndedBy or sibling cancellation. Uncertainty remains unresolved.
- F3 accepted explicit preservation inventory. Metadata, cached file context,
  worktree/environment/association/rename/setup and explicit cleanup remain;
  settled-idle local Codex model/effort changes use durable private controls
  and existing native thread. No new root API or new provider-wide config.
- F4 accepted named generation gate. A had already implemented admission in
  gimbalgen before this report: violations refuse generated registration/graph
  output. Hosting consumes generated definitions; no runtime source sandbox
  or cryptographic provenance machinery is added.
- F5 ownership nit fixed: P gets handwritten conversation route Go/Svelte
  (+95 existing lines, baseline 13,882/max 14,800); A owns generated skgo links
  copies by provenance and only regenerates them. Plan now covers changes to
  existing unowned files as well as additions.
- The four small gaps are construction clarifications: maintained example
  uses hosted registered admission with saved accepted ID; direct Run ingress
  is the existing host/control installation (absent it, no input is invented);
  ordinary run Interactive has Finish as well as message/Stop controls; expired
  native evidence never authorizes fresh dispatch of an unmatched recorded
  intent. Runtime is adding the private reconstruction guard.

A separate observed construction gap adds minimal existing audit-index read
modes (--completion and --verify-current) so recorded library commands preserve
ReadCompletion/VerifyCurrent validation. A owns its 83 existing lines; baseline
11,609/max 13,200. This is existing behavior, no new paid audit or root API.

These changes remain inside the one delivery; the next independent full review
will include the coherent implementation and observed proof as well as current
spec/plan, rather than repeat standalone spec passes while code is changing.
