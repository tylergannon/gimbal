# Route: scope rules and static graph contract

Use this route for scope/context provenance, Set/SetJSON, ownership, graph identity, or analyzer diagnostics. These are current source excerpts and audits; the generated graph/linter remains proposed. Treat older design language about mandatory constant keys or complete ownership as historical/proposed: the current user direction tolerates an incomplete linter and keeps dynamic Set keys supported for now.

- Scope identity, ancestry, shadowing, and end behavior: `rules-index-leaf.md:5-10` -> `rules-source-excerpts.md:25-86`.
- Session owner, post-end rejection, and Fork behavior: `rules-index-leaf.md:11-14` -> `rules-source-excerpts.md:103-148`.
- Group child scopes/Wait and Loop task scope: `rules-index-leaf.md:15-18` -> `rules-source-excerpts.md:150-205`.
- Runtime-enforced versus static-only distinction and counterexample: `rules-report.md:45-85`.
- Static output identity and explicit data edges: `rules-index-leaf.md:21-28`.
- Proposed typed hierarchy/relations and unresolved source mapping: `recommendation.md:15-30,83-97`.
- Supported analyzer approach and diagnostic fixture needs: `rules-index-leaf.md:31-38`; proposal `recommendation.md:69-81`.
- Set-key authoring study: `keys-recommendation.md:3-15`; observed cases and tradeoffs: `keys-evidence.md:13-38`. Stable outer keys are recommended, while dynamic keys remain legal and are disclosed as unresolved precision loss.

Do not route a claim of complete lifetime proof here: the audit and Go report bound what static checks can establish, and dynamic/indirect cases remain visible as incomplete coverage.
