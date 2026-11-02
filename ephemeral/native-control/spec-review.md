# Mechanical spec review

November 1, 2026. Sol's review of the unapproved draft; independent review is
retained separately under `ephemeral/reviews/`.

| Required pass | Assessment |
| --- | --- |
| Consistency | Approval is separated into a bounded feasibility gate and a concrete post-gate integration amendment. “Fresh” means a new execution task under the same persistent owner; no cold-start promise. |
| Claim traceability, both directions | C1–C4 cover worker/caller ownership, submission and lifecycle; C5–C6 cover actual locked native work and repetition. Socket audit serves early route selection, not runtime success. |
| Seam audit | Conditional worker/caller responsibilities differ by Desktop integration versus workflow policy; budgets and proposed file ownership are explicit. Final caller placement and any alternative seam await demonstrated feasibility and approved amendment. |
| Claim classification | Four cell-local claims and two integration claims. External Desktop/native seam has mandatory live claims. Cell-local lifecycle semantics are verified through the worker contract; actual remote termination remains part of live gate reconciliation. |
| Derivability / spanning | No universal impossibility conclusion from failed candidates. Current source cannot establish real peer admission or effective dynamic tools; these are named experiment prerequisites. Unselected architecture is excluded from implementation authorization. |
| Pump hygiene | No approved spec exists yet. Interview decisions and review corrections are incorporated into this draft for explicit HITL approval; implementers are not authorized to amend it. |

The interview has not reached explicit approval. Do not label the integration
constructible or start product changes until the remaining route-specific
decisions are settled through the post-gate amendment.
