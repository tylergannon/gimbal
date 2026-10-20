# Author's spec review

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
