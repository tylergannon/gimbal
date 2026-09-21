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
decision: Filed #347 separately from deferred #341 for stream identity in the route and standard Last-Event-ID resumption; included it in the active sequential queue.

correction: Tyler wants the whole series in one worktree. The workflow should
add an outer issue-list iteration around the existing implementation promise
loop. Remove the proposed per-issue worktrees/merges; checkpoint successful
issues in sequence on one branch.

decision: Tyler authorized trying the existing implement loop inside the outer
series, with active monitoring for cost and latency rather than stopping merely
because it is expensive or slow. Preserve run records and note timing, model
usage, retries, validation outcomes, and signs of process overhead.

decision: Every issue scope carries the absolute research-index path as context,
and the tracked series file points to the local issue snapshots. This makes the
Claude catalogue, twenty raw reports, POCs, and session-page WIP discoverable
without loading the full corpus into every prompt.

friction: gimblegen emits fixed `Graph` and `Command` identifiers, so two
generated entrypoints cannot share a package even though same-package placement
would let the extractor inline the existing implementation helper. Filed #348;
keep the issue-series package self-contained for this run rather than expanding
the current task into a generator refactor.

friction: the first full `just test` attempt reached the web tests before a
fresh `web/build/skgo.manifest.json` existed. Running `just build` first created
the normal generated web output; the following full `just test` passed. This is
an ordering dependency in a clean checkout, not a failure in the issue-series
workflow.
