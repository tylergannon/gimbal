# Author's spec review

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
