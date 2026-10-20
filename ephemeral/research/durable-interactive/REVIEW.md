# Author's spec review

Scope: complete SPEC.md and its SOURCE-MAP.md reference. This is the author's
six-pass review, not an independent review or implementation acceptance.
No software changes or live demonstrations were performed.

## Material finding

R1 — Spanning/public contract. See SPEC.md, Public shape and Open spec defect:
conversation termination provenance. Interactive must return normally for
both agent end and human finish, yet a conversation workflow must remain
interactive until archive. Its approved message-only transcript does not
tell ordinary workflow code which event ended the call. A special hidden host
policy would not resolve the public contract transparently.

Proposed amendment for human decision: expose agent-versus-human termination
provenance in the result, then keep the conversation's continuation and exit
in its ordinary workflow code. Re-entry must wait for further human input
rather than spin on repeated model end responses; preserve the disposition of
already accepted input across that boundary. The exact public representation
has not been added pending that decision.

## Passes

| Pass | Result |
| --- | --- |
| Consistency | Generic end, human finish and archive-only conversation lifetime are consistently stated, but their composition is not constructible without resolving R1. No detach/retain/process-count requirement is present. |
| Claim traceability | C1-C12 and L1-L4 route the specified durable ingress, atomic publication, replay, lifetimes, shared hosting, UI and admission behavior. Archive-only lifetime still depends on R1. Initial entry alternatives and native resource handling carry no invented acceptance claims. |
| Seam audit | All six proposed cells have distinct reasons to change, boundaries, contracts and counted ownership/budgets. Internal pseudocode is semantic rather than a demand for wrapper APIs. The decomposition and budget values remain proposed for human review. |
| Claim classification | Twelve cell-local claims and four integration claims. Every cell has routed claims; the native adapter seam is exercised live. Static admission is covered by C12. Cell-local stand-ins cannot substitute for live integration claims. |
| Derivability | Storage engine, notification mechanism, pending-input batching, file layout and native resource tactics remain internal. Stable identity, atomic publication, accepted-input durability and uncertainty boundaries belong at seams. R1 is the remaining identified underivable public decision. |
| Pump hygiene | This is initial authoring, not amendment of an approved authority. Human decisions are recorded in the worklog. No review opinion has silently become an approved public API; the proposed provenance amendment remains explicitly open. |

## Disposition

Not ready to govern implementation until R1 is resolved and the proposed
decomposition, budgets and derived contracts are accepted. Run spec-review on
the resulting amendment. An independent pass via request-adversarial-review
is available before implementation.
