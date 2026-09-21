# Svelte audit collation — 2026-09-21

decision: Tyler wants issue fixes delivered sequentially; no parallel writers
or bake-offs across the shared Svelte files. Current task is collection and a
short recommendation before workflow implementation.

friction: The published catalogue references twenty uncommitted reviewer reports
in the root checkout, and the experiments remain on separate Claude branches.
Collected them with local issue copies and the original scratchpad under the
existing ignored .gimble/issues path; tracked only the curated index/plan.

decision: #341 is explicitly blocked on skgo window/map support and unproved
recovery/byte comparisons. #344 mixes those prerequisites with later recurring
auditing, so issue-number order cannot determine execution order.

doc_bug: The old audit briefs require exactly ten findings; raw round-2 reviewer
09 calls correct code a finding. Future audits need confirmed findings without
a quota, independent verification, and deduplication against existing reports.

decision: Preserve the uncommitted claude/session-page files as research input
for #342; do not modify the other session's working tree.

correction: Tyler deferred #341 and rejects eliminating SSE as a cleanup goal.
Retain SSE, keep identity in route parameters, and prefer native Last-Event-ID
resumption. The current query's stream token is an observation generation, not
an agent session; the run ID is already in the route. Update audit guidance so
it does not repeatedly report this accepted exception.
